[![Status badge for tests](https://github.com/happal/monsoon/workflows/Build%20and%20tests/badge.svg)](https://github.com/happal/monsoon/actions?query=workflow%3A%22Build+and+tests%22)

# monsoon

A fast HTTP enumerator that allows you to execute a large number of HTTP
requests, filter the responses and display them in real-time.

## Example

Run an HTTP GET request for each entry in `filenames.txt`, hide all responses with the status code `403` or `404`:

![basic demo](demos/demo1.gif)

Common usage of monsoon is also covered in our blog article
["Introducing monsoon - a lean and versatile HTTP enumerator"](https://blog.redteam-pentesting.de/2020/introducing-monsoon/).

# Installation

## Building from source

These instructions will get you a compiled version of the code in the master branch.

You'll need a recent version of the [Go compiler](https://golang.org/dl), at
least version 1.14. For Debian, install the package `golang-go`.

Clone the repository, then from within the checkout run the following command:

```
$ go build
```

Afterwards you'll find a `monsoon` binary in the current directory. It can be
for other operating systems as follows:

```
$ GOOS=windows GOARCH=amd64 go build -o monsoon.exe
```

## Unofficial Packages

For Arch Linux based distributions `monsoon` is available as an unofficial
package on the [AUR](https://aur.archlinux.org/packages/monsoon). Using your
AUR helper of choice such as [yay](https://github.com/Jguer/yay):

```bash
yay -S monsoon
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

# Wordlists

The [SecLists Project](https://github.com/danielmiessler/SecLists) collects
wordlists that can be used with `monsoon`.

