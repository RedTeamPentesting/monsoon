package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/fd0/termstatus"
	"github.com/fd0/termstatus/progress"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	t := termstatus.New(os.Stdout, os.Stderr, false)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		t.Run(ctx)
		wg.Done()
	}()

	rd := progress.Reader(os.Stdin, t)

	io.Copy(ioutil.Discard, rd)

	fmt.Printf("done\n")
	cancel()
	wg.Wait()
}
