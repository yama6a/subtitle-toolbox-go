package model

import (
	"errors"
	"fmt"
	"strconv"
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

var (
	_ error = (*ParseError)(nil)
	_ error = (*UnwritableError)(nil)
	_ error = (*InvalidArgumentError)(nil)
)

// ParseError is an error of a reader that cannot read its input.
type ParseError struct {
	// Format is the format that the reader read, "" for a damaged container.
	Format Format
	// Line is the 1-based input line, 0 when the reader does not know it.
	Line int
	// Msg is the message, without the format and the line.
	Msg string
	// Option is the Go path of a missing or bad read option, for example "ReadOptions.MicroDVD.FrameRate", else "".
	Option string
	// Err is the cause, for example a *json.SyntaxError, or nil.
	Err error
}

// Error returns the format, the line and the message, for example "srt: line 6: Block #1 has no timing line ...".
func (e *ParseError) Error() string {
	s := prefix(e.Format)
	if e.Line > 0 {
		s += "line " + strconv.Itoa(e.Line) + ": "
	}
	return s + message(e.Msg, e.Err)
}

// Unwrap returns the cause.
func (e *ParseError) Unwrap() error { return e.Err }

// Is reports whether target is ErrParse.
func (e *ParseError) Is(target error) bool { return target == ErrParse }

// UnwritableError is an error of a writer whose format cannot hold the subtitle.
type UnwritableError struct {
	// Format is the output format.
	Format Format
	// CueIndex is the 0-based index of the first cue that the format cannot hold, -1 when no single cue causes it.
	CueIndex int
	// Reason is the message, without the format.
	Reason string
	// Err is the cause, for example an XML syntax error of a stored TTML head, or nil.
	Err error
}

// Error returns the format and the reason, for example "sami: Text that is not UTF-8 ...".
func (e *UnwritableError) Error() string {
	return prefix(e.Format) + message(e.Reason, e.Err)
}

// Unwrap returns the cause.
func (e *UnwritableError) Unwrap() error { return e.Err }

// Is reports whether target is ErrUnwritable.
func (e *UnwritableError) Is(target error) bool { return target == ErrUnwritable }

// InvalidArgumentError is an error of an invalid option or argument.
type InvalidArgumentError struct {
	// Option is the Go path of the option, for example "WriteOptions.MicroDVD.FrameRate", "" for a plain argument.
	Option string
	// Msg is the message, without the option.
	Msg string
}

// Error returns the option and the message, for example "stbox: invalid argument: WriteOptions.MicroDVD.FrameRate: ...".
func (e *InvalidArgumentError) Error() string {
	s := ErrInvalidArgument.Error() + ": "
	if e.Option != "" {
		s += e.Option + ": "
	}
	return s + e.Msg
}

// Is reports whether target is ErrInvalidArgument.
func (e *InvalidArgumentError) Is(target error) bool { return target == ErrInvalidArgument }

func prefix(f Format) string {
	if f == "" {
		return "stbox: "
	}
	return string(f) + ": "
}

// message leaves out the cause when msg is set, because msg quotes the cause where it matters.
func message(msg string, cause error) string {
	if msg == "" && cause != nil {
		return cause.Error()
	}
	return msg
}
