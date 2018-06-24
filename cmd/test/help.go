package test

import (
	"strings"

	"github.com/happal/monsoon/request"
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
`
