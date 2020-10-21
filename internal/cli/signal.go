package cli

import (
	"context"
	"os"
	"os/signal"
)

func SignalContext(ctx context.Context, signals ...os.Signal) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()

		c := make(chan os.Signal, 1)
		signal.Notify(c, signals...)
		defer signal.Stop(c)

		<-c
	}()

	return ctx
}
