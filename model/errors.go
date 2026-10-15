package model

import (
	"errors"
	"fmt"
)

// The sentinel errors. Package stbox re-exports each of them.
var (
	ErrParse             = errors.New("stbox: cannot parse the content")
	ErrUnsupportedFormat = errors.New("stbox: unsupported format")
	ErrUnknownFormat     = fmt.Errorf("%w: no format detected", ErrUnsupportedFormat)
	ErrInvalidArgument   = errors.New("stbox: invalid argument")
	ErrCueNotFound       = errors.New("stbox: cue not found")
	ErrUnwritable        = errors.New("stbox: the format cannot hold the content")
)

// ParseError is an error of a reader that cannot read its input.
type ParseError struct{}

// UnwritableError is an error of a writer whose format cannot hold the subtitle.
type UnwritableError struct{}

// InvalidArgumentError is an error of an invalid option or argument.
type InvalidArgumentError struct{}
