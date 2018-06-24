package request

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
