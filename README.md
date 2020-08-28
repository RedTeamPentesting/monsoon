[![Status badge for tests](https://github.com/happal/monsoon/workflows/Build%20and%20tests/badge.svg)](https://github.com/happal/monsoon/actions?query=workflow%3A%22Build+and+tests%22)
[![Status badge for style checkers](https://github.com/happal/monsoon/workflows/Style%20Checkers/badge.svg)](https://github.com/happal/monsoon/actions?query=workflow%3A%22Style+Checkers%22)

# monsoon

A fast HTTP enumerator that allows you to execute a large number of HTTP
requests, filter the responses and display them in real-time.

## Example

Run an HTTP GET request for each entry in `filenames.txt`, hide all responses with the status code 403 or 404:

![basic demo](demos/demo1.gif)

# Installing

These instructions will get you a compiled version of the code in the master branch.

You'll need a recent version of the [Go compiler](https://golang.org/dl), at
least version 1.11. For Debian, install the package `golang-go`.

Clone the repository, then from within the checkout run the following command:

```
$ go build
```

Afterwards you'll find a `monsoon` binary in the current directory. It can be
for other operating systems as follows:

```
$ GOOS=windows GOARCH=amd64 go build -o monsoon.exe
```

# Getting Help

The program has several subcommands, the most important one is `fuzz` which
contains the main functionality. You can display a list of commands as follows:

```
$ ./monsoon -h
Usage:
  monsoon command [options]

Available Commands:
  fuzz        Execute and filter HTTP requests
  help        Help about any command
  show        Construct and display an HTTP request
  test        Send an HTTP request to a server and show the result
  version     Display version information

Options:
  -h, --help   help for monsoon

Use "monsoon [command] --help" for more information about a command.
```

For each command, calling it with `--help` (e.g. `monsoon fuzz --help`) will
display a description of all the options, and calling `monsoon help fuzz`
also shows an extensive list of examples.
