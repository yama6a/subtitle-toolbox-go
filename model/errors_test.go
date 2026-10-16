package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	cause := errors.New("unexpected end of JSON input")
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "parse error with a line",
			err: &model.ParseError{
				Format: model.SubRip,
				Line:   6,
				Msg:    `Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".`,
			},
			want: `srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".`,
		},
		{
			name: "parse error without a line",
			err:  &model.ParseError{Format: model.JSON, Msg: "The JSON root must be an object."},
			want: "json: The JSON root must be an object.",
		},
		{
			name: "parse error with a message and a cause",
			err: &model.ParseError{
				Format: model.JSON,
				Msg:    "The content is not valid JSON: unexpected end of JSON input.",
				Err:    cause,
			},
			want: "json: The content is not valid JSON: unexpected end of JSON input.",
		},
		{
			name: "parse error with a cause and no message",
			err:  &model.ParseError{Format: model.WebVTT, Line: 3, Err: cause},
			want: "vtt: line 3: unexpected end of JSON input",
		},
		{
			name: "parse error with an empty format",
			err:  &model.ParseError{Msg: "The file is not a Matroska or WebM file."},
			want: "stbox: The file is not a Matroska or WebM file.",
		},
		{
			name: "parse error with an empty format and a line",
			err:  &model.ParseError{Line: 2, Msg: "The file is not a Matroska or WebM file."},
			want: "stbox: line 2: The file is not a Matroska or WebM file.",
		},
		{
			name: "unwritable error",
			err:  &model.UnwritableError{Format: model.SAMI, CueIndex: 3, Reason: "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding."},
			want: "sami: The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.",
		},
		{
			name: "unwritable error with a message and a cause",
			err: &model.UnwritableError{
				Format:   model.TTML,
				CueIndex: -1,
				Reason:   "The stored TTML head is not a well-formed <head> element.",
				Err:      cause,
			},
			want: "ttml: The stored TTML head is not a well-formed <head> element.",
		},
		{
			name: "unwritable error with a cause and no message",
			err:  &model.UnwritableError{Format: model.TTML, CueIndex: -1, Err: cause},
			want: "ttml: unexpected end of JSON input",
		},
		{
			name: "unwritable error with an empty format",
			err:  &model.UnwritableError{CueIndex: -1, Reason: "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding."},
			want: "stbox: The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.",
		},
		{
			name: "invalid argument error with an option",
			err:  &model.InvalidArgumentError{Option: "WriteOptions.MicroDVD.FrameRate", Msg: "MicroDVD output needs the frame rate of the video."},
			want: "stbox: invalid argument: WriteOptions.MicroDVD.FrameRate: MicroDVD output needs the frame rate of the video.",
		},
		{
			name: "invalid argument error without an option",
			err:  &model.InvalidArgumentError{Msg: "The sync points must not be empty."},
			want: "stbox: invalid argument: The sync points must not be empty.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorsMatchTheirSentinel(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		model.ErrParse,
		model.ErrUnsupportedFormat,
		model.ErrUnknownFormat,
		model.ErrInvalidArgument,
		model.ErrCueNotFound,
		model.ErrUnwritable,
		model.ErrInvalidUTF8,
	}
	tests := []struct {
		name string
		err  error
		want []error
	}{
		{"parse error", &model.ParseError{Format: model.SubRip, Line: 1}, []error{model.ErrParse}},
		{"unwritable error", &model.UnwritableError{Format: model.SAMI, CueIndex: -1}, []error{model.ErrUnwritable}},
		{
			"unwritable error of invalid UTF-8",
			&model.UnwritableError{Format: model.JSON, CueIndex: 0, Err: model.ErrInvalidUTF8},
			[]error{model.ErrUnwritable, model.ErrInvalidUTF8},
		},
		{"invalid argument error", &model.InvalidArgumentError{Msg: "The sync points must not be empty."}, []error{model.ErrInvalidArgument}},
		{"unknown format", model.ErrUnknownFormat, []error{model.ErrUnknownFormat, model.ErrUnsupportedFormat}},
		{"unsupported format", model.ErrUnsupportedFormat, []error{model.ErrUnsupportedFormat}},
		{
			"wrapped cue not found",
			fmt.Errorf("%w: index %d, the subtitle has %d cues", model.ErrCueNotFound, 4, 2),
			[]error{model.ErrCueNotFound},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, err := range []error{tt.err, fmt.Errorf("read movie.srt: %w", tt.err)} {
				for _, s := range sentinels {
					assert.Equal(t, slices.Contains(tt.want, s), errors.Is(err, s), "errors.Is(%q, %q)", err, s)
				}
			}
		})
	}
}

func TestErrorsUnwrapTheirCause(t *testing.T) {
	t.Parallel()

	var syntaxErr *json.SyntaxError
	require.ErrorAs(t, json.Unmarshal([]byte("{"), &struct{}{}), &syntaxErr)

	tests := []struct {
		name string
		err  interface {
			error
			Unwrap() error
		}
		want *json.SyntaxError
	}{
		{"parse error", &model.ParseError{Format: model.JSON, Err: syntaxErr}, syntaxErr},
		{"unwritable error", &model.UnwritableError{Format: model.TTML, CueIndex: -1, Err: syntaxErr}, syntaxErr},
		{"parse error without a cause", &model.ParseError{Format: model.SubRip}, nil},
		{"unwritable error without a cause", &model.UnwritableError{Format: model.SAMI, CueIndex: -1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.want == nil {
				require.NoError(t, tt.err.Unwrap())
				return
			}
			assert.Same(t, tt.want, tt.err.Unwrap())

			var got *json.SyntaxError
			require.ErrorAs(t, fmt.Errorf("read movie.json: %w", tt.err), &got)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestParseErrorIsFoundThroughTwoWraps(t *testing.T) {
	t.Parallel()

	want := &model.ParseError{
		Format: model.SubRip,
		Line:   6,
		Msg:    `Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".`,
	}
	err := fmt.Errorf("convert: %w", fmt.Errorf("read movie.srt: %w", want))

	var got *model.ParseError
	require.ErrorAs(t, err, &got)
	assert.Same(t, want, got)
	assert.Equal(t, 6, got.Line)
	require.ErrorIs(t, err, model.ErrParse)
}

func ExampleParseError() {
	read := func() error {
		return &model.ParseError{
			Format: model.SubRip,
			Line:   6,
			Msg:    `Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".`,
		}
	}
	err := fmt.Errorf("read movie.srt: %w", read())

	var perr *model.ParseError
	if errors.As(err, &perr) {
		fmt.Println(errors.Is(err, model.ErrParse), perr.Format, perr.Line)
	}
	fmt.Println(err)
	// Output:
	// true srt 6
	// read movie.srt: srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".
}
