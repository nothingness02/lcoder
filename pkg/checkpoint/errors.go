package checkpoint

import "errors"

// Common errors returned by checkpoint operations.
var (
	ErrVersionMismatch = errors.New("checkpoint version mismatch")
	ErrNotFound        = errors.New("checkpoint not found")
)
