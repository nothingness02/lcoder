package builtin

import "github.com/lcoder/lcoder/pkg/tools"

// Factories returns all built-in tool factories.
func Factories() []tools.Factory {
	return []tools.Factory{
		NewRead,
		NewWrite,
		NewEdit,
		NewBash,
		NewLs,
		NewGrep,
		NewFind,
	}
}
