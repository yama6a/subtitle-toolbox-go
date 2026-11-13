package orderedjson

import "errors"

var (
	// ErrInvalidNumber means that a text is not a JSON number literal.
	ErrInvalidNumber = errors.New("not a JSON number literal")
	// ErrNotFinite means that Encode met a NaN or an infinity.
	ErrNotFinite = errors.New("not finite")
	// ErrInvalidUTF8 means that Encode met a string that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("invalid UTF-8")
)
