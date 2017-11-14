package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	tomb "gopkg.in/tomb.v2"
)

// Runner executes HTTP requests.
type Runner struct {
	URL            string
	BodyBufferSize int
	Extract        []*regexp.Regexp

	RequestMethod string
	Body          string
	Header        http.Header

	Client    *http.Client
	Transport *http.Transport

	t      *tomb.Tomb
	input  <-chan string
	output chan<- Response
}

// DefaultBodyBufferSize is the default size for peeking at the body to extract strings via regexp.
const DefaultBodyBufferSize = 5 * 1024 * 1024

// NewRunner returns a new runner to execute HTTP requests.
func NewRunner(t *tomb.Tomb, url string, input <-chan string, output chan<- Response) *Runner {
	// for timeouts, see
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
	c := &http.Client{
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Runner{
		URL:            url,
		Client:         c,
		Transport:      tr,
		t:              t,
		input:          input,
		output:         output,
		BodyBufferSize: DefaultBodyBufferSize,
	}
}

func (r *Runner) request(ctx context.Context, item string) (response Response) {
	insertItem := func(s string) string {
		if !strings.Contains(s, "FUZZ") {
			return s
		}

		return strings.Replace(s, "FUZZ", item, -1)
	}

	url := insertItem(r.URL)
	response = Response{
		URL:  url,
		Item: item,
	}

	req, err := http.NewRequest(insertItem(r.RequestMethod), url, strings.NewReader(insertItem(r.Body)))
	if err != nil {
		response.Error = err
		return
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "monsoon")

	for k, vs := range r.Header {
		k = insertItem(k)
		if len(vs) == 1 {
			req.Header.Set(k, insertItem(vs[0]))
			continue
		}

		for _, v := range vs {
			req.Header.Add(k, insertItem(v))
		}
	}

	res, err := r.Client.Do(req.WithContext(ctx))
	if err != nil {
		response.Error = err
		return
	}

	bodyBuf := make([]byte, r.BodyBufferSize)
	err = response.ExtractBody(res.Body, bodyBuf, r.Extract)
	if err != nil {
		response.Error = err
		return
	}

	err = res.Body.Close()
	if err != nil {
		response.Error = err
		return
	}

	response.HTTPResponse = res

	buf := bytes.NewBuffer(nil)
	err = res.Header.Write(buf)
	if err == nil {
		_ = response.ExtractHeader(buf)
	}

	return
}

// Run processes items read from ch and executes HTTP requests.
func (r *Runner) Run() error {
	for item := range r.input {
		res := r.request(r.t.Context(context.Background()), item)

		select {
		case <-r.t.Dying():
			return nil
		case r.output <- res:
		}
	}

	return nil
}
