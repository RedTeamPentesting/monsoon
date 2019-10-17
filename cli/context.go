package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

// WithContext runs f with an errgroup.Group and a context. The context is
// cancelled when SIGINT is received or f returns. WithContext returns the
// error from the error group.
func WithContext(f func(context.Context, *errgroup.Group) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancel the context on SIGINT
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT)
	go func() {
		// track the number of events we receive
		received := 0
		for sig := range signalCh {
			if received == 0 {
				// if this is the first signal, try to exit gracefully
				fmt.Printf("received signal %v, finishing gracefully\n", sig)
				cancel()
			} else {
				// else just exit
				fmt.Printf("received signal %v again, exiting\n", sig)
				os.Exit(1)
			}
			received++
		}
	}()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return f(ctx, g)
	})
	return g.Wait()
}
