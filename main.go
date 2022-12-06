package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/RedTeamPentesting/monsoon/cmd/fuzz"
	"github.com/RedTeamPentesting/monsoon/cmd/list"
	"github.com/RedTeamPentesting/monsoon/cmd/show"
	"github.com/spf13/cobra"
)

var version = ""

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
		fmt.Println("monsoon " + buildVersionString(version))
	},
}

func init() {
	// configure cobra help texts
	setupHelp(cmdRoot)
	fuzz.AddCommand(cmdRoot)
	show.AddCommand(cmdRoot)
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

func buildVersionString(compileTimeVersion string) string {
	fallback := compileTimeVersion
	if fallback == "" {
		fallback = "unknown version"
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}

	buildSetting := func(key string) (string, bool) {
		for _, setting := range buildInfo.Settings {
			if setting.Key == key {
				return setting.Value, true
			}
		}

		return "", false
	}

	vcs, ok := buildSetting("vcs")
	if !ok {
		return fallback
	}

	commit, _ := buildSetting("vcs.revision")
	if !ok {
		return version
	}

	dirty, ok := buildSetting("vcs.modified")
	if ok && dirty == "true" && commit != "" {
		dirty = " (dirty)"
	}

	if compileTimeVersion != "" {
		versionString := compileTimeVersion
		if commit != "" {
			versionString += "-" + shortCommit(commit) + dirty
		}

		return versionString
	}

	return fmt.Sprintf("built from %s revision %s%s", vcs, commit, dirty)
}

func shortCommit(commit string) string {
	if len(commit) < 8 {
		return commit
	}

	return commit[:8]
}
