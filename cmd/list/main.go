package list

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/happal/monsoon/recorder"
	"github.com/spf13/cobra"
)

// Options collect options for the command.
type Options struct {
	Logdir string

	Host string
	Port string

	ShowIncomplete bool
	ShowLogfile    bool
	ShowResponses  bool
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	fs.StringVar(&opts.Logdir, "logdir", os.Getenv("MONSOON_LOG_DIR"), "automatically log all output to files in `dir`")

	fs.StringVar(&opts.Host, "host", "", "only display runs for hosts containing the string `str`")
	fs.StringVar(&opts.Port, "port", "", "only display runs for `port`")
	fs.BoolVar(&opts.ShowIncomplete, "incomplete", false, "show incomplete runs")
	fs.BoolVar(&opts.ShowLogfile, "logfile", false, "show log file name")
	fs.BoolVar(&opts.ShowResponses, "responses", false, "show responses")
}

func filterRuns(list []recorder.Run, opts Options) (res []recorder.Run) {
	for _, run := range list {
		if run.Data.Cancelled && !opts.ShowIncomplete {
			continue
		}

		if !strings.Contains(run.Host, opts.Host) {
			continue
		}
		if opts.Port != "" && opts.Port != run.Port {
			continue
		}

		res = append(res, run)
	}

	return res
}

var cmd = &cobra.Command{
	Use:                   "list [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(opts)
	},
}

const HostTemplate = `{{ .Hostport }}
{{- $opt := .Options }}
{{ range .Runs }}
  {{ .PathQuery }}
    Time:      {{ .Start.Format "2006-01-02 15:04:05" }}
{{- if $opt.ShowLogfile }}
    Log:       {{ .Logfile -}}
{{ end }}
    Duration:  {{ duration .Start .End }}
    Requests:  {{ .SentRequests }}
    Responses: {{ len .Responses }}
{{- if ne .InputFile "" }}
    Inputfile: {{ .InputFile }}
{{ end -}}
{{- if ne .Range "" }}
    Range:     {{ .Range }}
{{ end -}}
{{- if ne .Template.Method "GET" }}
    Method:    {{ .Template.Method -}}
{{ end -}}
{{- if ne .Template.Body "" }}
    Body:      {{ .Template.Body -}}
{{ end }}
{{- if $opt.ShowResponses -}}
{{ range .Responses }}
      {{ .StatusCode }} {{ .Item }}
{{- end }}
{{- end }}
{{ end }}
`

var FuncMap = map[string]interface{}{
	"contains": strings.Contains,
	"duration": func(t1, t2 time.Time) (s string) {
		sec := uint64(t2.Sub(t1).Seconds())
		if sec > 3600 {
			s += fmt.Sprintf("%dh", sec/3600)
			sec = sec % 3600
		}

		if sec > 60 {
			s += fmt.Sprintf("%dm", sec/60)
			sec = sec % 60
		}
		s += fmt.Sprintf("%ds", sec)
		return s
	},
}

type Host struct {
	Options
	Hostport string
	Runs     []recorder.Run
}

func runList(opts Options) error {
	if opts.Logdir == "" {
		return errors.New("no log directory specified")
	}

	list, err := recorder.LoadRuns(opts.Logdir)
	if err != nil {
		return err
	}

	tmpl, err := template.New("").Funcs(FuncMap).Parse(HostTemplate)
	if err != nil {
		return err
	}

	recorder.SortRuns(list)
	list = filterRuns(list, opts)

	hostports, runs := recorder.HostPorts(list)
	for _, hostport := range hostports {
		err := tmpl.Execute(os.Stdout, Host{
			Options:  opts,
			Hostport: hostport,
			Runs:     runs[hostport],
		})
		if err != nil {
			return err
		}
	}

	return nil
}
