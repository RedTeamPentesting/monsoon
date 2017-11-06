package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// GlobalOptions contains global options.
type GlobalOptions struct {
	Range string
	File  string
}

var globalOptions GlobalOptions

func init() {
	fs := cmdRoot.Flags()
	fs.StringVarP(&globalOptions.Range, "range", "r", "", "set range `from-to`")
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

func run(opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	url := args[0]
	fmt.Printf("fuzzin %v", url)
	return nil
}
