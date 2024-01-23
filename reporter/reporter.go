package reporter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http/httputil"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RedTeamPentesting/monsoon/cli"
	"github.com/RedTeamPentesting/monsoon/response"
)

// Reporter prints the Responses to a terminal.
type Reporter struct {
	term                    cli.Terminal
	longRequest             time.Duration
	printRequestAndResponse bool
	showValues              []bool
}

// New returns a new reporter. For requests which took longer than longRequest
// to process, the time is shown.
func New(term cli.Terminal, longRequest time.Duration, printRequestAndResponse bool, showValues []bool) *Reporter {
	return &Reporter{term: term, longRequest: longRequest, printRequestAndResponse: printRequestAndResponse, showValues: showValues}
}

// HTTPStats collects statistics about several HTTP responses.
type HTTPStats struct {
	Start            time.Time
	StatusCodes      map[int]int
	InvalidInputData map[string][]string
	Errors           int
	Responses        int
	ShownResponses   int
	Count            int

	lastRPS time.Time
	rps     float64
}

func formatSeconds(secs float64) string {
	sec := int(secs)
	hours := sec / 3600
	sec -= hours * 3600
	min := sec / 60
	sec -= min * 60

	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, min, sec)
	}

	return fmt.Sprintf("%dm%02ds", min, sec)
}

// Report returns a report about the received HTTP status codes.
func (h *HTTPStats) Report(last []string) (res []string) {
	res = append(res, "")
	status := fmt.Sprintf("%v of %v requests shown", h.ShownResponses, h.Responses)
	dur := time.Since(h.Start) / time.Second

	if dur > 0 && time.Since(h.lastRPS) > time.Second {
		h.rps = float64(h.Responses) / float64(dur)
		h.lastRPS = time.Now()
	}

	if h.rps > 0 {
		status += fmt.Sprintf(", %.0f req/s", h.rps)
	}

	todo := h.Count - h.Responses
	if todo > 0 {
		status += fmt.Sprintf(", %d todo", todo)

		if h.rps > 0 {
			rem := float64(todo) / h.rps
			status += fmt.Sprintf(", %s remaining", formatSeconds(rem))
		}
	}

	switch {
	case len(last) == 1:
		status += fmt.Sprintf(", last: %v", last[0])
	case len(last) > 1:
		status += fmt.Sprintf(", last: [%v]", strings.Join(last, ", "))
	}

	res = append(res, status)

	// add list of status codes sorted by the code
	statusCodes := make([]int, 0, len(h.StatusCodes))
	for code := range h.StatusCodes {
		statusCodes = append(statusCodes, code)
	}

	sort.Ints(statusCodes)

	for _, code := range statusCodes {
		res = append(res, fmt.Sprintf("%s: %v", colorStatusCode(code, ""), h.StatusCodes[code]))
	}

	if len(h.InvalidInputData) > 0 {
		res = append(res, colored(red, "Invalid Input Data:"))
	}

	for _, errString := range sortedKeys(h.InvalidInputData) {
		res = append(res, Bold("  - "+errString)+": "+strings.Join(h.InvalidInputData[errString], ", "))
	}

	return res
}

func filterArray(toFilter []string, showValue []bool) []string {
	var out []string
	for i, v := range toFilter {
		if showValue[i] {
			out = append(out, v)
		}
	}
	return out
}

// Display shows incoming Responses.
func (r *Reporter) Display(ch <-chan response.Response, countChannel <-chan int) error {
	r.term.Printf(Bold("%7s %8s %8s   %-8s %s\n"), "status", "header", "body", "value", "extract")

	stats := &HTTPStats{
		Start:            time.Now(),
		StatusCodes:      make(map[int]int),
		InvalidInputData: make(map[string][]string),
	}

	// make sure we update the status at least once per second
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var last []string

next_response:
	for {
		var (
			resp response.Response
			ok   bool
		)

		select {
		case resp, ok = <-ch:
			resp.Values = filterArray(resp.Values, r.showValues)
			if !ok {
				break next_response
			}
		case c := <-countChannel:
			stats.Count = c
			countChannel = nil
			continue next_response
		case <-ticker.C:
			r.term.SetStatus(stats.Report(last))
			continue next_response
		}

		stats.Responses++

		if resp.Error != nil {
			stats.Errors++

			var reqErr response.InvalidRequest
			if errors.As(resp.Error, &reqErr) {
				errString := cleanedErrorString(reqErr.Err)
				stats.InvalidInputData[errString] = append(stats.InvalidInputData[errString], formatValues(resp.Values))

				continue
			}
		} else {
			stats.StatusCodes[resp.HTTPResponse.StatusCode]++
		}

		if !resp.Hide || resp.Error != nil {
			r.term.Printf("%v\n", FormatResponse(resp, r.longRequest))
			stats.ShownResponses++
		}

		last = resp.Values

		r.term.SetStatus(stats.Report(last))

		if r.printRequestAndResponse {
			defer r.dislayRequestAndResponse(resp)
		}
	}

	r.term.Print("\n")
	r.term.Printf("processed %d HTTP requests in %v\n", stats.Responses, formatSeconds(time.Since(stats.Start).Seconds()))

	for _, line := range stats.Report(nil)[1:] {
		r.term.Print(line)
	}

	return nil
}

func formatValues(values []string) string {
	if len(values) == 1 {
		return fmt.Sprintf("%q", values[0])
	}

	return fmt.Sprintf("%q", values)
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))

	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func (r *Reporter) dislayRequestAndResponse(res response.Response) {
	if res.HTTPResponse == nil {
		return
	}

	r.term.Print(delimiter("Request"))
	requestHeaderBytes, err := httputil.DumpRequestOut(res.HTTPResponse.Request, false)
	if err != nil {
		r.term.Print("Error: cannot dump request header: " + err.Error())
		return
	}

	// we need to override the request protocol version because DumpRequestOut
	// sends the request through a mock transport that resets the version, so
	// our best guess for the actual version that we sent is the response
	// protocol version
	r.term.Print(styleHeader(strings.TrimSpace(string(requestHeaderBytes))+"\n\n", res.HTTPResponse.Proto))

	requestBodyBytes, err := io.ReadAll(res.HTTPResponse.Request.Body)
	if err != nil {
		r.term.Print("Error: cannot dump request body: " + err.Error())
		return
	}

	if len(requestBodyBytes) != 0 {
		r.term.Print(string(requestBodyBytes))
	}

	if res.Decompressed {
		r.term.Print(delimiter("Decompressed Response"))
	} else {
		r.term.Print(delimiter("Response"))
	}

	r.term.Print(styleHeader(strings.TrimSpace(string(res.RawHeader))+"\n\n", ""))

	if len(res.Body) != 0 {
		r.term.Print(string(res.Body))
	}
}

func styleHeader(hdr string, requestVersion string) string {
	scanner := bufio.NewScanner(strings.NewReader(hdr))
	scanner.Split(bufio.ScanLines)

	if !scanner.Scan() {
		return ""
	}

	var sb strings.Builder

	firstLine := scanner.Text()
	if strings.HasPrefix(firstLine, "HTTP") {
		sb.WriteString(styleFirstResponseLine(firstLine))
	} else {
		sb.WriteString(styleFirstRequestLine(firstLine, requestVersion))
	}

	sb.WriteString("\n")

	for scanner.Scan() {
		sb.WriteString(styleHeaderLine(scanner.Text()) + "\n")
	}

	return sb.String()
}

func styleFirstResponseLine(line string) string {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return line
	}

	code, _ := strconv.Atoi(parts[1])

	return parts[0] + " " + Bold(coloredByStatusCode(code, strings.Join(parts[1:], " ")))
}

func styleFirstRequestLine(line string, version string) string {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return line
	}

	return Bold(parts[0]) + " " + colored(cyan, Bold(parts[1])) + " " + version
}

func styleHeaderLine(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return line
	}

	return Dim(parts[0]+":") + parts[1]
}

func delimiter(name string) string {
	repeats := 50 - len(name)
	if repeats < 1 {
		repeats = 1
	}

	return colored(blue, Bold("\n―― "+name+": "+strings.Repeat("―", repeats)))
}
