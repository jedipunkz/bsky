package ui

import "testing"

func TestSupportsSixel_OptIn(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "WezTerm")
	t.Setenv("TERM", "xterm-256color")
	if supportsSixel() {
		t.Error("Sixel must stay off unless BSKY_SIXEL=1: TERM_PROGRAM survives multiplexers that drop Sixel")
	}
	t.Setenv("BSKY_SIXEL", "1")
	if !supportsSixel() {
		t.Error("BSKY_SIXEL=1 must enable Sixel")
	}
}
