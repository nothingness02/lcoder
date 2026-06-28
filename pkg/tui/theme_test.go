package tui

import "testing"

func TestAccentResolves(t *testing.T) {
	// styleAccent must produce a non-empty render for non-empty input.
	if out := styleAccent().Render("x"); out == "" {
		t.Fatal("accent render empty")
	}
}

func TestApplyAccentSwapsColor(t *testing.T) {
	orig := colorAccent
	applyAccent(accentPresets[1]) // ocean
	if colorAccent == orig {
		t.Fatal("applyAccent did not change colorAccent")
	}
	applyAccent(accentPresets[0]) // restore frost
}

func TestIsDarkBackgroundStable(t *testing.T) {
	a := isDarkBackground()
	b := isDarkBackground()
	if a != b {
		t.Fatal("isDarkBackground not stable across calls")
	}
}
