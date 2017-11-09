package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/fd0/termstatus"
	"github.com/fd0/termstatus/progress"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t := termstatus.New(ctx, os.Stdout)

	rd := progress.Reader(os.Stdin, t)

	io.Copy(ioutil.Discard, rd)

	err := t.Finish()
	if err != nil {
		panic(err)
	}

	fmt.Printf("done\n")
}
