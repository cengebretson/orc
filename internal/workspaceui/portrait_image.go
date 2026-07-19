package workspaceui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"strings"
)

// portraitCacheEntry holds a previously rendered portrait so chafa isn't
// re-invoked on every Bubble Tea render cycle (e.g. during resize).
type portraitCacheEntry struct {
	class string
	cols  int
	rows  int
	out   string
}

var portraitCache portraitCacheEntry

// pngAspectHeight returns the terminal row count that preserves the image's
// aspect ratio when rendered at cols columns using half-block characters.
// Each terminal cell is ~2× taller than wide; half-blocks encode 2 pixel rows
// per cell, so the two effects cancel: rows ≈ cols × imgH / imgW.
func pngAspectHeight(data []byte, cols int) int {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil || cols == 0 {
		return cols / 2
	}
	b := img.Bounds()
	imgW, imgH := b.Dx(), b.Dy()
	if imgW == 0 {
		return cols / 2
	}
	return cols * imgH / imgW
}

// renderPortrait renders PNG data as a terminal image at cols×rows cells.
// On kitty/Ghostty it uses Kitty Unicode placeholders for true pixel output
// (see portrait_kitty.go); elsewhere it falls back to chafa character art.
// Results are cached by (class, cols, rows) so neither path re-runs on every
// Bubble Tea render cycle. Returns "" if no renderer is available.
func renderPortrait(class string, data []byte, cols, rows int) string {
	if portraitCache.class == class && portraitCache.cols == cols && portraitCache.rows == rows {
		return portraitCache.out
	}

	if kittyPortraitSupported() {
		if out := renderPortraitKitty(class, data, cols, rows); out != "" {
			portraitCache = portraitCacheEntry{class: class, cols: cols, rows: rows, out: out}
			return out
		}
	}

	path, err := exec.LookPath("chafa")
	if err != nil {
		return ""
	}
	tmp, err := os.CreateTemp("", "orc-portrait-*.png")
	if err != nil {
		return ""
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return ""
	}
	_ = tmp.Close()

	out, err := exec.Command(path,
		"--size", fmt.Sprintf("%dx%d", cols, rows),
		"--format", "symbols", // Kitty/pixel output breaks lipgloss layout; character art composes correctly
		"--colors", "full",
		"--symbols", "half",
		tmp.Name(),
	).Output()
	if err != nil {
		return ""
	}
	result := strings.TrimRight(string(out), "\n")
	portraitCache = portraitCacheEntry{class: class, cols: cols, rows: rows, out: result}
	return result
}
