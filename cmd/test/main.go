package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/RedTeamPentesting/monsoon/cli"
	"github.com/RedTeamPentesting/monsoon/request"
	"github.com/RedTeamPentesting/monsoon/response"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// Options collect options for the command.
type Options struct {
	Request     *request.Request // the template for the HTTP request
	Value       string
	ShowRequest bool
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	opts.Request = request.New("")
	request.AddFlags(opts.Request, fs)

	fs.StringVarP(&opts.Value, "value", "v", "test", "Use `string` for the placeholder")
	fs.BoolVar(&opts.ShowRequest, "show-request", false, "Also print HTTP request")
}

func header(name string) string {
	if len(name) == 0 {
		return strings.Repeat("-", 80)
	}

	if len(name) > 70 {
		return name
	}

	return fmt.Sprintf("---- %s %s", name, strings.Repeat("-", 80-5-len(name)))
}

var cmd = &cobra.Command{
	Use:                   "test [options] URL",
	DisableFlagsInUseLine: true,

	Short:   helpShort,
	Long:    helpLong,
	Example: helpExamples,

	RunE: func(cmd *cobra.Command, args []string) error {
		return cli.WithContext(func(ctx context.Context, g *errgroup.Group) error {
			return run(ctx, g, &opts, args)
		})
	},
}

func run(ctx context.Context, g *errgroup.Group, opts *Options, args []string) error {
	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	opts.Request.URL = args[0]

	req, err := opts.Request.Apply(opts.Value)
	if err != nil {
		return err
	}

	host, port, err := request.Target(req)
	if err != nil {
		return err
	}

	// remote server
	fmt.Printf("remote %v, port %v\n\n", host, port)

	if opts.ShowRequest {
		fmt.Println(header("request"))
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
		if err != nil {
			return err
		}
	}

	input := make(chan string, 1)
	input <- opts.Value
	close(input)

	output := make(chan response.Response, 1)

	tr, err := response.NewTransport(opts.Request.Insecure, opts.Request.TLSClientKeyCertFile,
		opts.Request.DisableHTTP2, 1)
	if err != nil {
		return err
	}

	runner := response.NewRunner(tr, opts.Request, input, output)
	runner.Run(ctx)
	close(output)

	res := <-output

	if opts.ShowRequest {
		// we only need the separator when request and response are both shown
		fmt.Println(header("response"))
	}

	if res.Error != nil {
		fmt.Printf("error: %v\n", res.Error)
		return nil
	}

	// print response
	_, err = os.Stdout.Write(res.RawHeader)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(res.RawBody)
	if err != nil {
		return err
	}

	// be nice to the CLI user and append a newline if there isn't one yet
	if !bytes.HasSuffix(res.RawBody, []byte("\n")) {
		fmt.Println()
	}

	return nil
}
