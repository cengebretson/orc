package main

import "os"

func main() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already printed the error; just set the exit code
		os.Exit(1)
	}
}
