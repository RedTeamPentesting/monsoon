package fuzz

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RedTeamPentesting/monsoon/cli"
	"github.com/RedTeamPentesting/monsoon/producer"
	"github.com/RedTeamPentesting/monsoon/recorder"
	"github.com/RedTeamPentesting/monsoon/reporter"
	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/RedTeamPentesting/monsoon/response"
	"github.com/RedTeamPentesting/monsoon/shell"
	"github.com/fd0/termstatus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// Options collect options for a run.

type Options struct {
	Range       []string
	RangeFormat string
	Filename    string
	Replace     []string

	Logfile string
	Logdir  string
	Threads int

	RequestsPerSecond float64

	BufferSize int
	Skip       int
	Limit      int

	Request *request.Request // the template for the HTTP request

	response.TransportOptions

	FollowRedirect int

	HideStatusCodes []string
	ShowStatusCodes []string
	HideHeaderSize  []string
	HideBodySize    []string
	HidePattern     []string
	hidePattern     []*regexp.Regexp
	ShowPattern     []string
	showPattern     []*regexp.Regexp

	Extract              []string
	extract              []*regexp.Regexp
	ExtractPipe          []string
	extractPipe          [][]string
	MaxBodySize          int
	DisableDecompression bool

	LongRequest time.Duration

	IPv4Only bool
	IPv6Only bool

	IsTest      bool
	Values      []string
	ShowRequest bool
}

var opts Options

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
		args, err := shell.Split(cmd)
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

// valid validates the options and returns an error if something is invalid.
func (opts *Options) valid() (err error) {
	if opts.Threads <= 0 {
		return errors.New("invalid number of threads")
	}

	if len(opts.Range) > 0 && opts.Filename != "" {
		return errors.New("both range and filename specified")
	}

	if len(opts.Range) > 0 && len(opts.Replace) > 0 {
		return errors.New("both range and replace specified, use --replace if you want both")
	}

	if len(opts.Filename) > 0 && len(opts.Replace) > 0 {
		return errors.New("both filename and replace specified, use --replace if you want both")
	}

	if len(opts.Values) > 0 {
		if len(opts.Replace) > 0 || len(opts.Filename) > 0 || len(opts.Range) > 0 {
			return errors.New("if values is specified, range, filename, and replace cannot be used")
		}
	}

	if len(opts.Range) == 0 && opts.Filename == "" && len(opts.Replace) == 0 && !(opts.IsTest && len(opts.Values) > 0) {
		return errors.New("no replace specified, nothing to do")
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

	switch {
	case opts.IPv4Only && opts.IPv6Only:
		return fmt.Errorf("--ipv4-only and --ipv6-only cannot be used together")
	case opts.IPv4Only:
		opts.TransportOptions.Network = "tcp4"
	case opts.IPv6Only:
		opts.TransportOptions.Network = "tcp6"
	}

	var ignoredOptions []string

	if opts.IsTest {
		if opts.Threads > 0 {
			ignoredOptions = append(ignoredOptions, "threads")
		}

		if opts.RequestsPerSecond > 0 {
			ignoredOptions = append(ignoredOptions, "requests-per-second")
		}

		if opts.Skip > 0 {
			ignoredOptions = append(ignoredOptions, "skip")
		}

		if opts.Limit > 0 {
			ignoredOptions = append(ignoredOptions, "limit")
		}

		opts.Limit = 1
		opts.Skip = 0

		if len(opts.Values) > 0 {
			opts.Limit = len(opts.Values)
		}

		if len(ignoredOptions) > 0 {
			fmt.Fprintf(os.Stderr, reporter.Dim("Warning: The following options are ignored in test mode: %s\n"),
				strings.Join(ignoredOptions, ", "))
		}
	}

	return nil
}

var cmd = &cobra.Command{
	Use:                   "fuzz [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(func(ctx context.Context, g *errgroup.Group) error {
			return run(ctx, g, &opts, args)
		})
	},
}

var cmdTest = &cobra.Command{
	Use:                   "test [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(func(ctx context.Context, g *errgroup.Group) error {
			opts.IsTest = true
			return run(ctx, g, &opts, args)
		})
	},
}

// AddCommand adds the 'run' command to cmd.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	fs.StringSliceVarP(&opts.Range, "range", "r", nil, "set range `from-to`")
	fs.StringVar(&opts.RangeFormat, "range-format", "%d", "set `format` for range (when used with --range)")
	fs.StringVarP(&opts.Filename, "file", "f", "", "read values from `filename`")
	fs.StringArrayVar(&opts.Replace, "replace", []string{}, "add replace var `name:type:options` (valid types: 'file','range', and 'value', e.g. 'FUZZ:range:1-100'), mutually exclusive with --range and --file")

	fs.StringVar(&opts.Logfile, "logfile", "", "write copy of printed messages to `filename`.log")
	fs.StringVar(&opts.Logdir, "logdir", os.Getenv("MONSOON_LOG_DIR"), "automatically log all output to files in `dir`")

	fs.IntVarP(&opts.Threads, "threads", "t", 5, "make as many as `n` parallel requests")
	fs.IntVar(&opts.BufferSize, "buffer-size", 100000, "set number of buffered items to `n`")
	fs.IntVar(&opts.Skip, "skip", 0, "skip the first `n` requests")
	fs.IntVar(&opts.Limit, "limit", 0, "only run `n` requests, then exit")
	fs.Float64Var(&opts.RequestsPerSecond, "requests-per-second", 0, "do at most `n` requests per second (e.g. 0.5)")

	// add all options to define a request
	opts.Request = request.New(nil)
	request.AddFlags(opts.Request, fs)

	fs.IntVar(&opts.FollowRedirect, "follow-redirect", 0, "follow `n` redirects")

	fs.StringSliceVar(&opts.HideStatusCodes, "hide-status", nil, "hide responses with this status `code,[code-code],[-code],[...]`")
	fs.StringSliceVar(&opts.ShowStatusCodes, "show-status", nil, "show only responses with this status `code,[code-code],[code-],[...]`")
	fs.StringSliceVar(&opts.HideHeaderSize, "hide-header-size", nil, "hide responses with this header size (`size,from-to,from-,-to`)")
	fs.StringSliceVar(&opts.HideBodySize, "hide-body-size", nil, "hide responses with this body size (`size,from-to,from-,-to`)")
	fs.StringArrayVar(&opts.HidePattern, "hide-pattern", nil, "hide responses containing `regex` in response header or body (can be specified multiple times)")
	fs.StringArrayVar(&opts.ShowPattern, "show-pattern", nil, "show only responses containing `regex` in response header or body (can be specified multiple times)")

	fs.StringArrayVar(&opts.Extract, "extract", nil, "extract `regex` from response header or body (can be specified multiple times)")
	fs.StringArrayVar(&opts.ExtractPipe, "extract-pipe", nil, "pipe response body to `cmd` to extract data (can be specified multiple times)")
	fs.IntVar(&opts.MaxBodySize, "max-body-size", 5, "read at most `n` MiB from a returned response body (used for extracting data from the body)")
	fs.BoolVar(&opts.DisableDecompression, "disable-decompression", false, "disable automatic decompression of the response body")

	// add transport options
	response.AddTransportFlags(fs, &opts.TransportOptions)

	fs.DurationVar(&opts.LongRequest, "long-request", 5*time.Second, "show response duration for requests longer than `duration`")

	fs.BoolVar(&opts.IPv4Only, "ipv4-only", false, "only connect to target host via IPv4")
	fs.BoolVar(&opts.IPv6Only, "ipv6-only", false, "only connect to target host via IPv6")

	c.AddCommand(cmdTest)
	testFlags := cmdTest.Flags()
	testFlags.AddFlagSet(fs)
	testFlags.StringSliceVarP(&opts.Values, "value", "v", []string{}, "use `string` as the value (can be specified multiple times)")
	testFlags.BoolVar(&opts.ShowRequest, "show-request", false, "also print HTTP request")
}

// logfilePath returns the prefix for the logfiles, if any.
func logfilePath(opts *Options, inputURL string) (prefix string, err error) {
	if opts.Logdir != "" && opts.Logfile == "" {
		url, err := url.Parse(inputURL)
		if err != nil {
			return "", err
		}

		ts := time.Now().Format("20060102_150405")
		fn := fmt.Sprintf("monsoon_%s_%s", url.Host, ts)
		p := filepath.Join(opts.Logdir, fn)
		return p, nil
	}

	return opts.Logfile, nil
}

func setupProducer(ctx context.Context, opts *Options) (*producer.Multiplexer, error) {
	multiplexer := &producer.Multiplexer{}

	switch {
	// handle old user interface, only range
	case len(opts.Range) > 0:
		var ranges []producer.Range
		for _, r := range opts.Range {
			rng, err := producer.NewRange(r)
			if err != nil {
				return nil, err
			}

			ranges = append(ranges, rng)
		}

		src := producer.NewRanges(ranges, opts.RangeFormat)
		multiplexer.AddSource("FUZZ", src)

		return multiplexer, nil

	// handle old user interface, read from stdin
	case opts.Filename == "-":
		src := producer.NewFile(os.Stdin, false)
		multiplexer.AddSource("FUZZ", src)

		return multiplexer, nil

	// handle old user interface, only filename
	case opts.Filename != "":
		file, err := os.Open(opts.Filename)
		if err != nil {
			return nil, err
		}

		src := producer.NewFile(file, true)
		multiplexer.AddSource("FUZZ", src)

		return multiplexer, nil
	case len(opts.Values) > 0:
		inValues := make([]byte, 0)
		for _, v := range opts.Values {
			inValues = append(inValues, []byte(v)...)
			inValues = append(inValues, []byte("\n")...)
		}
		multiplexer.AddSource("FUZZ", producer.NewFile(bytes.NewReader(inValues), true))
		return multiplexer, nil
	}

	if len(opts.Replace) == 0 {
		return nil, errors.New("no source specified, nothing to do")
	}

	// handle new user interface with replace rules
	for _, s := range opts.Replace {
		r, err := ParseReplace(s)
		if err != nil {
			return nil, fmt.Errorf("invalid replace rule: %w", err)
		}

		switch r.Type {
		case "file":
			if r.Options == "-" {
				multiplexer.AddSource(r.Name, producer.NewFile(os.Stdin, false))
			} else {
				f, err := os.Open(r.Options)
				if err != nil {
					return nil, fmt.Errorf("file source: %w", err)
				}

				multiplexer.AddSource(r.Name, producer.NewFile(f, true))
			}

		case "range":
			rangeFormat := "%d"

			// check if there is a format specifier
			if strings.Contains(r.Options, ":") {
				data := strings.SplitN(r.Options, ":", 2)
				if len(data) != 2 {
					return nil, fmt.Errorf("wrong options format for range, want NAME:range:A-B[,C-D][:format]")
				}

				r.Options = data[0]
				rangeFormat = data[1]
			}

			var ranges []producer.Range
			for _, r := range strings.Split(r.Options, ",") {
				rng, err := producer.NewRange(r)
				if err != nil {
					return nil, err
				}

				ranges = append(ranges, rng)
			}

			src := producer.NewRanges(ranges, rangeFormat)
			multiplexer.AddSource(r.Name, src)
		case "value":
			multiplexer.AddSource(r.Name, producer.NewValue(r.Options))
		default:
			return nil, fmt.Errorf("unknown replace type %q", r.Type)
		}
	}

	return multiplexer, nil
}

func setupTerminal(ctx context.Context, g *errgroup.Group, maxFrameRate uint, logfilePrefix string) (term cli.Terminal, cleanup func(), err error) {
	ctx, cancel := context.WithCancel(context.Background())

	statusTerm := termstatus.New(os.Stdout, os.Stderr, false)
	if maxFrameRate != 0 {
		statusTerm.MaxFrameRate = maxFrameRate
	}

	term = statusTerm

	if logfilePrefix != "" {
		fmt.Printf(reporter.Bold("Logfile:")+" %s.log\n", logfilePrefix)

		logfile, err := os.Create(logfilePrefix + ".log")
		if err != nil {
			return nil, cancel, err
		}

		fmt.Fprintln(logfile, shell.Join(os.Args))

		// write copies of messages to logfile
		term = &cli.LogTerminal{
			Terminal: statusTerm,
			Writer:   logfile,
		}
	}

	// make sure error messages logged via the log package are printed nicely
	w := cli.NewStdioWrapper(term)
	log.SetOutput(w.Stderr())

	g.Go(func() error {
		term.Run(ctx)
		return nil
	})

	return term, cancel, nil
}

func setupResponseFilters(opts *Options) ([]response.Filter, error) {
	var filters []response.Filter

	filter, err := response.NewFilterStatusCode(opts.HideStatusCodes, opts.ShowStatusCodes)
	if err != nil {
		return nil, err
	}

	filters = append(filters, filter)

	if len(opts.HideHeaderSize) > 0 || len(opts.HideBodySize) > 0 {
		f, err := response.NewFilterSize(opts.HideHeaderSize, opts.HideBodySize)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}

	if len(opts.hidePattern) > 0 {
		filters = append(filters, response.FilterRejectPattern{Pattern: opts.hidePattern})
	}

	if len(opts.showPattern) > 0 {
		filters = append(filters, response.FilterAcceptPattern{Pattern: opts.showPattern})
	}

	return filters, nil
}

func setupValueFilters(ctx context.Context, opts *Options, cancelProducer func(), valueCh <-chan []string, countCh <-chan int) (<-chan []string, <-chan int) {
	if opts.Skip > 0 {
		f := &producer.FilterSkip{Skip: opts.Skip}
		countCh = f.Count(ctx, countCh)
		valueCh = f.Select(ctx, valueCh)
	}

	if opts.Limit > 0 {
		f := &producer.FilterLimit{Max: opts.Limit, CancelProducer: cancelProducer}
		countCh = f.Count(ctx, countCh)
		valueCh = f.Select(ctx, valueCh)
	}

	return valueCh, countCh
}

func startRunners(ctx context.Context, opts *Options, in <-chan []string) (<-chan response.Response, error) {
	out := make(chan response.Response)

	var wg sync.WaitGroup

	transport, err := response.NewTransport(opts.TransportOptions, opts.Threads)
	if err != nil {
		return nil, err
	}

	for i := 0; i < opts.Threads; i++ {
		runner := response.NewRunner(transport, opts.Request, in, out)
		runner.MaxBodySize = opts.MaxBodySize * 1024 * 1024
		runner.Extract = opts.extract
		runner.DecompressResponseBody = !opts.DisableDecompression
		runner.PreserveRequestBody = opts.IsTest

		runner.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) <= opts.FollowRedirect {
				return nil
			}
			return http.ErrUseLastResponse
		}
		wg.Add(1)
		go func() {
			runner.Run(ctx)
			wg.Done()
		}()
	}

	go func() {
		// wait until the runners are done, then close the output channel
		wg.Wait()
		close(out)
	}()

	return out, nil
}

func run(ctx context.Context, g *errgroup.Group, opts *Options, args []string) error {
	// make sure the options and arguments are valid
	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	err := opts.valid()
	if err != nil {
		return err
	}

	inputURL := args[0]
	opts.Request.URL = inputURL

	// setup logging and the terminal
	logfilePrefix, err := logfilePath(opts, inputURL)
	if err != nil {
		return err
	}

	var maxFrameRate uint
	if s, ok := os.LookupEnv("MONSOON_PROGRESS_FPS"); ok {
		rate, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return fmt.Errorf("parse $MONSOON_PROGRESS_FPS: %w", err)
		}
		maxFrameRate = uint(rate)
	}

	term, cleanup, err := setupTerminal(ctx, g, maxFrameRate, logfilePrefix)
	defer cleanup()
	if err != nil {
		return err
	}

	// collect the filters for the responses
	responseFilters, err := setupResponseFilters(opts)
	if err != nil {
		return err
	}

	// setup the pipeline for the values
	vch := make(chan []string, opts.BufferSize)
	var valueCh <-chan []string = vch
	cch := make(chan int, 1)
	var countCh <-chan int = cch

	// start produces and initialize multiplexer
	producerCtx, producerCancel := context.WithCancel(ctx)
	defer producerCancel()

	multiplexer, err := setupProducer(producerCtx, opts)
	if err != nil {
		return err
	}

	opts.Request.Names = multiplexer.Names

	// run Multiplexer
	g.Go(func() error {
		return multiplexer.Run(producerCtx, vch, cch)
	})

	// filter values (skip, limit)
	valueCh, countCh = setupValueFilters(ctx, opts, producerCancel, valueCh, countCh)

	// limit the throughput (if requested)
	if opts.RequestsPerSecond > 0 {
		valueCh = producer.Limit(ctx, opts.RequestsPerSecond, valueCh)
	}

	// start the runners
	responseCh, err := startRunners(ctx, opts, valueCh)
	if err != nil {
		return err
	}

	// filter the responses
	responseCh = response.Mark(responseCh, responseFilters)

	// extract data from all interesting (non-hidden) responses
	extracter := &response.Extracter{
		Pattern:  opts.extract,
		Commands: opts.extractPipe,
	}
	responseCh = extracter.Run(responseCh)

	if logfilePrefix != "" {
		rec, err := recorder.New(logfilePrefix+".json", opts.Request)
		if err != nil {
			return err
		}

		// fill in information for generating the request
		rec.Data.InputFile = opts.Filename
		rec.Data.Ranges = opts.Range
		rec.Data.RangeFormat = opts.RangeFormat
		rec.Data.Extract = opts.Extract
		rec.Data.ExtractPipe = opts.ExtractPipe

		out := make(chan response.Response)
		in := responseCh
		responseCh = out

		outCount := make(chan int)
		inCount := countCh
		countCh = outCount

		g.Go(func() error {
			return rec.Run(ctx, in, out, inCount, outCount)
		})
	}

	targetURL, err := opts.Request.TargetURL()
	if err != nil {
		return err
	}

	// run the reporter
	term.Printf(reporter.Bold("Target URL:")+" %v\n\n", targetURL)
	reporter := reporter.New(term, opts.LongRequest, opts.IsTest)
	err = reporter.Display(responseCh, countCh)

	return err
}
