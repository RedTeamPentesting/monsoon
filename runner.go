package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Runner executes HTTP requests.
type Runner struct {
	URL    string
	client *http.Client
}

// NewRunner returns a new runner to execute HTTP requests.
func NewRunner(url string) *Runner {
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
		URL:    url,
		client: c,
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

	response.Body, err = Count(res.Body)
	if err != nil {
		_ = res.Body.Close()
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
		response.Header, _ = Count(buf)
	}

	return
}

// Run processes items read from ch and executes HTTP requests.
func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup, input <-chan string, output chan<- Response) {
	defer wg.Done()
	for item := range input {
		res := r.request(ctx, item)

		select {
		case <-ctx.Done():
			return
		case output <- res:
		}
	}
}
