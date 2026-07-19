package workspaceui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestKittyPlaceholderGridComposesAsText(t *testing.T) {
	const cols, rows = 24, 12
	grid := kittyPlaceholderGrid(7, cols, rows)

	lines := strings.Split(grid, "\n")
	if len(lines) != rows {
		t.Fatalf("got %d lines, want %d", len(lines), rows)
	}
	// lipgloss must measure each placeholder line as cols cells wide —
	// this is the property that raw Kitty escape output lacks.
	for i, line := range lines {
		if w := lipgloss.Width(line); w != cols {
			t.Errorf("line %d: lipgloss.Width = %d, want %d", i, w, cols)
		}
		if !strings.HasPrefix(line, "\x1b[38;5;7m") {
			t.Errorf("line %d: missing image-ID foreground SGR", i)
		}
		if !strings.HasSuffix(line, "\x1b[39m") {
			t.Errorf("line %d: missing foreground reset", i)
		}
	}

	// Each cell encodes its row and column via combining diacritics.
	wantCell := string(kittyPlaceholder) + string(kittyDiacritics[3]) + string(kittyDiacritics[5])
	if !strings.Contains(lines[3], wantCell) {
		t.Errorf("line 3 missing placeholder cell for row 3, col 5")
	}
}

func TestKittyTransmitSingleChunk(t *testing.T) {
	var buf bytes.Buffer
	if err := kittyTransmit(&buf, 9, []byte("png-bytes"), 10, 5, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "\x1b_Ga=T,U=1,q=2,f=100,i=9,c=10,r=5,m=0;") {
		t.Errorf("unexpected control data: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("missing escape terminator: %q", out)
	}
}

func TestKittyTransmitChunksAndTmuxWrap(t *testing.T) {
	var buf bytes.Buffer
	data := bytes.Repeat([]byte{0xAB}, 5000) // base64 length > 4096 → two chunks
	if err := kittyTransmit(&buf, 3, data, 8, 4, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if got := strings.Count(out, "\x1bPtmux;"); got != 2 {
		t.Fatalf("got %d tmux passthrough wrappers, want 2", got)
	}
	if !strings.Contains(out, "\x1b\x1b_Ga=T,U=1,q=2,f=100,i=3,c=8,r=4,m=1;") {
		t.Errorf("first chunk missing doubled-ESC control data: %q", out[:80])
	}
	if !strings.Contains(out, "\x1b\x1b_Gm=0;") {
		t.Errorf("final chunk missing m=0 continuation")
	}
}

func TestKittyPortraitSupported(t *testing.T) {
	for _, v := range []string{"ORC_PORTRAIT", "KITTY_WINDOW_ID", "GHOSTTY_RESOURCES_DIR", "GHOSTTY_BIN_DIR", "TERM", "TMUX"} {
		t.Setenv(v, "")
	}
	if kittyPortraitSupported() {
		t.Error("supported with no terminal hints")
	}
	t.Setenv("TERM", "xterm-ghostty")
	if !kittyPortraitSupported() {
		t.Error("not supported with TERM=xterm-ghostty")
	}
	t.Setenv("ORC_PORTRAIT", "symbols")
	if kittyPortraitSupported() {
		t.Error("ORC_PORTRAIT=symbols should force fallback")
	}
	t.Setenv("ORC_PORTRAIT", "kitty")
	if !kittyPortraitSupported() {
		t.Error("ORC_PORTRAIT=kitty should force pixel mode")
	}
}

func TestKittyPortraitSupportedTmuxPassthrough(t *testing.T) {
	for _, v := range []string{"ORC_PORTRAIT", "KITTY_WINDOW_ID", "GHOSTTY_RESOURCES_DIR", "GHOSTTY_BIN_DIR"} {
		t.Setenv(v, "")
	}
	t.Setenv("TERM", "xterm-ghostty")
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")

	orig := tmuxAllowsPassthrough
	defer func() { tmuxAllowsPassthrough = orig }()

	tmuxAllowsPassthrough = func() bool { return false }
	if kittyPortraitSupported() {
		t.Error("supported inside tmux without allow-passthrough")
	}
	tmuxAllowsPassthrough = func() bool { return true }
	if !kittyPortraitSupported() {
		t.Error("not supported inside tmux with allow-passthrough on")
	}
}

func TestKittyDelete(t *testing.T) {
	var buf bytes.Buffer
	kittyDelete(&buf, 7, false)
	if got := buf.String(); got != "\x1b_Ga=d,d=I,i=7,q=2\x1b\\" {
		t.Errorf("delete sequence = %q", got)
	}

	buf.Reset()
	kittyDelete(&buf, 7, true)
	out := buf.String()
	if !strings.HasPrefix(out, "\x1bPtmux;") || !strings.Contains(out, "\x1b\x1b_Ga=d,d=I,i=7,q=2") {
		t.Errorf("tmux-wrapped delete sequence = %q", out)
	}
}

func encodeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPadToPlacementAspect(t *testing.T) {
	orig := cellPixelSize
	defer func() { cellPixelSize = orig }()
	cellPixelSize = func() (int, int) { return 10, 20 }

	decode := func(data []byte) image.Image {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}

	// Square image into a 10×10-cell box (100×200 px): image is relatively
	// wider, so it gains transparent rows below — art stays at the top.
	out := decode(padToPlacementAspect(encodeTestPNG(t, 100, 100), 10, 10))
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 200 {
		t.Fatalf("bottom-pad dims = %dx%d, want 100x200", out.Bounds().Dx(), out.Bounds().Dy())
	}
	if _, _, _, a := out.At(50, 50).RGBA(); a == 0 {
		t.Error("art region should stay opaque at the top")
	}
	if _, _, _, a := out.At(50, 150).RGBA(); a != 0 {
		t.Error("padding below the art should be transparent")
	}

	// Tall image into a wide 30×10-cell box (300×200 px): image is relatively
	// taller, so width padding is split evenly and the art stays centered.
	out = decode(padToPlacementAspect(encodeTestPNG(t, 100, 400), 30, 10))
	if out.Bounds().Dx() != 600 || out.Bounds().Dy() != 400 {
		t.Fatalf("side-pad dims = %dx%d, want 600x400", out.Bounds().Dx(), out.Bounds().Dy())
	}
	if _, _, _, a := out.At(300, 200).RGBA(); a == 0 {
		t.Error("art should remain centered horizontally")
	}
	if _, _, _, a := out.At(10, 200).RGBA(); a != 0 {
		t.Error("left padding should be transparent")
	}

	// Garbage input is returned unchanged.
	if got := padToPlacementAspect([]byte("not a png"), 4, 4); string(got) != "not a png" {
		t.Error("undecodable input should pass through")
	}
}

func TestRenderPortraitKittyDeletesBeforeRetransmit(t *testing.T) {
	t.Setenv("TMUX", "")
	pngData := encodeTestPNG(t, 8, 8)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	got := renderPortraitKitty("RESIZE-TEST", pngData, 4, 4)
	os.Stdout = orig
	_ = w.Close()
	captured, _ := io.ReadAll(r)

	if got == "" {
		t.Fatal("renderPortraitKitty returned no grid")
	}
	out := string(captured)
	del := strings.Index(out, "a=d,d=I")
	tx := strings.Index(out, "a=T,U=1")
	if del == -1 || tx == -1 || del > tx {
		t.Errorf("expected delete before transmit, got delete@%d transmit@%d", del, tx)
	}
}

func TestKittyImageIDStable(t *testing.T) {
	a := kittyImageID("WARRIOR-test")
	if a < 1 || a > 255 {
		t.Fatalf("id %d outside 1..255", a)
	}
	if b := kittyImageID("WARRIOR-test"); b != a {
		t.Errorf("id not stable: %d then %d", a, b)
	}
	if c := kittyImageID(fmt.Sprintf("OTHER-%d", a)); c == a {
		t.Errorf("distinct classes share id %d", a)
	}
}
