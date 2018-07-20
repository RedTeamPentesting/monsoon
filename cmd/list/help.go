package list

import (
	"strings"

	"github.com/happal/monsoon/request"
)

const helpShort = "List and filter previous runs of 'fuzz'"

var helpLong = strings.TrimSpace(`
The 'list' command displays previous runs of the 'fuzz' command for which it
can detect log files in the log directory. It also allows fitering, e.g. by
host, port, or path.
` + request.LongHelp)

const helpExamples = ``
