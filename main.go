package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/fd0/termstatus"
	"github.com/spf13/cobra"
	tomb "gopkg.in/tomb.v2"
)

// GlobalOptions contains global options.
type GlobalOptions struct {
	Range       string
	RangeFormat string
	Filename    string
	Threads     int
	BufferSize  int

	RequestMethod string
	Data          string
	Header        []string
	header        http.Header

	HideStatusCodes []int
	HideHeaderSize  []string
	HideBodySize    []string

	Extract        []string
	extract        []*regexp.Regexp
	BodyBufferSize int

	PrintVersion bool
	Insecure     bool
}

// Valid validates the options and returns an error if something is invalid.
func (opts *GlobalOptions) Valid() error {
	if opts.Range != "" && opts.Filename != "" {
		return errors.New("only one source allowed but both range and filename specified")
	}

	for _, extract := range opts.Extract {
		r, err := regexp.Compile(extract)
		if err != nil {
			return fmt.Errorf("regexp %q failed to compile: %v", extract, err)
		}

		opts.extract = append(opts.extract, r)
	}

	opts.header = http.Header{}
	for _, s := range opts.Header {
		data := strings.SplitN(s, ":", 2)
		name := data[0]
		var val string
		if len(data) > 1 {
			val = data[1]
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
		}
		opts.header.Add(name, val)
	}

	return nil
}

var globalOptions GlobalOptions

func init() {
	fs := cmdRoot.Flags()
	fs.SortFlags = false

	fs.StringVarP(&globalOptions.Range, "range", "r", "", "set range `from-to`")
	fs.StringVar(&globalOptions.RangeFormat, "range-format", "%d", "set `format` for range")

	fs.StringVarP(&globalOptions.Filename, "file", "f", "", "read values from `filename`")

	fs.IntVarP(&globalOptions.Threads, "threads", "t", 5, "make as many as `n` parallel requests")
	fs.IntVar(&globalOptions.BufferSize, "buffer-size", 100000, "set number of buffered items to `n`")

	fs.StringVarP(&globalOptions.RequestMethod, "request", "X", "GET", "use HTTP request `method`")
	fs.StringVarP(&globalOptions.Data, "data", "d", "", "transmit `data` in the HTTP request body")
	fs.StringArrayVarP(&globalOptions.Header, "header", "H", nil, "add `name: value` as an HTTP request header")

	fs.IntSliceVar(&globalOptions.HideStatusCodes, "hide-status", nil, "hide http responses with this status `code,[code],[...]`")
	fs.StringSliceVar(&globalOptions.HideHeaderSize, "hide-header-size", nil, "hide http responses with this header size (`size,from-to,from-,-to`)")
	fs.StringSliceVar(&globalOptions.HideBodySize, "hide-body-size", nil, "hide http responses with this body size (`size,from-to,from-,-to`)")

	fs.StringArrayVar(&globalOptions.Extract, "extract", nil, "extract `regex` from response body (can be specified multiple times)")
	fs.IntVar(&globalOptions.BodyBufferSize, "body-buffer-size", 5, "use `n` MiB as the buffer size for extracting strings from a response body")

	fs.BoolVar(&globalOptions.PrintVersion, "version", false, "print version")
	fs.BoolVarP(&globalOptions.Insecure, "insecure", "k", false, "disable TLS certificate verification")
}

const longHelpText = `
Monsoon is a fast HTTP enumerator which allows fine-grained control over the
displayed HTTP responses

Examples:

Use the file filenames.txt as input, hide all 200 and 404 responses:

    monsoon --file filenames.txt \
      --hide-status 200,404 \
      https://example.com/FUZZ

Hide responses with body size between 100 and 200 bytes (inclusive), exactly
533 bytes or more than 10000 bytes:

    monsoon --file filenames.txt \
      --hide-body-size 100-200,533,10000- \
      https://example.com/FUZZ

Try all strings in passwords.txt as the password for the admin user, using an
HTTP POST request:

    monsoon --file passwords.txt \
      --request POST \
      --data 'username=admin&password=FUZZ' \
      --hide-status 403 \
      https://example.com/login

Run requests with a range from 100 to 500 with the request value inserted into
the cookie "sessionid":

    monsoon --range 100-500 \
      --header 'Cookie: sessionid=FUZZ' \
      --hide-status 500 https://example.com/login/session

Request 500 session IDs and extract the cookie values (matching case insensitive):

    monsoon --range 1-500 \
      --extract '(?i)Set-Cookie: (.*)' \
      https://example.com/login

The regular expression syntax documentation can be found here:
https://golang.org/pkg/regexp/syntax/#hdr-Syntax
`

var cmdRoot = &cobra.Command{
	Use:           "monsoon [flags] URL",
	Long:          longHelpText,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(&globalOptions, args)
	},
}

func main() {
	err := cmdRoot.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(1)
	}
}

// Producer yields values for enumerating.
type Producer interface {
	Start(*tomb.Tomb, chan<- string, chan<- int) error
}

var version = "compiled manually"

func run(opts *GlobalOptions, args []string) error {
	if opts.PrintVersion {
		fmt.Printf("monsoon %s\ncompiled with %v on %v\n",
			version, runtime.Version(), runtime.GOOS)
		return nil
	}

	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	err := opts.Valid()
	if err != nil {
		return err
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	term := termstatus.New(rootCtx, os.Stdout)

	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT)
	go func() {
		for sig := range signalCh {
			term.Printf("received signal %v\n", sig)
			cancel()
		}
	}()

	url := args[0]

	var producer Producer
	switch {
	case opts.Range != "":
		rp := &RangeProducer{Format: opts.RangeFormat}
		_, err := fmt.Sscanf(opts.Range, "%d-%d", &rp.First, &rp.Last)
		if err != nil {
			return errors.New("wrong format for range, expected: first-last")
		}
		producer = rp
	case opts.Filename != "":
		producer = &FileProducer{Filename: opts.Filename}
	default:
		return errors.New("neither file nor range specified, nothing to do")
	}

	filters := []Filter{
		NewFilterStatusCode(opts.HideStatusCodes),
	}

	if len(opts.HideHeaderSize) > 0 || len(opts.HideBodySize) > 0 {
		f, err := NewFilterSize(opts.HideHeaderSize, opts.HideBodySize)
		if err != nil {
			return err
		}
		filters = append(filters, f)
	}

	term.Printf("fuzzing %v\n\n", url)

	producerChannel := make(chan string, opts.BufferSize)
	countChannel := make(chan int, 1)

	prodTomb, _ := tomb.WithContext(ctx)
	err = producer.Start(prodTomb, producerChannel, countChannel)
	if err != nil {
		return fmt.Errorf("unable to read values from file: %v", err)
	}
	go func() {
		// wait until the producer is done, then close the output channel
		<-prodTomb.Dead()
		close(producerChannel)
	}()

	responseChannel := make(chan Response)

	runnerTomb, _ := tomb.WithContext(ctx)
	for i := 0; i < opts.Threads; i++ {
		runner := NewRunner(runnerTomb, url, producerChannel, responseChannel)
		runner.BodyBufferSize = opts.BodyBufferSize * 1024 * 1024
		runner.Extract = opts.extract
		runner.RequestMethod = opts.RequestMethod
		runner.Header = opts.header
		runner.Body = opts.Data
		if opts.Insecure {
			runner.Transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		runnerTomb.Go(runner.Run)
	}

	go func() {
		// wait until the runners are done, then close the output channel
		<-runnerTomb.Dead()
		close(responseChannel)
	}()

	reporter := NewReporter(term, filters)
	displayTomb, _ := tomb.WithContext(ctx)
	displayTomb.Go(reporter.Display(responseChannel, countChannel))
	<-displayTomb.Dead()

	return term.Finish()
}
