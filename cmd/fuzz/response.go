package fuzz

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Response is an HTTP response.
type Response struct {
	Item  string
	URL   string
	Error error

	Header, Body TextStats
	Extract      []string

	HTTPResponse *http.Response
	RawBody      []byte
	RawHeader    []byte
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

		return fmt.Sprintf("%7s %18s   %v", "error", r.Error, r.Item)
	}

	res := r.HTTPResponse
	status := fmt.Sprintf("%7d %8d %8d   %-8v", res.StatusCode, r.Header.Bytes, r.Body.Bytes, r.Item)
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, ok := res.Header["Location"]
		if ok {
			status += fmt.Sprintf(", Location: %v", loc[0])
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

func extractCommand(buf []byte, cmds [][]string) (data []string, err error) {
	for _, command := range cmds {
		if len(command) < 1 {
			panic("command is invalid")
		}
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Stdin = bytes.NewReader(buf)
		cmd.Stderr = os.Stderr

		buf, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		data = append(data, string(buf))
	}
	return data, nil
}

// ReadBody reads at most maxBodySize bytes from the body and saves it to a buffer in the
// Respons struct for later processing.
func (r *Response) ReadBody(body io.Reader, maxBodySize int) error {
	r.RawBody = make([]byte, maxBodySize)

	n, err := io.ReadFull(body, r.RawBody)
	if n == 0 && err == io.EOF {
		err = nil
	}

	r.RawBody = r.RawBody[:n]
	if err == io.ErrUnexpectedEOF {
		err = nil
	}

	return err
}

// ExtractBody extracts data from the HTTP response body.
func (r *Response) ExtractBody(targets []*regexp.Regexp, cmds [][]string) (err error) {
	r.Extract = append(r.Extract, extractRegexp(r.RawBody, targets)...)
	data, err := extractCommand(r.RawBody, cmds)
	if err != nil {
		return err
	}
	r.Extract = append(r.Extract, data...)
	r.Body, err = Count(bytes.NewReader(r.RawBody))
	return err
}

// ExtractHeader extracts data from an HTTP header. This fills r.Header.
func (r *Response) ExtractHeader(res *http.Response, targets []*regexp.Regexp) error {
	buf := bytes.NewBuffer(nil)
	err := res.Header.Write(buf)
	if err != nil {
		return err
	}

	r.RawHeader = buf.Bytes()
	r.Header, err = Count(bytes.NewReader(buf.Bytes()))
	r.Extract = append(r.Extract, extractRegexp(buf.Bytes(), targets)...)

	return err
}

// TextStats reports statistics about some text.
type TextStats struct {
	Bytes, Words, Lines int
}

// Count counts the bytes, words and lines in the body.
func Count(rd io.Reader) (stats TextStats, err error) {
	bufReader := bufio.NewReader(rd)
	var previous, current byte
	for {
		current, err = bufReader.ReadByte()
		if err == io.EOF {
			err = nil
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
