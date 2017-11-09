package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fd0/termstatus"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Second)
		cancel()
	}()

	t := termstatus.New(ctx, os.Stdout)

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

		err := t.SetStatus(lines)
		if err != nil {
			panic(err)
		}
		time.Sleep(50 * time.Millisecond)

	}
}
