package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
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
		return fmt.Sprintf("error: %v", r.Error)
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

// ExtractBody extracts data from an HTTP response body. The body is drained in
// the process. This fills r.Body and r.Extract.
func (r *Response) ExtractBody(body io.Reader, buf []byte, targets []*regexp.Regexp) error {
	n, err := io.ReadFull(body, buf)
	buf = buf[:n]
	if err == io.EOF {
		err = nil
	}

	r.Extract = append(r.Extract, extractRegexp(buf, targets)...)
	bodyReader := io.MultiReader(bytes.NewReader(buf), body)
	r.Body, err = Count(bodyReader)
	return err
}

// ExtractHeader extracts data from an HTTP header. This fills r.Header.
func (r *Response) ExtractHeader(res *http.Response, targets []*regexp.Regexp) error {
	buf := bytes.NewBuffer(nil)
	err := res.Header.Write(buf)
	if err != nil {
		return err
	}

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
