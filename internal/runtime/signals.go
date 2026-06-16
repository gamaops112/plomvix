package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// withSignalContext returns a context that is cancelled when SIGTERM, SIGINT,
// SIGHUP, or SIGQUIT is received. The caller must call the returned cancel
// function to release resources.
func withSignalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)

	signalCtx, cancel := withSignalContextFromChan(ctx, ch)

	return signalCtx, func() {
		signal.Stop(ch)
		cancel()
	}
}

// withSignalContextFromChan returns a context that is cancelled when a signal
// is received on ch or when the returned cancel function is called.
func withSignalContextFromChan(ctx context.Context, ch <-chan os.Signal) (context.Context, context.CancelFunc) {
	signalCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-signalCtx.Done():
		}
	}()
	return signalCtx, cancel
}

// RunWithSignals is the production entry point that creates a signal-aware
// context and calls Run. The returned context is cancelled on SIGTERM, SIGINT,
// SIGHUP, or SIGQUIT, triggering a clean shutdown.
func RunWithSignals(opts Options) error {
	signalCtx, cancel := withSignalContext(context.Background())
	defer cancel()
	return Run(signalCtx, opts)
}
