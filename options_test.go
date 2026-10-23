package stbox_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
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
		{name: "MicroDVD frame rate", opts: stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: 23.976}}},
		{
			name: "negative MicroDVD frame rate",
			opts: stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: -1}},
			wantErr: &stbox.InvalidArgumentError{
				Option: "ReadOptions.MicroDVD.FrameRate",
				Msg:    "The frame rate must be greater than 0, got -1.",
			},
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
		{name: "MicroDVD frame rate", opts: stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 25, WriteFrameRateLine: true}}},
		{
			name:    "MicroDVD frame rate NaN",
			opts:    stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: math.NaN()}},
			wantErr: &stbox.InvalidArgumentError{Option: "WriteOptions.MicroDVD.FrameRate", Msg: "The MicroDVD frame rate must be greater than 0, got NAN."},
		},
		{name: "MPSub frame rate", opts: stbox.WriteOptions{MPSub: mpsub.WriteOptions{FrameRate: 25}}},
		{
			name:    "MPSub frame rate 29.97",
			opts:    stbox.WriteOptions{MPSub: mpsub.WriteOptions{FrameRate: 29.97}},
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
	_, _, parseErr := stbox.Parse(strings.NewReader(""), read)
	writeErr := stbox.Write(&bytes.Buffer{}, &stbox.Subtitle{}, "foo", stbox.WriteOptions{BOM: stbox.BOM(3)})
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
	plain, _, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip})
	require.NoError(t, err)
	with, _, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip, MicroDVD: microdvd.ReadOptions{FrameRate: 25}})
	require.NoError(t, err)
	assert.Equal(t, plain, with)

	_, _, err = stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip, MicroDVD: microdvd.ReadOptions{FrameRate: math.Inf(1)}})
	require.ErrorIs(t, err, stbox.ErrInvalidArgument, "Validate checks the options of every format")

	var a, b bytes.Buffer
	require.NoError(t, stbox.Write(&a, plain, stbox.SubRip, stbox.WriteOptions{}))
	require.NoError(t, stbox.Write(&b, plain, stbox.SubRip, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 25, WriteFrameRateLine: true}}))
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
