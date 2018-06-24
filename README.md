[![Build Status](https://travis-ci.org/happal/monsoon.svg?branch=master)](https://travis-ci.org/happal/monsoon)

# monsoon

A fast HTTP enumerator that allows you to execute a large number of HTTP
requests, filter the responses and display them in real-time.

These instructions will get you a compiled version of the code in the master branch.

## Prerequisites

You'll need a recent version of the [Go compiler](https://golang.org/dl). For
Debian, install the package `golang-go`

## Installing

Clone the repository, then from within the checkout run the following command:

```
$ go run build.go
```

Afterwards you'll find a `monsoon` binary in the current directory. You can test it by running `./monsoon version`, which will print the version:

```
$ ./monsoon version
monsoon v0.1.0-3-g4a39f0e
compiled with go1.9.2 on linux
```

## Getting Help

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

## Example

Run an HTTP GET request for each entry in `filenames.txt`, hide all responses with the status code 403 or 404:

![basic demo](demos/demo1.gif)
