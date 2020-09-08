package test

import (
	"strings"

	"github.com/RedTeamPentesting/monsoon/request"
)

const helpShort = "Send an HTTP request to a server and show the result"

var helpLong = strings.TrimSpace(`
The 'test' command can be use to send a request to the server and display the
result. The options are the same as for the 'fuzz' command, so they can
directly applied to it once the request works.
` + request.LongHelp)

const helpExamples = `
Send a request with the string 'FUZZ' replaced by 'hunter2', send it to the
server at example.com and display the result:

    monsoon test --method POST --data 'foo=bar' \
      --header "X-secret: FUZZ" \
      --value hunter2 \
      https://www.example.com

A Proxy for HTTP and HTTPS requests can be configured separately via the environment
variables HTTP_PROXY and HTTPS_PROXY. Both HTTP and socks5 proxies are supported:

    HTTP_PROXY=socks5://user:pass@proxyhost:123 monsoon fuzz [...]

Request to the loopback device are excluded from this proxy configuration. However,
an unconditional socks5 server can be configured as follows:

    FORCE_SOCKS5_PROXY=user:pass@proxyhost:123 monsoon fuzz [...]
`
