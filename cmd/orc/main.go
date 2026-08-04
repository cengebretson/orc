package main

import (
	"context"
	"os"
	"os/signal"
)

func main() {
	prepareCtlOutput(os.Args[1:])
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if isCtlInvocation(os.Args[1:]) {
			writeCtlError(os.Stderr, err)
		}
		// Cobra prints non-ctl errors. ctl errors are structured above.
		os.Exit(1)
	}
}
