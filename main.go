package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
}

var globalOptions GlobalOptions

func init() {
	fs := cmdRoot.Flags()

	fs.StringVarP(&globalOptions.Range, "range", "r", "", "set range `from-to`")
	fs.StringVar(&globalOptions.RangeFormat, "range-format", "%d", "set `format` for range")

	fs.StringVarP(&globalOptions.Filename, "file", "f", "", "read values from `filename`")

	fs.IntVarP(&globalOptions.Threads, "threads", "t", 5, "make as many as `n` parallel requests")
	fs.IntVar(&globalOptions.BufferSize, "buffer-size", 100000, "set number of buffered items to `n`")
	fs.SortFlags = false
}

var cmdRoot = &cobra.Command{
	Use:           "monsoon URL",
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

func run(opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
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
			return fmt.Errorf("wrong format for range: %v", err)
		}
		producer = rp
	case opts.Filename != "":
		producer = &FileProducer{Filename: opts.Filename}
	default:
		return errors.New("neither file nor range specified, nothing to do")
	}

	term.Printf("fuzzing %v\n", url)

	producerChannel := make(chan string, opts.BufferSize)
	countChannel := make(chan int, 1)

	prodTomb, _ := tomb.WithContext(ctx)
	producer.Start(prodTomb, producerChannel, countChannel)
	go func() {
		// wait until the producer is done, then close the output channel
		<-prodTomb.Dead()
		close(producerChannel)
	}()

	responseChannel := make(chan Response)

	runnerTomb, _ := tomb.WithContext(ctx)
	for i := 0; i < opts.Threads; i++ {
		runner := NewRunner(runnerTomb, url, producerChannel, responseChannel)
		runnerTomb.Go(runner.Run)
	}

	go func() {
		// wait until the runners are done, then close the output channel
		<-runnerTomb.Dead()
		close(responseChannel)
	}()

	filter := &SimpleFilter{
		Hide: map[int]bool{
			404: true,
		},
	}

	reporter := NewReporter(term, filter)
	displayTomb, _ := tomb.WithContext(ctx)
	displayTomb.Go(reporter.Display(responseChannel, countChannel))
	<-displayTomb.Dead()

	return term.Finish()
}
