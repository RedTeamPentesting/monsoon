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
	"syscall"
	"time"

	"github.com/fd0/termstatus"
	"github.com/happal/monsoon/request"
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

	Request        *request.Request // the template for the HTTP request
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

	return nil
}

var cmdFuzz = &cobra.Command{
	Use: "fuzz [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

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

	// add all options to define a request
	runOptions.Request = request.New()
	request.AddFlags(runOptions.Request, fs)

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
	opts.Request.URL = inputURL

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
		runner := NewRunner(runnerTomb, opts.Request, outputChan, responseChannel)
		runner.BodyBufferSize = opts.BodyBufferSize * 1024 * 1024
		runner.Extract = opts.extract
		runner.ExtractPipe = opts.extractPipe

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
