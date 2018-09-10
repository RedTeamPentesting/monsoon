package list

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/happal/monsoon/recorder"
	"github.com/spf13/cobra"
)

// Options collect options for the command.
type Options struct {
	Logdir string

	Host string
	Port string
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	fs.StringVar(&opts.Logdir, "logdir", os.Getenv("MONSOON_LOG_DIR"), "automatically log all output to files in `dir`")

	fs.StringVar(&opts.Host, "host", "", "only display runs for `host`")
	fs.StringVar(&opts.Port, "port", "", "only display runs for `port`")
}

func findJSONFiles(dir string) (files []string, err error) {
	err = filepath.Walk(dir, func(name string, fi os.FileInfo, err error) error {
		if err != nil {
			// try to continue despite error
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return nil
		}

		if fi == nil {
			return nil
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		if filepath.Ext(name) == ".json" {
			files = append(files, name)
		}

		return nil
	})

	return files, err
}

// Run describes one run of the 'fuzz' command.
type Run struct {
	Logfile  string
	JSONFile string
	URL      *url.URL
	Host     string
	Port     string
	Hostport string
	recorder.Data
}

func readJSONFiles(dir string) (runs []Run, err error) {
	files, err := findJSONFiles(dir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		buf, err := ioutil.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to read file, skipping: %v\n", file)
			continue
		}

		run := Run{
			JSONFile: file,
			Logfile:  strings.TrimSuffix(file, filepath.Ext(file)) + ".log",
		}
		err = json.Unmarshal(buf, &run.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to read JSON data from file %v, skipping: %v\n", file, err)
			continue
		}

		run.URL, err = url.Parse(run.Data.Template.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to parse template URL %v, skipping: %v\n", run.Data.Template.URL, err)
			continue
		}

		port := run.URL.Port()
		if port == "" {
			switch run.URL.Scheme {
			case "http":
				port = "80"
			case "https":
				port = "443"
			}
		}

		run.Host = run.URL.Hostname()
		run.Port = port
		run.Hostport = run.URL.Hostname() + ":" + port

		runs = append(runs, run)
	}

	return runs, nil
}

// sortRuns sorts the list first by URL then start timestamp.
func sortRuns(list []Run) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		switch {
		case a.Hostport < b.Hostport:
			return true
		case a.Hostport > b.Hostport:
			return false
		default:
			return a.URL.String() < b.URL.String()
		}
	})
}

func filterRuns(list []Run, opts Options) (res []Run) {
	for _, run := range list {
		if opts.Host != "" && opts.Host != run.Host {
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
		if opts.Logdir == "" {
			return errors.New("no log directory specified")
		}

		runs, err := readJSONFiles(opts.Logdir)
		if err != nil {
			return err
		}

		sortRuns(runs)
		runs = filterRuns(runs, opts)

		var hostport string
		first := true
		for _, run := range runs {
			if hostport != run.Hostport {
				if !first {
					fmt.Printf("-------------------------------------\n")
				}
				first = false
				fmt.Printf("%v, port %v:\n\n", run.Host, run.Port)
				hostport = run.Hostport
			}

			fmt.Printf("  %v\n", run.Logfile)
			fmt.Printf("  %s", run.URL)
			if run.Data.Cancelled {
				var complete float64
				if run.Data.TotalRequests > 0 {
					complete = float64(run.Data.SentRequests) / float64(run.Data.TotalRequests)
					fmt.Printf(" (incomplete, %.0f%%)", complete*100)
				} else {
					fmt.Printf(" (incomplete)")
				}
			}
			fmt.Printf("\n\n")
		}

		return nil
	},
}
