package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"unicode"
)

// Response is an HTTP response.
type Response struct {
	Item  string
	URL   string
	Error error

	Bytes, Words, Lines int

	HTTPResponse *http.Response
}

func (r Response) String() string {
	if r.Error != nil {
		return fmt.Sprintf("error: %v", r.Error)
	}

	res := r.HTTPResponse
	status := fmt.Sprintf("%7d %7d %7d %7v -- %v", r.Bytes, r.Words, r.Lines, res.StatusCode, r.Item)
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, ok := res.Header["Location"]
		if ok {
			status += fmt.Sprintf(", Location: %v", loc[0])
		}
	}
	return status
}

// ReadBody counts the bytes, words and lines in the body.
func ReadBody(rd io.Reader) (bytes, words, lines int, err error) {
	bufReader := bufio.NewReader(rd)
	var previous, current byte
	for {
		current, err = bufReader.ReadByte()
		if err == io.EOF {
			err = nil
			return
		}

		if err != nil {
			return
		}

		bytes++
		if unicode.IsSpace(rune(current)) && !unicode.IsSpace(rune(previous)) {
			words++
		}

		if current == '\n' {
			lines++
		}
	}
}
