package cli

import (
	"errors"
	"io/fs"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
)

// The exit codes of docs/errors.md.
const (
	exitResult    = 1
	exitUsage     = 2
	exitFile      = 3
	exitInterrupt = 130
)

// ExitError carries the exit code of a call whose messages are printed.
type ExitError struct {
	// Code is the exit code of the process.
	Code int
}

// Error returns the code, for example "exit code 3".
func (e *ExitError) Error() string {
	return "exit code " + strconv.Itoa(e.Code)
}

// ExitCode returns 0 for nil, Code for an *ExitError, and 3 for any other error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := errors.AsType[*ExitError](err); ok {
		return e.Code
	}
	return exitFile
}

// usageError is an error of the arguments or of a check before the first read. Outside the input loop it gives
// exit code 2 and the help hint. Inside the loop it fails the input.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usage(msg string) error { return &usageError{msg: msg} }

// fileError is an output that cannot be created or a side file that cannot be read. It stops the run at once
// with exit code 3, also with --keep-going.
type fileError struct{ msg string }

func (e *fileError) Error() string { return e.msg }

// asUsage turns any error of the steps before the input loop into a usage error.
func asUsage(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*fileError](err); ok {
		return err
	}
	return usage(message(err))
}

const (
	unknownFormatMessage = "Format detection found no subtitle format. Pass --from FORMAT."
	missingFileMessage   = "The file does not exist."
	lenientHint          = "Pass --lenient to skip or repair broken cues."
	encodingHint         = "Pass --encoding to set the source encoding."
)

// message returns the CLI text of err, without a label. It names no Go type and no Go option path.
func message(err error) string {
	if msg, ok := typedMessage(err); ok {
		return msg
	}
	switch {
	case errors.Is(err, stbox.ErrUnknownFormat):
		return unknownFormatMessage
	case errors.Is(err, fs.ErrNotExist):
		return missingFileMessage
	}
	if _, ok := errors.AsType[*fs.PathError](err); ok {
		return "Cannot read the file."
	}
	for _, sentinel := range []error{
		stbox.ErrParse, stbox.ErrUnsupportedFormat, stbox.ErrInvalidArgument, stbox.ErrCueNotFound, stbox.ErrUnwritable,
	} {
		if !errors.Is(err, sentinel) {
			continue
		}
		if _, after, ok := strings.Cut(err.Error(), sentinel.Error()+": "); ok {
			return after
		}
	}
	return err.Error()
}

// typedMessage returns the message of the error types of the CLI and of the library.
func typedMessage(err error) (string, bool) {
	if e, ok := errors.AsType[*usageError](err); ok {
		return e.msg, true
	}
	if e, ok := errors.AsType[*fileError](err); ok {
		return e.msg, true
	}
	if e, ok := errors.AsType[*stbox.ParseError](err); ok {
		msg := withCause(e.Msg, e.Err)
		if e.Line > 0 {
			msg = "line " + strconv.Itoa(e.Line) + ": " + msg
		}
		return withAdvice(msg, e.Option, e.Err), true
	}
	if e, ok := errors.AsType[*stbox.InvalidArgumentError](err); ok {
		return withAdvice(e.Msg, e.Option, nil), true
	}
	if e, ok := errors.AsType[*stbox.UnwritableError](err); ok {
		return withAdvice(withCause(e.Reason, e.Err), "", e.Err), true
	}
	return "", false
}

func withCause(msg string, cause error) string {
	if msg == "" && cause != nil {
		return cause.Error()
	}
	return msg
}

// withAdvice adds the CLI sentence for the Go option path or the sentinel cause that an error names.
func withAdvice(msg, option string, cause error) string {
	switch {
	case option == "ReadOptions.Encoding":
		return msg + " Known encodings: " + strings.Join(stbox.Encodings(), ", ") + "."
	case errors.Is(cause, stboxjson.ErrInvalidUTF8):
		return msg + " Pass --encoding with the source encoding of the file."
	default:
		return msg
	}
}
