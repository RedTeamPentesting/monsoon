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
		for sig := range signalCh {
			fmt.Printf("received signal %v\n", sig)
			cancel()
		}
	}()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return f(ctx, g)
	})
	return g.Wait()
}
