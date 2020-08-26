package list

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/RedTeamPentesting/monsoon/recorder"
	"github.com/spf13/cobra"
)

func init() {
	cmdList.AddCommand(cmdListTree)
}

func printTree(target string, runs []recorder.Run) error {
	url, err := url.Parse(target)
	if err != nil {
		return err
	}
	fmt.Printf("\n%v\n====================================\n", url)

	structure := make(map[string]string)
	for _, run := range runs {
		// we're only interested in the runs exactly matching the host and port
		if run.Host != url.Hostname() {
			continue
		}

		if run.Port != url.Port() {
			continue
		}

		if run.URL.Scheme != url.Scheme {
			continue
		}

		// only consider the runs for which the path was tested
		if !strings.Contains(run.URL.Path, "FUZZ") {
			continue
		}

		for _, resp := range run.Responses {
			// ignore errors
			if resp.Error != "" {
				continue
			}

			// rebuild the request URL
			responseURL, err := url.Parse(strings.Replace(run.URL.String(), "FUZZ", resp.Item, -1))
			if err != nil {
				// ignore errors for now
				continue
			}

			switch resp.StatusCode {
			case 200:
				structure[responseURL.Path] = "file"
			case 301:
				structure[responseURL.Path] = "dir"
			case 401, 403:
				structure[responseURL.Path] = "forbidden"
			case 400, 404:
				// ignore
			default:
				structure[responseURL.Path] = fmt.Sprintf("unknown (%v)", resp.StatusCode)
			}
		}
	}

	var names []string
	for path := range structure {
		names = append(names, path)
	}
	sort.Strings(names)

	var current string
	for _, name := range names {
		if path.Dir(name) != current {
			current = path.Dir(name)
			fmt.Printf("%v\n", current)
		}
		state := structure[name]
		fmt.Printf("     %v %v\n", path.Base(name), state)
	}

	return nil
}

var cmdListTree = &cobra.Command{
	Use: "tree [options]",

	DisableFlagsInUseLine: true,

	Short: "Display discovered directory/file structure",
	Long: strings.TrimSpace(`
The 'list tree' command prints the directory/file structure discovered by using
'fuzz' on targets.
`),

	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := recorder.LoadRuns(opts.Logdir)
		if err != nil {
			return err
		}

		list = filterRuns(list, opts)
		targets := extractTargets(list)

		for _, target := range targets {
			err := printTree(target, list)
			if err != nil {
				return err
			}
		}

		return nil
	},
}
