package show

import (
	"bytes"
	"errors"
	"fmt"
	"net/http/httputil"
	"os"

	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/spf13/cobra"
)

// Options collect options for the command.
type Options struct {
	Request *request.Request // the template for the HTTP request
	Values  []string
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	opts.Request = request.New(nil)
	request.AddFlags(opts.Request, fs)

	fs.StringSliceVarP(&opts.Values, "value", "v", []string{}, "use `string` as the value (can be specified multiple times)")
}

var cmd = &cobra.Command{
	Use:                   "show [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("last argument needs to be the URL")
		}

		if len(args) > 1 {
			return errors.New("more than one target URL specified")
		}

		opts.Request.URL = args[0]
		opts.Request.Names = []string{"FUZZ"}

		if len(opts.Values) == 0 {
			return errors.New("no value specified, use --value")
		}

		req, err := opts.Request.Apply(opts.Values)
		if err != nil {
			return err
		}

		host, port, err := request.Target(req)
		if err != nil {
			return err
		}

		// remote server
		fmt.Printf("remote %v, port %v\n\n", host, port)

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
