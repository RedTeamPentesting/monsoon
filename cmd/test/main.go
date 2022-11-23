package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
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
	Request        *request.Request // the template for the HTTP request
	FollowRedirect int

	Values      []string
	ShowRequest bool

	response.TransportOptions

	IPv4Only bool
	IPv6Only bool
}

var opts Options

// AddCommand adds the command to c.
func AddCommand(c *cobra.Command) {
	c.AddCommand(cmd)

	fs := cmd.Flags()
	fs.SortFlags = false

	opts.Request = request.New(nil)
	request.AddFlags(opts.Request, fs)

	fs.IntVar(&opts.FollowRedirect, "follow-redirect", 0, "follow `n` redirects")

	fs.StringSliceVarP(&opts.Values, "value", "v", []string{}, "use `string` as the value (can be specified multiple times)")
	fs.BoolVar(&opts.ShowRequest, "show-request", false, "also print HTTP request")

	// add transport options
	response.AddTransportFlags(fs, &opts.TransportOptions)

	fs.BoolVar(&opts.IPv4Only, "ipv4-only", false, "only connect to target host via IPv4")
	fs.BoolVar(&opts.IPv6Only, "ipv6-only", false, "only connect to target host via IPv6")
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
	switch {
	case opts.IPv4Only && opts.IPv6Only:
		return fmt.Errorf("--ipv4-only and --ipv6-only cannot be used together")
	case opts.IPv4Only:
		opts.TransportOptions.Network = "tcp4"
	case opts.IPv6Only:
		opts.TransportOptions.Network = "tcp6"
	}

	if len(args) == 0 {
		return errors.New("last argument needs to be the URL")
	}

	if len(args) > 1 {
		return errors.New("more than one target URL specified")
	}

	err := opts.TransportOptions.Valid()
	if err != nil {
		return err
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

	input := make(chan []string, 1)
	input <- opts.Values
	close(input)

	output := make(chan response.Response, 1)

	tr, err := response.NewTransport(opts.TransportOptions, 1)
	if err != nil {
		return err
	}

	runner := response.NewRunner(tr, opts.Request, input, output)
	runner.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) <= opts.FollowRedirect {
			return nil
		}
		return http.ErrUseLastResponse
	}

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
