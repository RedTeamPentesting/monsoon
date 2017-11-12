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

	client *http.Client

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
	c := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			IdleConnTimeout:       15 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Runner{
		URL:            url,
		client:         c,
		t:              t,
		input:          input,
		output:         output,
		BodyBufferSize: DefaultBodyBufferSize,
	}
}

func (r *Runner) request(ctx context.Context, item string) (response Response) {
	url := strings.Replace(r.URL, "FUZZ", item, -1)

	response = Response{
		URL:  url,
		Item: item,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		response.Error = err
		return
	}

	req.Header.Add("Accept", "*/*")

	res, err := r.client.Do(req.WithContext(ctx))
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
