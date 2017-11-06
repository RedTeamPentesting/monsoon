package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
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

// Run processes items read from ch and executes HTTP requests.
func (r *Runner) Run(ctx context.Context, input <-chan string) {
	for item := range input {
		url := strings.Replace(r.URL, "FUZZ", item, -1)
		res, err := r.client.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		n, err := io.Copy(ioutil.Discard, res.Body)
		if err != nil {
			_ = res.Body.Close()
			fmt.Printf("error reading body: %v\n", err)
			continue
		}

		err = res.Body.Close()
		if err != nil {
			fmt.Printf("error closing body: %v\n", err)
		}

		// if res.StatusCode == 404 {
		// 	continue
		// }

		fmt.Printf("res: %v (%v) -> %v\n", res.Status, res.StatusCode, n)
	}
}
