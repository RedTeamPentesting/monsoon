package list

import (
	"os"

	"github.com/spf13/cobra"
)

// ListOptions collect options for the command.
type ListOptions struct {
	Logdir string

	Host string
	Port string

	ShowIncomplete bool
	ShowLogfile    bool
	ShowResponses  bool
}

var opts ListOptions

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmdList)

	fs := cmdList.PersistentFlags()
	fs.SortFlags = false

	fs.StringVar(&opts.Logdir, "logdir", os.Getenv("MONSOON_LOG_DIR"), "automatically log all output to files in `dir`")

	fs.StringVar(&opts.Host, "host", "", "only display runs for hosts containing the string `str`")
	fs.StringVar(&opts.Port, "port", "", "only display runs for `port`")
	fs.BoolVar(&opts.ShowIncomplete, "incomplete", false, "show incomplete runs")
	fs.BoolVar(&opts.ShowLogfile, "logfile", false, "show log file name")
	fs.BoolVar(&opts.ShowResponses, "responses", false, "show responses")
}
