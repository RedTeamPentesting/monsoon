package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fd0/termstatus"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Second)
		cancel()
	}()

	t := termstatus.New(os.Stdout, os.Stderr, false)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		t.Run(ctx)
		wg.Done()
	}()

	go func() {
		i := 1
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			t.Printf("message %v\n", i)
			time.Sleep(300 * time.Millisecond)
			i++
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		lines := []string{
			fmt.Sprintf("current time: %v", time.Now()),
			"foobar line2",
		}

		t.SetStatus(lines)
		time.Sleep(50 * time.Millisecond)
	}

	wg.Wait()
}
