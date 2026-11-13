package stbox_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func imageCue(start, end time.Duration, lines ...string) stbox.Cue {
	c := stbox.Cue{Start: start, End: end, Lines: lines}
	c.Image = &stbox.Image{PNG: []byte("\x89PNG"), X: 1, Y: 2, Width: 3, Height: 4, ScreenWidth: 720, ScreenHeight: 576}
	return c
}

func writeBytes(t *testing.T, s *stbox.Subtitle, f stbox.Format, opts stbox.WriteOptions) ([]byte, error) {
	t.Helper()

	var b bytes.Buffer
	_, err := stbox.Write(&b, s, f, opts)
	return b.Bytes(), err
}

func subtitleOfCues(t *testing.T, comment int, cues ...stbox.Cue) *stbox.Subtitle {
	t.Helper()

	s := &stbox.Subtitle{}
	s.AddCues(cues...)
	require.NoError(t, s.AddComment("note", comment))
	return s
}

func TestWritersHandleImageCues(t *testing.T) {
	t.Parallel()

	text := func(start, end time.Duration, line string) stbox.Cue {
		return stbox.Cue{Start: start, End: end, Lines: []string{line}}
	}
	one, two, three := text(time.Second, 2*time.Second, "One"), text(5*time.Second, 6*time.Second, "Two"), text(7*time.Second, 8*time.Second, "Three")
	withImages := subtitleOfCues(t, 1, one, imageCue(3*time.Second, 4*time.Second), two, imageCue(7*time.Second, 8*time.Second, "Three"))
	// The comment before the dropped image cue moves to the next kept cue.
	skipped := subtitleOfCues(t, 1, one, two, three)

	for _, f := range stbox.Formats() {
		if !f.CanWrite() || f == stbox.PGS {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			got, err := writeBytes(t, withImages, f, stbox.WriteOptions{})
			if f == stbox.JSON {
				require.NoError(t, err)
				assert.Contains(t, string(got), `"image"`)
				return
			}
			var uerr *stbox.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, 1, uerr.CueIndex)
			assert.Equal(t, f, uerr.Format)
			assert.Equal(t, "Cue #1 holds an image but no text, and "+string(f)+" needs text.", uerr.Reason)

			got, err = writeBytes(t, withImages, f, stbox.WriteOptions{SkipImageCues: true})
			want, wantErr := writeBytes(t, skipped, f, stbox.WriteOptions{})
			assert.Equal(t, wantErr, err)
			assert.Equal(t, want, got)
		})
	}
}
