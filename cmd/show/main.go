package show

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httputil"
	"os"

	"github.com/happal/monsoon/request"
	"github.com/spf13/cobra"
)

// Options collect options for the command.
type Options struct {
	Request *request.Request // the template for the HTTP request
	Value   string
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	opts.Request = request.New()
	opts.Request.AddFlags(fs)

	fs.StringVarP(&opts.Value, "value", "v", "FUZZ", "Use `string` instead for the placeholder")
}

var cmd = &cobra.Command{
	Use:   "show [options] URL",
	Short: "Construct and display an HTTP request",
	// Example: longHelpText,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("last argument needs to be the URL")
		}

		if len(args) > 1 {
			return errors.New("more than one target URL specified")
		}

		opts.Request.URL = args[0]

		req, err := opts.Request.Apply("FUZZ", opts.Value)
		if err != nil {
			return err
		}

		port := req.URL.Port()
		if port == "" {
			// fill in default ports
			switch req.URL.Scheme {
			case "http":
				port = "80"
			case "https":
				port = "443"
			default:
				return fmt.Errorf("unknown URL scheme %q", req.URL.Scheme)
			}
		}

		// remote server
		fmt.Printf("target server: %v:%v\n\n", req.URL.Hostname(), port)

		// print request with body
		buf, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			return err
		}

		// be nice to the CLI user and append a newline if there isn't one yet
		if !bytes.HasSuffix(buf, []byte("\n")) {
			buf = append(buf, '\n')
		}
		_, err = os.Stdout.Write(buf)
		return err
	},
}
