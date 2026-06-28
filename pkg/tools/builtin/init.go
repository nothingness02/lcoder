package builtin

import (
	"github.com/lcoder/lcoder/pkg/tools"
)

func init() {
	for _, f := range []struct {
		name    string
		factory tools.Factory
	}{
		{"read", NewRead},
		{"write", NewWrite},
		{"edit", NewEdit},
		{"bash", NewBash},
		{"ls", NewLs},
		{"grep", NewGrep},
		{"find", NewFind},
	} {
		tools.DefaultFactories.Register(f.name, f.factory)
	}
}
