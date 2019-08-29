package response

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/happal/monsoon/request"
	"golang.org/x/net/http2"
)

// Runner executes HTTP requests.
type Runner struct {
	Template *request.Request

	BodyBufferSize int
	Extract        []*regexp.Regexp

	Client    *http.Client
	Transport *http.Transport

	input  <-chan string
	output chan<- Response
}

// DefaultBodyBufferSize is the default size for peeking at the body to extract strings via regexp.
const DefaultBodyBufferSize = 5 * 1024 * 1024

// NewTransport creates a new shared transport for clients to use.
func NewTransport(insecure bool, TLSClientCertKeyFilename string, disableHTTP2 bool) (*http.Transport, error) {
	// for timeouts, see
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       15 * time.Second,
		TLSClientConfig:       &tls.Config{},
	}

	if insecure {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}

	if !disableHTTP2 {
		// enable http2
		err := http2.ConfigureTransport(tr)
		if err != nil {
			return nil, err
		}
	}

	if TLSClientCertKeyFilename != "" {
		certs, key, err := readPEMCertKey(TLSClientCertKeyFilename)
		if err != nil {
			return nil, err
		}

		crt, err := tls.X509KeyPair(certs, key)
		if err != nil {
			return nil, fmt.Errorf("parse TLS client cert or key: %v", err)
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{crt}
	}

	return tr, nil
}

// readPEMCertKey reads a file and returns the PEM encoded certificate and key
// blocks.
func readPEMCertKey(filename string) (certs []byte, key []byte, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("ReadFile: %v", err)
	}

	var block *pem.Block
	for {
		if len(data) == 0 {
			break
		}
		block, data = pem.Decode(data)
		if block == nil {
			break
		}

		switch {
		case strings.HasSuffix(block.Type, "CERTIFICATE"):
			certs = append(certs, pem.EncodeToMemory(block)...)
		case strings.HasSuffix(block.Type, "PRIVATE KEY"):
			if key != nil {
				return nil, nil, fmt.Errorf("error loading TLS cert and key from %v: more than one private key found", filename)
			}
			key = pem.EncodeToMemory(block)
		default:
			return nil, nil, fmt.Errorf("error loading TLS cert and key from %v: unknown block type %v found", filename, block.Type)
		}
	}

	return certs, key, nil
}

// NewRunner returns a new runner to execute HTTP requests.
func NewRunner(tr *http.Transport, template *request.Request, input <-chan string, output chan<- Response) *Runner {
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
		input:          input,
		output:         output,
		BodyBufferSize: DefaultBodyBufferSize,
	}
}

func (r *Runner) request(ctx context.Context, item string) (response Response) {
	req, err := r.Template.Apply(item)
	if err != nil {
		response.Error = err
		return
	}

	response = Response{
		URL:  req.URL.String(),
		Item: item,
	}

	start := time.Now()
	res, err := r.Client.Do(req.WithContext(ctx))
	response.Duration = time.Since(start)
	if err != nil {
		response.Error = err
		return
	}

	err = response.ReadBody(res.Body, r.BodyBufferSize)
	if err != nil {
		response.Error = err
		return
	}

	err = response.ExtractHeader(res, r.Extract)
	if err != nil {
		response.Error = err
		return
	}

	response.ExtractBody(r.Extract)

	err = res.Body.Close()
	if err != nil {
		response.Error = err
		return
	}

	response.HTTPResponse = res

	return
}

// Run processes items read from ch and executes HTTP requests.
func (r *Runner) Run(ctx context.Context) {
	for item := range r.input {
		res := r.request(ctx, item)

		select {
		case <-ctx.Done():
			return
		case r.output <- res:
		}
	}
}
