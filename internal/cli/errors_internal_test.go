package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

func TestMessage(t *testing.T) {
	t.Parallel()

	encodings := "Known encodings: " + strings.Join(stbox.Encodings(), ", ") + "."
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"parse error with a line", &stbox.ParseError{Format: stbox.SubRip, Line: 6, Msg: "Block #1 has no timing line."}, "line 6: Block #1 has no timing line."},
		{"parse error without a line", &stbox.ParseError{Format: stbox.JSON, Msg: "The field cues must be a list."}, "The field cues must be a list."},
		{"parse error with a cause only", &stbox.ParseError{Err: errors.New("cause")}, "cause"},
		{"wrapped parse error", fmt.Errorf("read: %w", &stbox.ParseError{Line: 2, Msg: "Bad."}), "line 2: Bad."},
		{"parse error with an option", &stbox.ParseError{Msg: "Bad.", Option: "ReadOptions.Encoding"}, "Bad. " + encodings},
		{"unknown format", fmt.Errorf("%w: %s", stbox.ErrUnknownFormat, "Format detection found no subtitle format."), unknownFormatMessage},
		{"invalid argument", &stbox.InvalidArgumentError{Option: "ReadOptions.Encoding", Msg: `The encoding "CP1125" is unknown.`}, `The encoding "CP1125" is unknown. ` + encodings},
		{"invalid argument without advice", &stbox.InvalidArgumentError{Option: "WriteOptions.BOM", Msg: "The BOM rule 7 is unknown."}, "The BOM rule 7 is unknown."},
		{"unwritable", &stbox.UnwritableError{Format: stbox.JSON, Reason: "The output cannot hold it."}, "The output cannot hold it."},
		{"unwritable with a cause only", &stbox.UnwritableError{Err: errors.New("cause")}, "cause"},
		{"missing file", fmt.Errorf("load subtitle: %w", &fs.PathError{Op: "open", Path: "/x", Err: fs.ErrNotExist}), missingFileMessage},
		{"unreadable file", fmt.Errorf("load subtitle: %w", &fs.PathError{Op: "read", Path: "/x", Err: fs.ErrPermission}), "Cannot read the file."},
		{"wrapped sentinel", fmt.Errorf("%w: %s", stbox.ErrUnsupportedFormat, "The format txt can be written but not read."), "The format txt can be written but not read."},
		{"twice wrapped sentinel", fmt.Errorf("load: %w", fmt.Errorf("%w: index 4, the subtitle has 2 cues", stbox.ErrCueNotFound)), "index 4, the subtitle has 2 cues"},
		{"bare sentinel", stbox.ErrParse, stbox.ErrParse.Error()},
		{"usage error", usage("The option --to is given twice."), "The option --to is given twice."},
		{"file failure", &fileError{msg: "Cannot create x."}, "Cannot create x."},
		{"other error", errors.New("write /dev/full: no space left on device"), "write /dev/full: no space left on device"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, message(tt.err))
		})
	}
}

func TestExitCode(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, ExitCode(nil))
	assert.Equal(t, 130, ExitCode(&ExitError{Code: 130}))
	assert.Equal(t, 2, ExitCode(fmt.Errorf("run: %w", &ExitError{Code: 2})))
	assert.Equal(t, 3, ExitCode(errors.New("other")))
	assert.Equal(t, "exit code 3", (&ExitError{Code: 3}).Error())
}

func TestOtherErrorsOfACommandExitWith3(t *testing.T) {
	t.Parallel()

	cl := call{commands: []command{failingCommand{summaryCommand{"convert", "Convert."}}}}
	assert.Equal(t, result{Code: exitFile, Stderr: "Error: The format txt can be written but not read.\n"}, cl.run(t, "convert"))
}

type failingCommand struct{ summaryCommand }

func (failingCommand) run(context.Context, *args, *console) error {
	return fmt.Errorf("%w: %s", stbox.ErrUnsupportedFormat, "The format txt can be written but not read.")
}
