package main

import (
	"fmt"
	"os"

	"github.com/RedTeamPentesting/monsoon/cmd/fuzz"
	"github.com/RedTeamPentesting/monsoon/cmd/list"
	"github.com/RedTeamPentesting/monsoon/cmd/show"
	"github.com/RedTeamPentesting/monsoon/cmd/test"
	"github.com/spf13/cobra"
)

var version = "built from source"

var cmdRoot = &cobra.Command{
	Use:           "monsoon COMMAND [options]",
	SilenceErrors: true,
	SilenceUsage:  true,
}

var cmdVersion = &cobra.Command{
	Use:   "version",
	Short: "Print the current version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("monsoon " + version)
	},
}

func init() {
	// configure cobra help texts
	setupHelp(cmdRoot)
	fuzz.AddCommand(cmdRoot)
	show.AddCommand(cmdRoot)
	test.AddCommand(cmdRoot)
	list.AddCommand(cmdRoot)
	cmdRoot.AddCommand(cmdVersion)
}

func injectDefaultCommand(args []string) []string {
	validCommands := make(map[string]struct{})
	for _, cmd := range cmdRoot.Commands() {
		validCommands[cmd.Name()] = struct{}{}
	}

	// check that there's a command in the arguments
	for _, arg := range args {
		if _, ok := validCommands[arg]; ok {
			// valid command found, nothing to do
			return args
		}

		if arg == "-h" || arg == "--help" || arg == "help" {
			// request for help found, do not inject default command
			return args
		}
	}

	// inject default command as first argument
	fmt.Fprintf(os.Stderr, "no command found, assuming 'monsoon fuzz'\n\n")
	args = append([]string{"fuzz"}, args...)
	return args
}

func main() {
	os.Args = append(os.Args[:1], injectDefaultCommand(os.Args[1:])...)
	cmdRoot.SetArgs(os.Args[1:])

	err := cmdRoot.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
