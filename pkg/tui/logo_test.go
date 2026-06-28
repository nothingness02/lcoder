package tui

import (
	"strings"
	"testing"
)

func TestLogoFrameLineCount(t *testing.T) {
	for n := 0; n <= logoFrames; n++ {
		out := logoFrame(n)
		lines := strings.Split(stripANSI(out), "\n")
		if len(lines) != logoHeight {
			t.Fatalf("frame %d has %d lines, want %d", n, len(lines), logoHeight)
		}
	}
}

func TestLogoFrameColumnsBounded(t *testing.T) {
	full := logoFrame(logoFrames)
	for _, ln := range strings.Split(stripANSI(full), "\n") {
		if displayWidth(ln) > logoWidth {
			t.Fatalf("line width %d exceeds logoWidth %d", displayWidth(ln), logoWidth)
		}
	}
}
