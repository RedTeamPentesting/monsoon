package show

import (
	"strings"

	"github.com/RedTeamPentesting/monsoon/request"
)

const helpShort = "Construct and display an HTTP request"

var helpLong = strings.TrimSpace(`
The 'show' command can be use to construct a request and inspect it. The
options are the same as for the 'fuzz' command, so they can directly applied to
it once the request works.
` + request.LongHelp)

const helpExamples = `
Construct a request from scratch with a custom header, using the string
'hunter2' as the value for FUZZ:

    monsoon show --method POST --data 'foo=bar' \
      --header "X-secret: FUZZ" \
      --value hunter2 \
      https://www.example.com

Load a request from the file 'request.txt', replacing the 'Accept' header:

    monsoon show --template-file 'request.txt' \
      --header 'Accept: */*' \
      https://www.example.com
`
