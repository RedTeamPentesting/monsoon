package fuzz

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fd0/termstatus"
	"github.com/spf13/cobra"
	tomb "gopkg.in/tomb.v2"
)

// RunOptions collect options for a run.
type RunOptions struct {
	Range       string
	RangeFormat string
	Filename    string
	Logfile     string
	Logdir      string
	Threads     int

	BufferSize int
	Skip       int
	Limit      int

	Method         string
	Data           string
	Header         []string
	header         http.Header
	FollowRedirect int

	HideStatusCodes []int
	HideHeaderSize  []string
	HideBodySize    []string
	HidePattern     []string
	hidePattern     []*regexp.Regexp
	ShowPattern     []string
	showPattern     []*regexp.Regexp

	Extract        []string
	extract        []*regexp.Regexp
	ExtractPipe    []string
	extractPipe    [][]string
	BodyBufferSize int

	Insecure bool
}

var runOptions RunOptions

func compileRegexps(pattern []string) (res []*regexp.Regexp, err error) {
	for _, pat := range pattern {
		r, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("regexp %q failed to compile: %v", pat, err)
		}

		res = append(res, r)
	}

	return res, nil
}

func splitShell(cmds []string) ([][]string, error) {
	var data [][]string
	for _, cmd := range cmds {
		args, err := SplitShellStrings(cmd)
		if err != nil {
			return nil, err
		}
		if len(args) < 1 {
			return nil, fmt.Errorf("invalid command: %q", cmd)
		}
		data = append(data, args)
	}
	return data, nil
}

// Valid validates the options and returns an error if something is invalid.
func (opts *RunOptions) Valid() (err error) {
	if opts.Range != "" && opts.Filename != "" {
		return errors.New("only one source allowed but both range and filename specified")
	}

	opts.extract, err = compileRegexps(opts.Extract)
	if err != nil {
		return err
	}

	opts.extractPipe, err = splitShell(opts.ExtractPipe)
	if err != nil {
		return err
	}

	opts.hidePattern, err = compileRegexps(opts.HidePattern)
	if err != nil {
		return err
	}

	opts.showPattern, err = compileRegexps(opts.ShowPattern)
	if err != nil {
		return err
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

var cmdFuzz = &cobra.Command{
	Use:     "fuzz [options] URL",
	Short:   "Execute and filter HTTP requests",
	Example: longHelpText,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(&runOptions, args)
	},
}

// AddCommand adds the 'run' command to cmd.
func AddCommand(cmd *cobra.Command) {
	cmd.AddCommand(cmdFuzz)

	fs := cmdFuzz.Flags()
	fs.SortFlags = false

	fs.StringVarP(&runOptions.Range, "range", "r", "", "set range `from-to`")
	fs.StringVar(&runOptions.RangeFormat, "range-format", "%d", "set `format` for range")

	fs.StringVarP(&runOptions.Filename, "file", "f", "", "read values from `filename`")
	fs.StringVar(&runOptions.Logfile, "logfile", "", "write copy of printed messages to `filename`")
	fs.StringVar(&runOptions.Logdir, "logdir", os.Getenv("MONSOON_LOG_DIR"), "automatically log all output to files in `dir`")

	fs.IntVarP(&runOptions.Threads, "threads", "t", 5, "make as many as `n` parallel requests")
	fs.IntVar(&runOptions.BufferSize, "buffer-size", 100000, "set number of buffered items to `n`")
	fs.IntVar(&runOptions.Skip, "skip", 0, "skip the first `n` requests")
	fs.IntVar(&runOptions.Limit, "limit", 0, "only run `n` requests, then exit")

	fs.StringVar(&runOptions.Method, "request", "GET", "use HTTP request `method`")
	fs.MarkDeprecated("request", "use --method")
	fs.StringVarP(&runOptions.Method, "method", "X", "GET", "use HTTP request `method`")

	fs.StringVarP(&runOptions.Data, "data", "d", "", "transmit `data` in the HTTP request body")
	fs.StringArrayVarP(&runOptions.Header, "header", "H", nil, "add `\"name: value\"` as an HTTP request header")
	fs.IntVar(&runOptions.FollowRedirect, "follow-redirect", 0, "follow `n` redirects")

	fs.IntSliceVar(&runOptions.HideStatusCodes, "hide-status", nil, "hide http responses with this status `code,[code],[...]`")
	fs.StringSliceVar(&runOptions.HideHeaderSize, "hide-header-size", nil, "hide http responses with this header size (`size,from-to,from-,-to`)")
	fs.StringSliceVar(&runOptions.HideBodySize, "hide-body-size", nil, "hide http responses with this body size (`size,from-to,from-,-to`)")
	fs.StringArrayVar(&runOptions.HidePattern, "hide-pattern", nil, "hide all responses containing `regex` in response header or body (can be specified multiple times)")
	fs.StringArrayVar(&runOptions.ShowPattern, "show-pattern", nil, "show only responses containing `regex` in response header or body (can be specified multiple times)")

	fs.StringArrayVar(&runOptions.Extract, "extract", nil, "extract `regex` from response body (can be specified multiple times)")
	fs.StringArrayVar(&runOptions.ExtractPipe, "extract-pipe", nil, "pipe response body to `cmd` to extract data (can be specified multiple times)")
	fs.IntVar(&runOptions.BodyBufferSize, "body-buffer-size", 5, "use `n` MiB as the buffer size for extracting strings from a response body")

	fs.BoolVarP(&runOptions.Insecure, "insecure", "k", false, "disable TLS certificate verification")
}

const longHelpText = `
Use the file filenames.txt as input, hide all 200 and 404 responses:

    monsoon fuzz --file filenames.txt \
      --hide-status 200,404 \
      https://example.com/FUZZ

Skip the first 23 entries in filenames.txt and send at most 2000 requests:

    monsoon fuzz --file filenames.txt \
      --skip 23 \
      --limit 2000 \
      --hide-status 404 \
      https://example.com/FUZZ

Hide responses with body size between 100 and 200 bytes (inclusive), exactly
533 bytes or more than 10000 bytes:

    monsoon fuzz --file filenames.txt \
      --hide-body-size 100-200,533,10000- \
      https://example.com/FUZZ

Try all strings in passwords.txt as the password for the admin user, using an
HTTP POST request:

    monsoon fuzz --file passwords.txt \
      --method POST \
      --data 'username=admin&password=FUZZ' \
      --hide-status 403 \
      https://example.com/login

Run requests with a range from 100 to 500 with the request value inserted into
the cookie "sessionid":

    monsoon fuzz --range 100-500 \
      --header 'Cookie: sessionid=FUZZ' \
      --hide-status 500 https://example.com/login/session

Request 500 session IDs and extract the cookie values (matching case insensitive):

    monsoon fuzz --range 1-500 \
      --extract '(?i)Set-Cookie: (.*)' \
      https://example.com/login

Hide responses which contain a Date header with an uneven number of seconds:

    monsoon fuzz --range 1-500 \
      --hide-pattern 'Date: .* ..:..:.[13579] GMT' \
      https://example.com/FUZZ

Only show responses which contain the pattern "The secret is: " in the response:

    monsoon fuzz --range 1-500 \
      --show-pattern 'The secret is: ' \
      https://example.com/FUZZ


Filter Evaluation Order
#######################

The filters are evaluated in the following order. A response is displayed if:

 * The status code is not hidden (--hide-status)
 * The header and body size are not hidden (--header-size, --body-size)
 * The header and body does not contain a hide pattern (--hide-pattern)
 * The header or body contain all show pattern (--show-pattern, if specified)


References
##########

The regular expression syntax documentation can be found here:
https://golang.org/pkg/regexp/syntax/#hdr-Syntax
`

func run(opts *RunOptions, args []string) error {
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

	inputURL := args[0]

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

	var term Terminal
	if opts.Logdir != "" && opts.Logfile == "" {
		url, err := url.Parse(inputURL)
		if err != nil {
			return err
		}

		ts := time.Now().Format("20060102_150405")
		fn := fmt.Sprintf("monsoon_%s_%s.log", url.Host, ts)
		opts.Logfile = filepath.Join(opts.Logdir, fn)
	}

	termTomb, _ := tomb.WithContext(rootCtx)

	if opts.Logfile != "" {
		fmt.Printf("logfile is %s\n", opts.Logfile)

		logfile, err := os.Create(opts.Logfile)
		if err != nil {
			return err
		}

		fmt.Fprintln(logfile, recreateCommandline(os.Args))

		// write copies of messages to logfile
		term = &LogTerminal{
			Terminal: termstatus.New(os.Stdout, os.Stderr, false),
			w:        logfile,
		}
	} else {
		term = termstatus.New(os.Stdout, os.Stderr, false)
	}

	termTomb.Go(func() error {
		term.Run(termTomb.Context(rootCtx))
		return nil
	})

	// make sure error messages logged via the log package are printed nicely
	w := NewStdioWrapper(term)
	log.SetOutput(w.Stderr())

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

	filters := []ResponseFilter{
		NewFilterStatusCode(opts.HideStatusCodes),
	}

	if len(opts.HideHeaderSize) > 0 || len(opts.HideBodySize) > 0 {
		f, err := NewFilterSize(opts.HideHeaderSize, opts.HideBodySize)
		if err != nil {
			return err
		}
		filters = append(filters, f)
	}

	if len(opts.hidePattern) > 0 {
		filters = append(filters, FilterRejectPattern{Pattern: opts.hidePattern})
	}

	if len(opts.showPattern) > 0 {
		filters = append(filters, FilterAcceptPattern{Pattern: opts.showPattern})
	}

	term.Printf("input URL %v\n\n", inputURL)

	outputChan := make(chan string, opts.BufferSize)
	inputChan := outputChan
	outputCountChan := make(chan int, 1)
	inputCountChan := outputCountChan

	var valueFilters []ValueFilter

	if opts.Skip > 0 {
		valueFilters = append(valueFilters, &ValueFilterSkip{Skip: opts.Skip})
	}

	if opts.Limit > 0 {
		valueFilters = append(valueFilters, &ValueFilterLimit{Max: opts.Limit})
	}

	for _, f := range valueFilters {
		outputChan, outputCountChan, err = RunValueFilter(ctx, f, outputChan, outputCountChan)
		if err != nil {
			return err
		}
	}

	prodTomb, _ := tomb.WithContext(ctx)
	err = producer.Start(prodTomb, inputChan, inputCountChan)
	if err != nil {
		return fmt.Errorf("unable to start: %v", err)
	}

	responseChannel := make(chan Response)

	runnerTomb, _ := tomb.WithContext(ctx)
	for i := 0; i < opts.Threads; i++ {
		runner := NewRunner(runnerTomb, inputURL, outputChan, responseChannel)
		runner.BodyBufferSize = opts.BodyBufferSize * 1024 * 1024
		runner.Extract = opts.extract
		runner.ExtractPipe = opts.extractPipe
		runner.Method = opts.Method
		runner.Header = opts.header
		runner.Body = opts.Data
		runner.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) <= opts.FollowRedirect {
				return nil
			}
			return http.ErrUseLastResponse
		}
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
	displayTomb.Go(reporter.Display(responseChannel, outputCountChan))
	<-displayTomb.Dead()

	termTomb.Kill(nil)
	return termTomb.Wait()
}