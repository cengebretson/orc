package main

import "os"

func main() {
	prepareCtlOutput(os.Args[1:])
	if err := rootCmd.Execute(); err != nil {
		if isCtlInvocation(os.Args[1:]) {
			writeCtlError(os.Stderr, err)
		}
		// Cobra prints non-ctl errors. ctl errors are structured above.
		os.Exit(1)
	}
}
