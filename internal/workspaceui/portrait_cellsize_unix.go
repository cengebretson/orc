//go:build unix

package workspaceui

import (
	"os"

	"golang.org/x/sys/unix"
)

// cellPixelSize returns the terminal's cell size in pixels, or (0, 0) when
// the terminal doesn't report pixel dimensions (e.g. some tmux setups).
// Variable so tests can stub it.
var cellPixelSize = func() (w, h int) {
	for _, f := range []*os.File{os.Stdout, os.Stdin} {
		ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
			continue
		}
		return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row)
	}
	return 0, 0
}
