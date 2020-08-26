package list

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/RedTeamPentesting/monsoon/recorder"
	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/spf13/cobra"
)

var cmdList = &cobra.Command{
	Use:                   "list [options]",
	DisableFlagsInUseLine: true,

	Short: "List and filter previous runs of 'fuzz'",
	Long: strings.TrimSpace(`
The 'list' command displays previous runs of the 'fuzz' command for which it
can detect log files in the log directory. It also allows fitering, e.g. by
host, port, or path.
` + request.LongHelp),

	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(opts)
	},
}

func filterRuns(list []recorder.Run, opts ListOptions) (res []recorder.Run) {
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

const HostTemplate = `{{ .Hostport }}
{{- $opt := .ListOptions }}
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
{{- if .Ranges }}
    Range:     {{ join .Ranges "," }}
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
	"join": strings.Join,
}

type Host struct {
	ListOptions
	Hostport string
	Runs     []recorder.Run
}

func runList(opts ListOptions) error {
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
			ListOptions: opts,
			Hostport:    hostport,
			Runs:        runs[hostport],
		})
		if err != nil {
			return err
		}
	}

	return nil
}
