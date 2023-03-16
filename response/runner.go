package response

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/spf13/pflag"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

// Runner executes HTTP requests.
type Runner struct {
	Template *request.Request

	MaxBodySize int
	Extract     []*regexp.Regexp

	Client    *http.Client
	Transport *http.Transport

	input  <-chan []string
	output chan<- Response
}

const (
	// DefaultMaxBodySize is the default size for peeking at the body to extract strings via regexp.
	DefaultMaxBodySize = 5 * 1024 * 1024

	// DefaultConnectTimeout limits how long the TCP connection setup can take.
	DefaultConnectTimeout = 30 * time.Second

	// DefaultTLSHandshakeTimeout limits the time until a TLS connection must be established.
	DefaultTLSHandshakeTimeout = 10 * time.Second

	// DefaultResponseHeaderTimeout limits the time until the first HTTP
	// response header must have been received.
	DefaultResponseHeaderTimeout = 10 * time.Second
)

type TransportOptions struct {
	SkipCertificateVerification         bool
	EnableInsecureCiphersAndTLSVersions bool
	TLSClientCertKeyFilename            string
	DisableHTTP2                        bool
	Network                             string

	ConnectTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
}

func (t TransportOptions) Valid() error {
	return nil
}

func AddTransportFlags(fs *pflag.FlagSet, opts *TransportOptions) {
	// Transport
	fs.BoolVarP(&opts.SkipCertificateVerification, "insecure", "k", false, "disable TLS certificate verification")
	fs.BoolVar(&opts.EnableInsecureCiphersAndTLSVersions, "insecure-ciphersuites", false, "enable insecure ciphersuites and TLS versions")
	fs.StringVar(&opts.TLSClientCertKeyFilename, "client-cert", "", "read TLS client key and cert from `file`")
	fs.BoolVar(&opts.DisableHTTP2, "disable-http2", false, "do not try to negotiate an HTTP2 connection")

	fs.DurationVar(&opts.ConnectTimeout, "connect-timeout", DefaultConnectTimeout, "limit TCP connection establishment")
	fs.DurationVar(&opts.TLSHandshakeTimeout, "tls-handshake-timeout", DefaultTLSHandshakeTimeout, "limit TLS connection establishment")
	fs.DurationVar(&opts.ResponseHeaderTimeout, "response-header-timeout", DefaultResponseHeaderTimeout, "limit time until the first response header line must have been received")
}

// NewTransport creates a new shared transport for clients to use.
func NewTransport(opts TransportOptions, concurrentRequests int) (*http.Transport, error) {
	err := opts.Valid()
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}

	if concurrentRequests == 0 {
		return nil, errors.New("concurrentRequests is zero")
	}

	// for timeouts, see
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/

	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = DefaultConnectTimeout
	}

	if opts.TLSHandshakeTimeout == 0 {
		opts.TLSHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	if opts.ResponseHeaderTimeout == 0 {
		opts.ResponseHeaderTimeout = DefaultResponseHeaderTimeout
	}

	if opts.Network == "" {
		opts.Network = "tcp"
	}

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   opts.TLSHandshakeTimeout,
		ResponseHeaderTimeout: opts.ResponseHeaderTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       15 * time.Second,
		TLSClientConfig:       &tls.Config{},
		MaxIdleConns:          concurrentRequests,
		MaxIdleConnsPerHost:   concurrentRequests,
	}

	dialer := &net.Dialer{
		Timeout:   opts.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	}

	noProxy := len(os.Getenv("NO_PROXY")) > 0 || len(os.Getenv("no_proxy")) > 0

	socks5ProxyConfig := os.Getenv("FORCE_SOCKS5_PROXY")
	if socks5ProxyConfig == "" || noProxy {
		tr.DialContext = dialer.DialContext
	} else {
		// configure a socks5 proxy that also forwards requests
		// to loopback devices through the proxy connection
		socks5Dialer, err := socks5ContextDialer(dialer, socks5ProxyConfig)
		if err != nil {
			return nil, fmt.Errorf("configure socks5 proxy: %v", err)
		}

		tr.DialContext = socks5Dialer.DialContext
	}

	// modify dialer so we can force a certain network (e.g. tcp6 instead of tcp)
	originalDialContext := tr.DialContext
	tr.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return originalDialContext(ctx, opts.Network, addr)
	}

	tr.TLSClientConfig.InsecureSkipVerify = opts.SkipCertificateVerification
	if opts.EnableInsecureCiphersAndTLSVersions {
		tr.TLSClientConfig.CipherSuites = getAllCipherSuiteIDs()
		tr.TLSClientConfig.MinVersion = tls.VersionTLS10
	}

	if !opts.DisableHTTP2 {
		// enable http2
		err := http2.ConfigureTransport(tr)
		if err != nil {
			return nil, err
		}
	}

	if opts.TLSClientCertKeyFilename != "" {
		certs, key, err := readPEMCertKey(opts.TLSClientCertKeyFilename)
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

func getAllCipherSuiteIDs() []uint16 {
	allCiphers := make([]uint16, 0)
	// Adding all cipher suites, to communicate with most servers
	for _, v := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
		allCiphers = append(allCiphers, v.ID)
	}
	return allCiphers
}

func socks5ContextDialer(dialer proxy.Dialer, socks5Conf string) (proxy.ContextDialer, error) {
	socks5URL, err := url.Parse("socks5://" + socks5Conf)
	if err != nil {
		return nil, fmt.Errorf("parse socks5 configuration: %v", err)
	}

	socks5Dialer, err := proxy.FromURL(socks5URL, dialer)
	if err != nil {
		return nil, err
	}

	contextDialer, ok := socks5Dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("socks5 dialer does not support context")
	}

	return contextDialer, nil
}

// readPEMCertKey reads a file and returns the PEM encoded certificate and key
// blocks.
func readPEMCertKey(filename string) (certs []byte, key []byte, err error) {
	data, err := os.ReadFile(filename)
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
func NewRunner(tr *http.Transport, template *request.Request, input <-chan []string, output chan<- Response) *Runner {
	c := &http.Client{
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Runner{
		Template:    template,
		Client:      c,
		Transport:   tr,
		input:       input,
		output:      output,
		MaxBodySize: DefaultMaxBodySize,
	}
}

type InvalidRequest struct {
	Err error
}

func (ir InvalidRequest) Error() string {
	return "invalid request: " + ir.Err.Error()
}

func (r *Runner) request(ctx context.Context, values []string) (response Response) {
	response.Values = values

	req, err := r.Template.Apply(values)
	if err != nil {
		response.Error = InvalidRequest{err}
		return
	}

	response.URL = req.URL.String()

	start := time.Now()
	res, err := r.Client.Do(req.WithContext(ctx))
	response.Duration = time.Since(start)
	if err != nil {
		response.Error = err
		return
	}

	// dump the header and extract data now so the stats about the header are
	// present when the filter runs in the next step. We need to dump the header
	// for that, so we can easily run data extraction in the same step.
	err = response.ExtractHeader(res, r.Extract)
	if err != nil {
		response.Error = err
		return
	}

	parsed_header, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(response.RawHeader)), nil)
	if err != nil {
		response.Error = err
		return
	}
	response.ParsedHeader = parsed_header.Header

	err = response.ReadBody(res.Body, r.MaxBodySize)
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
