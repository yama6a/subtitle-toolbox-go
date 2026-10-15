package stbox_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestParseUnsupportedFormat(t *testing.T) {
	t.Parallel()

	formats := []stbox.Format{"foo"}
	for _, f := range stbox.Formats() {
		if !f.CanRead() {
			formats = append(formats, f)
		}
	}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Parse(strings.NewReader("1\nHello\n"), stbox.ReadOptions{Format: f})
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			require.NotErrorIs(t, err, stbox.ErrUnknownFormat)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestParseNoFormatFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"cue number and text only", "1\nHello\n"},
		{"JSON object of no format", `{"segments": []}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Parse(strings.NewReader(tt.content), stbox.ReadOptions{})
			require.ErrorIs(t, err, stbox.ErrUnknownFormat)
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	s, info, err := stbox.Load(filepath.Join(t.TempDir(), "missing.srt"), stbox.ReadOptions{})
	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Nil(t, s)
	assert.Zero(t, info)
}

func TestWriteUnsupportedFormat(t *testing.T) {
	t.Parallel()

	formats := []stbox.Format{"foo", ""}
	for _, f := range stbox.Formats() {
		if !f.CanWrite() {
			formats = append(formats, f)
		}
	}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			err := stbox.Write(&out, &stbox.Subtitle{}, f, stbox.WriteOptions{})
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			assert.Zero(t, out.Len())
		})
	}
}

func TestZeroOptionsAreValid(t *testing.T) {
	t.Parallel()

	require.NoError(t, stbox.ReadOptions{}.Validate())
	require.NoError(t, stbox.WriteOptions{}.Validate())
}

func TestSentinelsAreTheModelSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		root, model error
	}{
		{stbox.ErrParse, model.ErrParse},
		{stbox.ErrUnsupportedFormat, model.ErrUnsupportedFormat},
		{stbox.ErrUnknownFormat, model.ErrUnknownFormat},
		{stbox.ErrInvalidArgument, model.ErrInvalidArgument},
		{stbox.ErrCueNotFound, model.ErrCueNotFound},
		{stbox.ErrUnwritable, model.ErrUnwritable},
	}
	for _, tt := range tests {
		t.Run(tt.root.Error(), func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, fmt.Errorf("read movie.srt: %w", tt.model), tt.root)
			require.ErrorIs(t, fmt.Errorf("read movie.srt: %w", tt.root), tt.model)
		})
	}
}

func ExampleFormatFromPath() {
	for _, path := range []string{"movie.SUB", "notes.txt", "clip.xml", "movie"} {
		f, ok := stbox.FormatFromPath(path)
		fmt.Printf("%s: %q %t\n", path, f, ok)
	}
	// Output:
	// movie.SUB: "microdvd" true
	// notes.txt: "txt" true
	// clip.xml: "ttml" true
	// movie: "" false
}

func ExampleFormats() {
	for _, f := range stbox.Formats() {
		fmt.Println(f, f.Extensions())
	}
	// Output:
	// ass [ass ssa]
	// csv [csv]
	// json [json]
	// lrc [lrc]
	// microdvd [sub]
	// mpsub [mpsub]
	// sami [smi sami]
	// sbv [sbv]
	// srt [srt]
	// subviewer [sub]
	// ttml [ttml dfxp xml]
	// tsv [tsv]
	// txt [txt]
	// mpl2 [txt]
	// tmplayer [txt]
	// qttext [txt]
	// vtt [vtt]
}
