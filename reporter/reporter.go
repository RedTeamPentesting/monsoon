package reporter

import (
	"fmt"
	"sort"
	"time"

	"github.com/RedTeamPentesting/monsoon/cli"
	"github.com/RedTeamPentesting/monsoon/response"
)

// Reporter prints the Responses to a terminal.
type Reporter struct {
	term cli.Terminal
}

// New returns a new reporter.
func New(term cli.Terminal) *Reporter {
	return &Reporter{term: term}
}

// HTTPStats collects statistics about several HTTP responses.
type HTTPStats struct {
	Start          time.Time
	StatusCodes    map[int]int
	Errors         int
	Responses      int
	ShownResponses int
	Count          int

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
func (h *HTTPStats) Report(current string) (res []string) {
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

	if current != "" {
		status += fmt.Sprintf(", current: %v", current)
	}

	res = append(res, status)

	for code, count := range h.StatusCodes {
		res = append(res, fmt.Sprintf("%s: %v", colorStatusCode(code, ""), count))
	}

	sort.Strings(res[2:])

	return res
}

// Display shows incoming Responses.
func (r *Reporter) Display(ch <-chan response.Response, countChannel <-chan int) error {
	r.term.Printf(Bold("%7s %8s %8s   %-8s %s\n"), "status", "header", "body", "value", "extract")

	stats := &HTTPStats{
		Start:       time.Now(),
		StatusCodes: make(map[int]int),
	}

	for response := range ch {
		select {
		case c := <-countChannel:
			stats.Count = c
		default:
		}

		stats.Responses++

		if response.Error != nil {
			stats.Errors++
		} else {
			stats.StatusCodes[response.HTTPResponse.StatusCode]++
		}

		if !response.Hide {
			r.term.Printf("%v\n", FormatResponse(response))
			stats.ShownResponses++
		}

		r.term.SetStatus(stats.Report(response.Item))
	}

	r.term.Print("\n")
	r.term.Printf("processed %d HTTP requests in %v\n", stats.Responses, formatSeconds(time.Since(stats.Start).Seconds()))

	for _, line := range stats.Report("")[1:] {
		r.term.Print(line)
	}

	return nil
}
