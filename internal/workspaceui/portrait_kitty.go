package workspaceui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Kitty graphics protocol "Unicode placeholder" rendering (U=1 virtual
// placements). The image is transmitted once as a virtual placement, then the
// portrait area is filled with a grid of regular placeholder characters
// (U+10EEEE + row/column diacritics, image ID in the foreground color). The
// terminal paints pixels over those cells, while lipgloss and Bubble Tea see
// plain single-width text — so the portrait composes and repaints correctly,
// and tmux forwards the placeholder cells natively. Only the one-time
// transmission needs tmux passthrough (allow-passthrough on).
//
// Supported by kitty and Ghostty. Other terminals fall back to chafa
// character art (see renderPortrait).

// kittyPlaceholder is the placeholder codepoint defined by the protocol.
const kittyPlaceholder = '\U0010EEEE'

// kittyDiacritics maps a row or column index to the combining character that
// encodes it on a placeholder cell. From kitty's rowcolumn-diacritics.txt.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
	0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611,
	0x0612, 0x0613, 0x0614, 0x0615, 0x0616, 0x0617, 0x0657, 0x0658,
	0x0659, 0x065A, 0x065B, 0x065D, 0x065E, 0x06D6, 0x06D7, 0x06D8,
	0x06D9, 0x06DA, 0x06DB, 0x06DC, 0x06DF, 0x06E0, 0x06E1, 0x06E2,
	0x06E4, 0x06E7, 0x06E8, 0x06EB, 0x06EC, 0x0730, 0x0732, 0x0733,
	0x0735, 0x0736, 0x073A, 0x073D, 0x073F, 0x0740, 0x0741, 0x0743,
	0x0745, 0x0747, 0x0749, 0x074A, 0x07EB, 0x07EC, 0x07ED, 0x07EE,
	0x07EF, 0x07F0, 0x07F1, 0x07F3, 0x0816, 0x0817, 0x0818, 0x0819,
	0x081B, 0x081C, 0x081D, 0x081E, 0x081F, 0x0820, 0x0821, 0x0822,
	0x0823, 0x0825, 0x0826, 0x0827, 0x0829, 0x082A, 0x082B, 0x082C,
	0x082D, 0x0951, 0x0953, 0x0954, 0x0F82, 0x0F83, 0x0F86, 0x0F87,
	0x135D, 0x135E, 0x135F, 0x17DD, 0x193A, 0x1A17, 0x1A75, 0x1A76,
	0x1A77, 0x1A78, 0x1A79, 0x1A7A, 0x1A7B, 0x1A7C, 0x1B6B, 0x1B6D,
	0x1B6E, 0x1B6F, 0x1B70, 0x1B71, 0x1B72, 0x1B73, 0x1CD0, 0x1CD1,
	0x1CD2, 0x1CDA, 0x1CDB, 0x1CE0, 0x1DC0, 0x1DC1, 0x1DC3, 0x1DC4,
	0x1DC5, 0x1DC6, 0x1DC7, 0x1DC8, 0x1DC9, 0x1DCB, 0x1DCC, 0x1DD1,
	0x1DD2, 0x1DD3, 0x1DD4, 0x1DD5, 0x1DD6, 0x1DD7, 0x1DD8, 0x1DD9,
	0x1DDA, 0x1DDB, 0x1DDC, 0x1DDD, 0x1DDE, 0x1DDF, 0x1DE0, 0x1DE1,
	0x1DE2, 0x1DE3, 0x1DE4, 0x1DE5, 0x1DE6, 0x1DFE, 0x20D0, 0x20D1,
	0x20D4, 0x20D5, 0x20D6, 0x20D7, 0x20DB, 0x20DC, 0x20E1, 0x20E7,
	0x20E9, 0x20F0, 0x2CEF, 0x2CF0, 0x2CF1, 0x2DE0, 0x2DE1, 0x2DE2,
	0x2DE3, 0x2DE4, 0x2DE5, 0x2DE6, 0x2DE7, 0x2DE8, 0x2DE9, 0x2DEA,
	0x2DEB, 0x2DEC, 0x2DED, 0x2DEE, 0x2DEF, 0x2DF0, 0x2DF1, 0x2DF2,
	0x2DF3, 0x2DF4, 0x2DF5, 0x2DF6, 0x2DF7, 0x2DF8, 0x2DF9, 0x2DFA,
	0x2DFB, 0x2DFC, 0x2DFD, 0x2DFE, 0x2DFF, 0xA66F, 0xA67C, 0xA67D,
	0xA6F0, 0xA6F1, 0xA8E0, 0xA8E1, 0xA8E2, 0xA8E3, 0xA8E4, 0xA8E5,
	0xA8E6, 0xA8E7, 0xA8E8, 0xA8E9, 0xA8EA, 0xA8EB, 0xA8EC, 0xA8ED,
	0xA8EE, 0xA8EF, 0xA8F0, 0xA8F1, 0xAAB0, 0xAAB2, 0xAAB3, 0xAAB7,
	0xAAB8, 0xAABE, 0xAABF, 0xAAC1, 0xFE20, 0xFE21, 0xFE22, 0xFE23,
	0xFE24, 0xFE25, 0xFE26, 0x10A0F, 0x10A38, 0x1D185, 0x1D186, 0x1D187,
	0x1D188, 0x1D189, 0x1D1AA, 0x1D1AB, 0x1D1AC, 0x1D1AD, 0x1D242, 0x1D243,
	0x1D244,
}

// kittyPortraitSupported reports whether the terminal understands Unicode
// placeholders. ORC_PORTRAIT=kitty forces pixel mode on; ORC_PORTRAIT=symbols
// forces the chafa character-art fallback. Detection is env-based because
// inside tmux TERM is rewritten, so we rely on the variables kitty and
// Ghostty export into the environment the tmux server inherits.
func kittyPortraitSupported() bool {
	switch os.Getenv("ORC_PORTRAIT") {
	case "kitty":
		return true
	case "symbols":
		return false
	}
	// Without allow-passthrough tmux drops the image transmission but the
	// placeholder cells would still render, as garbage — fall back instead.
	if os.Getenv("TMUX") != "" && !tmuxAllowsPassthrough() {
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" || os.Getenv("GHOSTTY_BIN_DIR") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return strings.Contains(term, "kitty") || strings.Contains(term, "ghostty")
}

// tmuxAllowsPassthrough checks the pane's effective allow-passthrough value.
// Stubbed in tests.
var tmuxAllowsPassthrough = func() bool {
	out, err := exec.Command("tmux", "show", "-Apv", "allow-passthrough").Output()
	if err != nil {
		return false
	}
	v := strings.TrimSpace(string(out))
	return strings.HasPrefix(v, "on") || strings.HasPrefix(v, "all")
}

// kittyImageIDs assigns each class a stable image ID in 1..255 so the ID fits
// in a 256-color SGR foreground, which tmux forwards unmodified.
var kittyImageIDs = map[string]int{}

func kittyImageID(class string) int {
	if id, ok := kittyImageIDs[class]; ok {
		return id
	}
	id := len(kittyImageIDs)%255 + 1
	kittyImageIDs[class] = id
	return id
}

// kittyTransmit sends png as a virtual placement (U=1) sized cols×rows cells,
// chunked per the protocol. When wrap is true each escape is wrapped in a
// tmux passthrough sequence. q=2 suppresses terminal responses, which would
// otherwise arrive on Bubble Tea's stdin as garbage input.
func kittyTransmit(w io.Writer, id int, png []byte, cols, rows int, wrap bool) error {
	b64 := base64.StdEncoding.EncodeToString(png)
	const chunkSize = 4096
	first := true
	for len(b64) > 0 {
		n := min(chunkSize, len(b64))
		chunk, rest := b64[:n], b64[n:]
		more := 0
		if len(rest) > 0 {
			more = 1
		}
		var ctrl string
		if first {
			ctrl = fmt.Sprintf("a=T,U=1,q=2,f=100,i=%d,c=%d,r=%d,m=%d", id, cols, rows, more)
			first = false
		} else {
			ctrl = fmt.Sprintf("m=%d", more)
		}
		seq := "\x1b_G" + ctrl + ";" + chunk + "\x1b\\"
		if wrap {
			seq = "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
		}
		if _, err := io.WriteString(w, seq); err != nil {
			return err
		}
		b64 = rest
	}
	return nil
}

// kittyDelete frees the image and any placements registered under id. Sent
// before every retransmission: the id is stable per class, so without the
// delete a resize would leave the previous placement (with the old c×r grid)
// alive and the terminal could resolve placeholder cells against it.
func kittyDelete(w io.Writer, id int, wrap bool) {
	seq := fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
	if wrap {
		seq = "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
	}
	_, _ = io.WriteString(w, seq)
}

// padToPlacementAspect pads the PNG with transparent pixels so its aspect
// ratio exactly matches a cols×rows cell rectangle. The terminal scales an
// image to fit its placement preserving aspect ratio and centers the result,
// so any mismatch floats the art toward the middle. Matching exactly makes
// the image fill the placement: width padding is split evenly (art stays
// horizontally centered) and height padding goes entirely below (art stays
// top-aligned). Returns the input unchanged when it cannot decode or encode.
func padToPlacementAspect(data []byte, cols, rows int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	cw, ch := cellPixelSize()
	if cw <= 0 || ch <= 0 {
		cw, ch = 1, 2
	}
	boxW, boxH := cols*cw, rows*ch
	b := img.Bounds()
	imgW, imgH := b.Dx(), b.Dy()
	if imgW == 0 || imgH == 0 || boxW == 0 || boxH == 0 {
		return data
	}

	var out *image.RGBA
	if imgH*boxW >= imgW*boxH {
		// image is relatively taller than the box — pad width evenly
		newW := imgH * boxW / boxH
		out = image.NewRGBA(image.Rect(0, 0, newW, imgH))
		draw.Draw(out, image.Rect((newW-imgW)/2, 0, (newW-imgW)/2+imgW, imgH), img, b.Min, draw.Src)
	} else {
		// image is relatively wider than the box — pad height below only
		newH := imgW * boxH / boxW
		out = image.NewRGBA(image.Rect(0, 0, imgW, newH))
		draw.Draw(out, image.Rect(0, 0, imgW, imgH), img, b.Min, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return data
	}
	return buf.Bytes()
}

// kittyPlaceholderGrid returns the cols×rows placeholder text that the
// terminal replaces with the transmitted image. Each line carries its own
// foreground SGR holding the image ID so Bubble Tea can repaint any line
// independently without losing the association.
func kittyPlaceholderGrid(id, cols, rows int) string {
	var b strings.Builder
	for r := 0; r < rows; r++ {
		if r > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "\x1b[38;5;%dm", id)
		for c := 0; c < cols; c++ {
			b.WriteRune(kittyPlaceholder)
			b.WriteRune(kittyDiacritics[r])
			b.WriteRune(kittyDiacritics[c])
		}
		b.WriteString("\x1b[39m")
	}
	return b.String()
}

// renderPortraitKitty transmits the portrait and returns its placeholder
// grid, or "" when the size is unrepresentable or the write fails.
func renderPortraitKitty(class string, png []byte, cols, rows int) string {
	if cols < 1 || rows < 1 || cols > len(kittyDiacritics) || rows > len(kittyDiacritics) {
		return ""
	}
	id := kittyImageID(class)
	wrap := os.Getenv("TMUX") != ""
	kittyDelete(os.Stdout, id, wrap)
	if err := kittyTransmit(os.Stdout, id, padToPlacementAspect(png, cols, rows), cols, rows, wrap); err != nil {
		return ""
	}
	return kittyPlaceholderGrid(id, cols, rows)
}
