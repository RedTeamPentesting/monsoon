package request

import "github.com/spf13/pflag"

// LongHelp is a text which describes how constructing a request works. It is
// typically used in the long help text.
const LongHelp = `
The requests can either be constructed from scratch, or loaded from a template
file and modified with flags. The flags have priority and replace values loaded
from the file, this includes the HTTP headers, method and body.

When a template file is used, the URL passed as an argument to the command must
not have a path or query string set. It is just used to set the target host
name, port and protocol.
`

// AddFlags adds flags for all options of a request to fs.
func AddFlags(r *Request, fs *pflag.FlagSet) {
	// basics
	fs.StringVar(&r.Method, "request", "", "use HTTP request `method`")
	_ = fs.MarkDeprecated("request", "use --method")
	fs.StringVarP(&r.Method, "method", "X", "", "use HTTP request `method`")
	fs.VarP(r.Header, "header", "H", "add `\"name: value\"` as an HTTP request header, delete the header if only \"name\" is passed")
	fs.StringVarP(&r.Body, "data", "d", "", "transmit `data` in the HTTP request body")
	fs.StringVarP(&r.UserPass, "user", "u", "", "use `user:password` for HTTP basic auth")

	fs.StringVar(&r.TemplateFile, "template-file", "", "read HTTP request from `file`")

	// configure request
	fs.BoolVar(&r.ForceChunkedEncoding, "force-chunked-encoding", false, `do not set the Content-Length HTTP header and use chunked encoding`)

	// Transport
	fs.BoolVarP(&r.Insecure, "insecure", "k", false, "disable TLS certificate verification")
	fs.StringVar(&r.TLSClientKeyCertFile, "client-cert", "", "read TLS client key and cert from `file`")
	fs.BoolVar(&r.DisableHTTP2, "disable-http2", false, "do not try to negotiate an HTTP2 connection")
}
