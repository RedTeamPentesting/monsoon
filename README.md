<p align="center">
  <h1 align="center"><b>monsoon</b></h1>
  <p align="center"><i>A fast HTTP enumerator that allows you to execute a large number of HTTP requests, filter the responses and display them in real-time</i></p>
  <p align="center">
    <a href="https://github.com/RedTeamPentesting/monsoon/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/RedTeamPentesting/monsoon.svg?style=for-the-badge"></a>
    <a href="https://github.com/RedTeamPentesting/monsoon/actions?workflow=Build+and+tests"><img alt="GitHub Action: Check" src="https://img.shields.io/github/actions/workflow/status/RedTeamPentesting/monsoon/tests.yml?branch=main&style=for-the-badge"></a>
    <a href="/LICENSE"><img alt="Software License" src="https://img.shields.io/badge/license-MIT-brightgreen.svg?style=for-the-badge"></a>
    <a href="https://goreportcard.com/report/github.com/RedTeamPentesting/monsoon"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/RedTeamPentesting/monsoon?style=for-the-badge"></a>
  </p>
</p>

`monsoon` is a fast and flexible HTTP fuzzer that can be used for a wide variety
of actions ranging from content discovery to credential bruteforcing. You can
read about the various use cases in our blog posts ["Introducing monsoon - a
lean and versatile HTTP
enumerator"](https://blog.redteam-pentesting.de/2020/introducing-monsoon/) and
["Bringing Monsoon to the Next
Level"](https://blog.redteam-pentesting.de/2023/monsoon-next-level/).

In the following example, an HTTP GET request is sent for each entry in
`filenames.txt`, ignoring all responses with the status code `404`:


![basic demo](demos/demo1.gif)

## Installation

As `monsoon` is a single statically linked binary, you can simply download a
pre-build binary for your operating system from the
[releases page](https://github.com/RedTeamPentesting/monsoon/releases).

### Building from source

These instructions will get you a compiled version of the code in the main
branch. First, you'll need a recent version of the
[Go compiler](https://golang.org/dl), at least version 1.18. If your compiler is
set up, clone the `monsoon` repository and run the following command from within
the checkout:

```
$ go build
```

Afterwards you'll find a `monsoon` binary in the current directory. It can be
for other operating systems such as Windows as follows:

```
$ GOOS=windows GOARCH=amd64 go build -o monsoon.exe
```

### Unofficial Packages

**Please note that unofficial packages are not maintained by RedTeam Pentesting**

For Arch Linux based distributions `monsoon` is available as an unofficial
package on the [AUR](https://aur.archlinux.org/packages/monsoon). Using your
AUR helper of choice such as [yay](https://github.com/Jguer/yay):

```bash
yay -S monsoon
```

## Documentation

The program has several subcommands, the most important one is `fuzz` which
contains the main functionality. You can display a list of commands as follows:

```
$ ./monsoon -h
Usage:
  monsoon command [options]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  fuzz        Execute and filter HTTP requests
  help        Help about any command
  list        List and filter previous runs of 'fuzz'
  show        Construct and display an HTTP request
  test        Execute and filter HTTP requests
  version     Print the current version

Options:
  -h, --help   help for monsoon

Use "monsoon [command] --help" for more information about a command.
```

For each command, calling it with `--help` (e.g. `monsoon fuzz --help`) will
display a description of all the options, and calling `monsoon help fuzz`
also shows an extensive list of examples.

## Wordlists

The [SecLists Project](https://github.com/danielmiessler/SecLists) collects
wordlists that can be used with `monsoon`.
