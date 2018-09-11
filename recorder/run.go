package recorder

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	Logfile   string
	JSONFile  string
	URL       *url.URL
	Host      string
	Port      string
	Hostport  string
	PathQuery string
	Data
}

// LoadRuns parses all JSON files in dir and returns a list of runs.
func LoadRuns(dir string) (runs []Run, err error) {
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
		run.PathQuery = run.URL.Path
		if run.URL.RawQuery != "" {
			run.PathQuery += "?" + run.URL.RawQuery
		}

		runs = append(runs, run)
	}

	return runs, nil
}

// SortRuns sorts the list first by URL then start timestamp.
func SortRuns(list []Run) {
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

// HostPorts returns a list of host:port combinations, and the list of runs sorted by the hostport combination.
func HostPorts(list []Run) (hostports []string, runs map[string][]Run) {
	runs = make(map[string][]Run)
	for _, run := range list {
		hostport := run.Hostport
		if _, ok := runs[hostport]; !ok {
			hostports = append(hostports, hostport)
		}
		runs[hostport] = append(runs[hostport], run)
	}
	return hostports, runs
}
