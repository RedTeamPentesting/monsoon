package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

// GlobalOptions contains global options.
type GlobalOptions struct {
	Range       string
	RangeFormat string
	File        string
}

var globalOptions GlobalOptions

func init() {
	fs := cmdRoot.Flags()
	fs.StringVarP(&globalOptions.Range, "range", "r", "", "set range `from-to`")
	fs.StringVar(&globalOptions.RangeFormat, "range-format", "%d", "set `format` for range")
	fs.StringVarP(&globalOptions.File, "file", "f", "", "read values from `filename`")
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
		fmt.Fprintf(os.Stderr, "%#v\n", err)
		os.Exit(1)
	}
}

// Producer yields values for enumerating.
type Producer interface {
	Start(context.Context, chan<- string) error
}

func run(opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	url := args[0]
	fmt.Printf("fuzzing %v\n", url)

	producer := &RangeProducer{
		Format: opts.RangeFormat,
	}

	_, err := fmt.Sscanf(opts.Range, "%d-%d", &producer.First, &producer.Last)
	if err != nil {
		return fmt.Errorf("wrong format for range: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string)
	var producerWg sync.WaitGroup

	producer.Start(ctx, &producerWg, ch)

	go func() {
		producerWg.Wait()
		fmt.Printf("producer is done\n")
		close(ch)
	}()

	runner := NewRunner(url)
	runner.Run(ctx, ch)

	return nil
}
