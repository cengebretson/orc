package tmux

import "os/exec"

// Process boundaries are variables so command construction can be tested
// without requiring a live tmux server.
var (
	newCommand     = exec.Command
	findExecutable = exec.LookPath
)
