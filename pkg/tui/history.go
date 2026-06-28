package tui

// inputHistory recalls previously submitted prompts via Up/Down.
type inputHistory struct {
	items []string
	pos   int // index into items; len(items) == "current empty line"
}

func newInputHistory() *inputHistory {
	return &inputHistory{pos: 0}
}

func (h *inputHistory) add(s string) {
	if s == "" {
		return
	}
	h.items = append(h.items, s)
	h.pos = len(h.items)
}

// prev moves toward older entries and returns the entry (or "" if none).
func (h *inputHistory) prev() string {
	if len(h.items) == 0 {
		return ""
	}
	if h.pos > 0 {
		h.pos--
	}
	return h.items[h.pos]
}

// next moves toward newer entries; returns "" past the newest.
func (h *inputHistory) next() string {
	if len(h.items) == 0 {
		return ""
	}
	if h.pos < len(h.items)-1 {
		h.pos++
		return h.items[h.pos]
	}
	h.pos = len(h.items)
	return ""
}
