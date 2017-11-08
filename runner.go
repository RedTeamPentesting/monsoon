package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
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

// Response is an HTTP response.
type Response struct {
	Item  string
	URL   string
	Error error

	BodyLength   int64
	HTTPResponse *http.Response
}

func (r Response) String() string {
	if r.Error != nil {
		return fmt.Sprintf("error: %v", r.Error)
	}

	res := r.HTTPResponse
	status := fmt.Sprintf("%v -> %v", r.URL, res.StatusCode)
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, ok := res.Header["Location"]
		if ok {
			status += fmt.Sprintf(", Location: %v", loc[0])
		}
	}
	return status
}

func (r *Runner) request(ctx context.Context, url string) (*http.Response, int64, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	res, err := r.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, 0, err
	}

	n, err := io.Copy(ioutil.Discard, res.Body)
	if err != nil {
		_ = res.Body.Close()
		return nil, 0, err
	}

	err = res.Body.Close()
	if err != nil {
		return nil, 0, err
	}

	return res, n, nil
}

// Run processes items read from ch and executes HTTP requests.
func (r *Runner) Run(ctx context.Context, wg *sync.WaitGroup, input <-chan string, output chan<- Response) {
	defer wg.Done()
	for item := range input {
		url := strings.Replace(r.URL, "FUZZ", item, -1)

		response := Response{
			URL:  url,
			Item: item,
		}

		res, bodyBytes, err := r.request(ctx, url)
		if err != nil {
			response.Error = err
		} else {
			response.BodyLength = bodyBytes
			response.HTTPResponse = res
		}

		select {
		case <-ctx.Done():
			return
		case output <- response:
		}
	}
}
