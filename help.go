package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setupHelp sets the templates used by cobra to format the help messages.
func setupHelp(cmd *cobra.Command) {
	cobra.AddTemplateFunc("wrapFlags", wrapFlags)
	cmd.SetUsageTemplate(usageTemplateGlobal)
	cmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
		Simply type monsoon help [command] for full details.`,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := c.Root().Find(args)
			if cmd == nil || e != nil {
				c.Printf("Unknown help topic %q\n", args)
				_ = c.Root().Usage()
				return
			}

			cmd.InitDefaultHelpFlag() // make possible 'help' flag to be shown
			cmd.SetUsageTemplate(usageTemplateHelpCommand)
			_ = cmd.Help()
		},
	})
}

// wrapFlags returns a help text for all flags wrapped at the terminal size.
func wrapFlags(f *pflag.FlagSet) string {
	width := getTermWidth(int(os.Stdin.Fd()))
	return f.FlagUsagesWrapped(width)
}

const usageTemplateGlobal = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} command [options]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Options:
{{ wrapFlags .LocalFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Options:
{{ wrapFlags .InheritedFlags | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}{{if .HasExample}}

Use "{{.Parent.Name}} help {{.Name}}" for examples.{{end}}
`

const usageTemplateHelpCommand = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Options:
{{ wrapFlags .LocalFlags | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Options:
{{ wrapFlags .InheritedFlags | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
