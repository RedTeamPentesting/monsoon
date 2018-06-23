package fuzz

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/happal/monsoon/request"
	tomb "gopkg.in/tomb.v2"
)

// Runner executes HTTP requests.
type Runner struct {
	Template request.Request

	BodyBufferSize int
	Extract        []*regexp.Regexp
	ExtractPipe    [][]string

	Client    *http.Client
	Transport *http.Transport

	t      *tomb.Tomb
	input  <-chan string
	output chan<- Response
}

// DefaultBodyBufferSize is the default size for peeking at the body to extract strings via regexp.
const DefaultBodyBufferSize = 5 * 1024 * 1024

// NewRunner returns a new runner to execute HTTP requests.
func NewRunner(t *tomb.Tomb, template request.Request, input <-chan string, output chan<- Response) *Runner {
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
		Template:       template,
		Client:         c,
		Transport:      tr,
		t:              t,
		input:          input,
		output:         output,
		BodyBufferSize: DefaultBodyBufferSize,
	}
}

func (r *Runner) request(ctx context.Context, item string) (response Response) {
	req, err := r.Template.Apply("FUZZ", item)
	if err != nil {
		response.Error = err
		return
	}

	response = Response{
		URL:  req.URL.String(),
		Item: item,
	}

	res, err := r.Client.Do(req.WithContext(ctx))
	if err != nil {
		response.Error = err
		return
	}

	err = response.ReadBody(res.Body, r.BodyBufferSize)
	if err != nil {
		response.Error = err
		return
	}

	err = response.ExtractBody(r.Extract, r.ExtractPipe)
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

	if err == nil {
		_ = response.ExtractHeader(res, r.Extract)
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
