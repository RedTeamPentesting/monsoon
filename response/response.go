package response

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Response is an HTTP response.
type Response struct {
	Values   []string
	URL      string
	Error    error
	Duration time.Duration

	Header, BodyStats TextStats
	Extract           []string
	ExtractError      error

	HTTPResponse *http.Response
	Body         []byte
	RawHeader    []byte
	Decompressed bool

	Hide bool // can be set by a filter, response should not be displayed
}

func quote(strs []string) []string {
	res := make([]string, 0, len(strs))
	for _, s := range strs {
		r := strconv.Quote(strings.TrimSpace(s))
		r = r[1 : len(r)-1]
		res = append(res, r)
	}
	return res
}

func (r Response) String() string {
	if r.Error != nil {
		// don't print anything if the request has been cancelled
		if r.Error == context.Canceled {
			return ""
		}
		if e, ok := r.Error.(*url.Error); ok && e.Err == context.Canceled {
			return ""
		}

		return fmt.Sprintf("%7s %18s   %v", "error", r.Error, r.Values)
	}

	res := r.HTTPResponse
	status := fmt.Sprintf("%7d %8d %8d   %-8v", res.StatusCode, r.Header.Bytes, r.BodyStats.Bytes, r.Values)
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, ok := res.Header["Location"]
		if ok {
			status += ", Location: " + loc[0]
		}
	}
	if len(r.Extract) > 0 {
		status += " data: " + strings.Join(quote(r.Extract), ", ")
	}
	return status
}

func extractRegexp(buf []byte, targets []*regexp.Regexp) (data []string) {
	for _, reg := range targets {
		if !reg.Match(buf) {
			continue
		}

		if reg.NumSubexp() == 0 {
			for _, m := range reg.FindAll(buf, -1) {
				data = append(data, string(m))
			}
		} else {
			matches := reg.FindAll(buf, -1)
			for _, match := range matches {
				for _, m := range reg.FindSubmatch(match)[1:] {
					data = append(data, string(m))
				}
			}
		}
	}

	return data
}

func extractCommand(ctx context.Context, extraEnv []string, buf []byte, cmds [][]string) (data []string, err error) {
	for _, command := range cmds {
		if len(command) < 1 {
			panic("command is invalid")
		}

		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Stdin = bytes.NewReader(buf)

		// we throw away stderr here so that it does not break the output
		cmd.Stderr = io.Discard

		cmd.Env = append(os.Environ(), extraEnv...)

		buf, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("command %s failed: %v", command, err)
		}
		data = append(data, string(buf))
	}
	return data, nil
}

// ReadBody reads at most maxBodySize bytes from the body and saves it to a buffer in the
// Response struct for later processing.
func (r *Response) ReadBody(res *http.Response, maxBodySize int, decompress bool) (finalError error) {
	var err error

	// Read a limited amount of data from the response such that extraordinarily large
	// responses don't slow down the scan. If the actual bodyReader is larger, it will be
	// closed preemptively, closing the TCP connection. The reason is that opening a
	// new connection likely has a much lower performance impact than tranferring large
	// amounts of unwanted data over the network.
	bodyReader := io.NopCloser(io.LimitReader(res.Body, int64(maxBodySize)))

	if decompress {
		switch strings.ToLower(res.Header.Get("Content-Encoding")) {
		case "gzip":
			r.Decompressed = true

			bodyReader, err = gzip.NewReader(bodyReader)
			if err != nil {
				return fmt.Errorf("create gzip reader: %w", err)
			}

			defer func() {
				err := bodyReader.Close()
				if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && finalError == nil {
					finalError = fmt.Errorf("close gzip reader: %w", err)
				}
			}()
		default:
		}
	}

	r.Body, err = io.ReadAll(bodyReader)
	if err != nil && !(errors.Is(err, io.ErrUnexpectedEOF) && r.Decompressed) {
		return err
	}

	r.BodyStats, err = Count(bytes.NewReader(r.Body))

	return err
}

// ExtractBody extracts data from the HTTP response body.
func (r *Response) ExtractBody(targets []*regexp.Regexp) {
	r.Extract = append(r.Extract, extractRegexp(r.Body, targets)...)
}

// ExtractBodyCommand extracts data from the HTTP response body by running an external command.
func (r *Response) ExtractBodyCommand(ctx context.Context, cmds [][]string) (err error) {
	// pass values in environment variables
	env := make([]string, 0, len(r.Values)+1)

	if len(r.Values) > 0 {
		env = append(env, "MONSOON_VALUE="+r.Values[0])
	}

	for i, v := range r.Values {
		env = append(env, fmt.Sprintf("MONSOON_VALUE%d=%s", i+1, v))
	}

	data, err := extractCommand(ctx, env, r.Body, cmds)
	if err != nil {
		return err
	}

	r.Extract = append(r.Extract, data...)
	return nil
}

// ExtractHeader extracts data from an HTTP header. This fills r.Header.
func (r *Response) ExtractHeader(res *http.Response, targets []*regexp.Regexp) error {
	buf, err := httputil.DumpResponse(res, false)
	if err != nil {
		return err
	}

	r.RawHeader = buf
	r.Header, err = Count(bytes.NewReader(buf))
	r.Extract = append(r.Extract, extractRegexp(buf, targets)...)

	return err
}

// TextStats reports statistics about some text.
type TextStats struct {
	Bytes int `json:"bytes"`
	Words int `json:"words"`
	Lines int `json:"lines"`
}

// Count counts the bytes, words and lines in the body.
func Count(rd io.Reader) (TextStats, error) {
	var stats TextStats

	bufReader := bufio.NewReader(rd)
	var previous, current byte
	for {
		current, err := bufReader.ReadByte()
		if err == io.EOF {
			break
		}

		if err != nil {
			return TextStats{}, err
		}

		stats.Bytes++
		if current != '\n' && unicode.IsSpace(rune(current)) && !unicode.IsSpace(rune(previous)) {
			stats.Words++
		}

		if current == '\n' {
			stats.Lines++
		}

		previous = current
	}

	if stats.Bytes > 0 && !unicode.IsSpace(rune(current)) {
		stats.Words++
	}

	return stats, nil
}
