package stbox_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

func TestUnsupportedFormatMessages(t *testing.T) {
	t.Parallel()

	type messages struct {
		format    stbox.Format
		wantRead  string
		wantWrite string
	}
	tests := make([]messages, 0, 2+len(stbox.Formats()))
	tests = append(tests,
		messages{"foo", `The format "foo" is unknown.`, `The format "foo" is unknown.`},
		messages{"", `The format "" is unknown.`, `The format "" is unknown.`},
	)
	for _, f := range stbox.Formats() {
		tt := messages{format: f}
		switch {
		case f.CanRead():
		case f.CanWrite():
			tt.wantRead = "The format " + string(f) + " can be written but not read."
		default:
			tt.wantRead = "The format " + string(f) + " cannot be read."
		}
		switch {
		case f.CanWrite():
		case f.CanRead():
			tt.wantWrite = "The format " + string(f) + " can be read but not written."
		default:
			tt.wantWrite = "The format " + string(f) + " cannot be written."
		}
		tests = append(tests, tt)
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			t.Parallel()

			if tt.wantRead != "" && tt.format != "" {
				_, _, err := stbox.Parse(strings.NewReader("1\nHello\n"), stbox.ReadOptions{Format: tt.format})
				require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
				require.EqualError(t, err, "stbox: unsupported format: "+tt.wantRead)
			}
			if tt.wantWrite != "" {
				err := stbox.Write(&bytes.Buffer{}, &stbox.Subtitle{}, tt.format, stbox.WriteOptions{})
				require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
				require.EqualError(t, err, "stbox: unsupported format: "+tt.wantWrite)
			}
		})
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tests := []struct {
		name string
		path string
	}{
		{"missing file", filepath.Join(dir, "missing.srt")},
		{"folder", dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(tt.path, stbox.ReadOptions{})
			var pathErr *fs.PathError
			require.ErrorAs(t, err, &pathErr)
			assert.Equal(t, tt.path, pathErr.Path)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestLoadUnsupportedFormat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("1\nHello\n"), 0o600))

	s, info, err := stbox.Load(path, stbox.ReadOptions{Format: stbox.PlainText})
	require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
	assert.Nil(t, s)
	assert.Zero(t, info)
}
