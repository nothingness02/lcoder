package tui

import "testing"

func TestInputAutoGrow(t *testing.T) {
	m := NewInputModel()
	m.SetWidth(40)
	m.textarea.SetValue("one\ntwo\nthree")
	if h := m.desiredHeight(); h < 3 {
		t.Fatalf("desiredHeight = %d, want >= 3", h)
	}
}

func TestInputHeightCapped(t *testing.T) {
	m := NewInputModel()
	m.SetWidth(40)
	m.textarea.SetValue("a\nb\nc\nd\ne\nf\ng\nh\ni")
	if h := m.desiredHeight(); h > 6 {
		t.Fatalf("desiredHeight = %d, want <= 6", h)
	}
}

func TestInputValue(t *testing.T) {
	m := NewInputModel()
	m.textarea.SetValue("hi")
	if m.Value() != "hi" {
		t.Fatalf("Value = %q", m.Value())
	}
}
