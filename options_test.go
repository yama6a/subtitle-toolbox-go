package stbox_test

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// Ports testANegativeLastCueDurationThrows of tests/ReadOptionsTest.php of subtitle-toolbox-php.
func TestReadOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    stbox.ReadOptions
		wantErr *stbox.InvalidArgumentError
	}{
		{name: "zero value", opts: stbox.ReadOptions{}},
		{name: "last cue duration 0", opts: stbox.ReadOptions{LastCueDuration: new(time.Duration(0))}},
		{name: "last cue duration 7 s", opts: stbox.ReadOptions{LastCueDuration: new(7 * time.Second)}},
		{name: "known encoding", opts: stbox.ReadOptions{Encoding: "Windows-1252"}},
		{name: "alias in lower case", opts: stbox.ReadOptions{Encoding: "sjis"}},
		{name: "MicroDVD frame rate", opts: stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 2997, Den: 125}}}},
		{
			name: "negative MicroDVD frame rate",
			opts: stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: -1, Den: 1}}},
			wantErr: &stbox.InvalidArgumentError{
				Option: "ReadOptions.MicroDVD.FrameRate",
				Msg:    "The frame rate must be greater than 0, got -1.",
			},
		},
		{
			name:    "MCC service 64",
			opts:    stbox.ReadOptions{MCC: mcc.ReadOptions{Service: 64}},
			wantErr: &stbox.InvalidArgumentError{Option: "ReadOptions.MCC.Service", Msg: "The CEA-708 service must be from 1 to 63, got 64."},
		},
		{
			name:    "SCC channel 3",
			opts:    stbox.ReadOptions{SCC: scc.ReadOptions{Channel: 3}},
			wantErr: &stbox.InvalidArgumentError{Option: "ReadOptions.SCC.Channel", Msg: "The SCC data channel must be 1 or 2, got 3."},
		},
		{
			name:    "unknown encoding",
			opts:    stbox.ReadOptions{Encoding: "CP1125"},
			wantErr: &stbox.InvalidArgumentError{Option: "ReadOptions.Encoding", Msg: `The encoding "CP1125" is unknown.`},
		},
		{
			name: "negative last cue duration",
			opts: stbox.ReadOptions{LastCueDuration: new(-time.Second)},
			wantErr: &stbox.InvalidArgumentError{
				Option: "ReadOptions.LastCueDuration",
				Msg:    "The last cue duration must be 0 or more, got -1s.",
			},
		},
		{
			name: "1 ms below 0",
			opts: stbox.ReadOptions{LastCueDuration: new(-time.Millisecond)},
			wantErr: &stbox.InvalidArgumentError{
				Option: "ReadOptions.LastCueDuration",
				Msg:    "The last cue duration must be 0 or more, got -1ms.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertInvalidArgument(t, tt.wantErr, tt.opts.Validate())
		})
	}
}

func TestWriteOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    stbox.WriteOptions
		wantErr *stbox.InvalidArgumentError
	}{
		{name: "zero value", opts: stbox.WriteOptions{}},
		{name: "every known value", opts: stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMRemove, StripTags: true}},
		{
			name:    "unknown line ending",
			opts:    stbox.WriteOptions{LineEnding: stbox.LineEnding(2)},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.LineEnding", Msg: "The line ending 2 is unknown."},
		},
		{
			name:    "unknown BOM rule",
			opts:    stbox.WriteOptions{BOM: stbox.BOM(3)},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.BOM", Msg: "The BOM rule 3 is unknown."},
		},
		{
			name: "MCC frame rate 48",
			opts: stbox.WriteOptions{MCC: mcc.WriteOptions{FrameRate: stbox.FrameRate{Num: 48, Den: 1}}},
			wantErr: &stbox.InvalidArgumentError{
				Option: "WriteOptions.MCC.FrameRate",
				Msg:    "The MCC formatter accepts the frame rates 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60, got 48.",
			},
		},
		{
			name:    "unknown SCC drop-frame mode",
			opts:    stbox.WriteOptions{SCC: scc.WriteOptions{DropFrame: stbox.DropFrameMode(3)}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.SCC.DropFrame", Msg: "The SCC drop-frame mode 3 is unknown."},
		},
		{name: "MicroDVD frame rate", opts: stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}, WriteFrameRateLine: true}}},
		{
			name:    "MicroDVD frame rate with a zero denominator",
			opts:    stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25}}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.MicroDVD.FrameRate", Msg: "The MicroDVD frame rate must be greater than 0, got INF."},
		},
		{name: "MPSub frame rate", opts: stbox.WriteOptions{MPSub: mpsub.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}},
		{
			name:    "MPSub frame rate 29.97",
			opts:    stbox.WriteOptions{MPSub: mpsub.WriteOptions{FrameRate: stbox.FrameRate{Num: 2997, Den: 100}}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.MPSub.FrameRate", Msg: "The MPSub frame rate must be a positive integer, got 29.97."},
		},
		{name: "plain text paragraph gap 0", opts: stbox.WriteOptions{PlainText: plaintext.WriteOptions{ParagraphGap: new(time.Duration(0))}}},
		{
			name:    "plain text paragraph gap -1 s",
			opts:    stbox.WriteOptions{PlainText: plaintext.WriteOptions{ParagraphGap: new(-time.Second)}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.PlainText.ParagraphGap", Msg: "The paragraph gap must be 0 or more seconds, got -1."},
		},
		{name: "SubViewer 1", opts: stbox.WriteOptions{SubViewer: subviewer.WriteOptions{Version: subviewer.V1}}},
		{
			name:    "SubViewer version 3",
			opts:    stbox.WriteOptions{SubViewer: subviewer.WriteOptions{Version: subviewer.Version(3)}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.SubViewer.Version", Msg: "The SubViewer version must be 1 or 2, got 3."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assertInvalidArgument(t, tt.wantErr, tt.opts.Validate())
		})
	}
}

func assertInvalidArgument(t *testing.T, want *stbox.InvalidArgumentError, err error) {
	t.Helper()

	if want == nil {
		require.NoError(t, err)
		return
	}
	require.ErrorIs(t, err, stbox.ErrInvalidArgument)
	var argErr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &argErr)
	assert.Equal(t, *want, *argErr)
}

func TestEntryPointsValidateBeforeTheyRead(t *testing.T) {
	t.Parallel()

	read := stbox.ReadOptions{Format: "foo", LastCueDuration: new(-time.Second)}
	_, _, loadErr := stbox.Load(filepath.Join(t.TempDir(), "missing.srt"), read)
	_, _, parseErr := parseBytes([]byte(""), read)
	_, writeErr := stbox.Write(&bytes.Buffer{}, &stbox.Subtitle{}, "foo", stbox.WriteOptions{BOM: stbox.BOM(3)})
	for _, err := range []error{loadErr, parseErr, writeErr} {
		require.ErrorIs(t, err, stbox.ErrInvalidArgument)
		require.NotErrorIs(t, err, stbox.ErrUnsupportedFormat)
	}
}

// Ports the case of the MicroDVD options of testEachParserIgnoresTheOptionsOfOtherFormats of tests/ReadOptionsTest.php
// of subtitle-toolbox-php.
func TestOtherFormatsIgnoreTheMicroDVDOptions(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "srt/real/own_styled.srt")
	plain, _, err := parseBytes(content, stbox.ReadOptions{Format: stbox.SubRip})
	require.NoError(t, err)
	with, _, err := parseBytes(content, stbox.ReadOptions{Format: stbox.SubRip, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}})
	require.NoError(t, err)
	assert.Equal(t, plain, with)

	_, _, err = parseBytes(content, stbox.ReadOptions{Format: stbox.SubRip, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25}}})
	require.ErrorIs(t, err, stbox.ErrInvalidArgument, "Validate checks the options of every format")

	var a, b bytes.Buffer
	_, err = stbox.Write(&a, plain, stbox.SubRip, stbox.WriteOptions{})
	require.NoError(t, err)
	_, err = stbox.Write(&b, plain, stbox.SubRip, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}, WriteFrameRateLine: true}})
	require.NoError(t, err)
	assert.Equal(t, a.String(), b.String())
}

func ExampleReadOptions_Validate() {
	err := stbox.ReadOptions{LastCueDuration: new(-time.Second)}.Validate()
	fmt.Println(errors.Is(err, stbox.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: ReadOptions.LastCueDuration: The last cue duration must be 0 or more, got -1s.
}
