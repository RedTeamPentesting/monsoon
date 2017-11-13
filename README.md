[![Build Status](https://travis-ci.org/fd0/monsoon.svg?branch=master)](https://travis-ci.org/fd0/monsoon)

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
go run build.go
```

Afterwards you'll find a `monsoon` binary in the current directory. You can test it by running `./monsoon --version`, which will print the version:

```
./monsoon --version
monsoon v0.1.0-3-g4a39f0e
compiled with go1.9.2 on linux
```

## Example

Run an HTTP GET request for each entry in `raft-large-files.txt`, hide all responses with the status code 404:

```
./monsoon -f raft-large-files.txt --hide-status 403,404 --insecure https://invalid.example.com/FUZZ

fuzzing https://wolke.gnuzifer.de/FUZZ
 status   header     body   value    extract
    302     1122        0   index.php, Location: https://wolke.gnuzifer.de/index.php/login
    200     1073       20   cron.php
    200      565      179   index.html
    403      285      218   .htaccess
    200      558       26   robots.txt
    302     1124        0   .       , Location: https://wolke.gnuzifer.de/index.php/login
    403      285      214   .html
625 requests, 162 req/s, 14375 todo, 1m28s remaining, current: process_order.php
200: 3
302: 2
403: 2
404: 618
```

![basic demo](demos/demo1.gif)
