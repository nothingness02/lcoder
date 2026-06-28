package tui

import "testing"

func TestSpinnerGlyphCycles(t *testing.T) {
	s := newSpinner()
	g0 := s.glyph()
	s.frame++
	g1 := s.glyph()
	if g0 == g1 {
		t.Fatal("spinner glyph did not advance")
	}
}

func TestSpinnerPhraseStable(t *testing.T) {
	s := newSpinner()
	p := s.phrase()
	if p == "" {
		t.Fatal("empty phrase")
	}
}
