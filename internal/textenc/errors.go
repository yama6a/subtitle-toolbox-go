package textenc

import "errors"

// ErrUnknownEncoding is the error of Lookup for a name outside the supported list.
var ErrUnknownEncoding = errors.New("unknown encoding")

// ConvertError is the error of Decode for content that its encoding does not define.
type ConvertError struct {
	// Encoding is the name of the encoding as the BOM or the caller gave it.
	Encoding string
}

// Error returns the message, for example "Cannot convert the content from Windows-1252 to UTF-8.".
func (e *ConvertError) Error() string {
	return "Cannot convert the content from " + e.Encoding + " to UTF-8."
}
