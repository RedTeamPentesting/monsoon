package list

import (
	"fmt"
	"strings"

	"github.com/RedTeamPentesting/monsoon/recorder"
	"github.com/spf13/cobra"
)

func init() {
	cmdList.AddCommand(cmdListTargets)
}

func extractTargets(list []recorder.Run) []string {
	known := make(map[string]struct{})
	var targets []string

	for _, run := range list {
		target := fmt.Sprintf("%v://%v", run.URL.Scheme, run.Hostport)
		if _, ok := known[target]; ok {
			continue
		}

		known[target] = struct{}{}
		targets = append(targets, target)
	}

	return targets
}

var cmdListTargets = &cobra.Command{
	Use: "targets [options]",

	DisableFlagsInUseLine: true,

	Short: "Print all targets (scheme, host, port)",
	Long: strings.TrimSpace(`
The 'list targets' command prints a list of all targets used with 'fuzz'. A
target consists of the URL scheme (http or https), the host name or IP address,
and the port.
`),

	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := recorder.LoadRuns(opts.Logdir)
		if err != nil {
			return err
		}

		list = filterRuns(list, opts)
		hosts := extractTargets(list)

		for _, host := range hosts {
			fmt.Println(host)
		}

		return nil
	},
}
