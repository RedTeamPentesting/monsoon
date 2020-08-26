package list

import (
	"fmt"
	"strings"

	"github.com/RedTeamPentesting/monsoon/recorder"
	"github.com/spf13/cobra"
)

func init() {
	cmdList.AddCommand(cmdListHosts)
}

func extractHosts(list []recorder.Run) []string {
	known := make(map[string]struct{})
	var hosts []string

	for _, run := range list {
		host := run.URL.Host
		if _, ok := known[host]; ok {
			continue
		}

		known[host] = struct{}{}
		hosts = append(hosts, host)
	}

	return hosts
}

var cmdListHosts = &cobra.Command{
	Use: "hosts [options]",

	DisableFlagsInUseLine: true,

	Short: "Print all host names",
	Long: strings.TrimSpace(`
The 'list hosts' command prints a list of all host names found in the all runs
of 'fuzz'.
`),

	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := recorder.LoadRuns(opts.Logdir)
		if err != nil {
			return err
		}

		list = filterRuns(list, opts)
		hosts := extractHosts(list)

		for _, host := range hosts {
			fmt.Println(host)
		}

		return nil
	},
}
