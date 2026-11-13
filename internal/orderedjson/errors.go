package orderedjson

import "errors"

// The reasons of an *model.UnwritableError for a value that Encode rejects. A writer puts them in its error.
const (
	MalformedUTF8Reason = "JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
		"Read a file in another encoding with its source encoding."
	NotFiniteReason = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
)

var (
	// ErrInvalidNumber means that a text is not a JSON number literal.
	ErrInvalidNumber = errors.New("not a JSON number literal")
	// ErrNotFinite means that Encode met a NaN or an infinity.
	ErrNotFinite = errors.New("not finite")
	// ErrInvalidUTF8 means that Encode met a string that is not valid UTF-8.
	ErrInvalidUTF8 = errors.New("invalid UTF-8")
)
