package stbox_test

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
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

func ExampleReadOptions_Validate() {
	err := stbox.ReadOptions{LastCueDuration: new(-time.Second)}.Validate()
	fmt.Println(errors.Is(err, stbox.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: ReadOptions.LastCueDuration: The last cue duration must be 0 or more, got -1s.
}
