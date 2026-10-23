package stbox_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

// TestCappedLastCueEndReadsBack reads a file whose last cue end the reader caps at 99999:59:59.999, writes it in
// every format that has a writer, and reads that output.
func TestCappedLastCueEndReadsBack(t *testing.T) {
	t.Parallel()

	inputs := []struct {
		name    string
		format  stbox.Format
		content string
	}{
		{"SAMI start at the limit", stbox.SAMI, "<SAMI><BODY><SYNC Start=359999999999><P>last</BODY></SAMI>"},
		{"SAMI start 3 s before the limit", stbox.SAMI, "<SAMI><BODY><SYNC Start=359999997000><P>last</BODY></SAMI>"},
		{"TMPlayer start at 99999:59:59", stbox.TMPlayer, "99999:59:59:last\n"},
		{"TMPlayer start 3 s before the limit", stbox.TMPlayer, "99999:59:57:last\n"},
		// MPL2 has no cue without an end, so this row ends just below the limit.
		{"MPL2 end at the limit", stbox.MPL2, "[3599999990][3599999999]last\n"},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			t.Parallel()

			opts := stbox.ReadOptions{Format: in.format, MicroDVD: microdvd.ReadOptions{FrameRate: 25}}
			s, _, err := stbox.Parse(strings.NewReader(in.content), opts)
			require.NoError(t, err)
			require.Equal(t, 1, s.Len())
			if in.format != stbox.MPL2 {
				c, _ := s.Cue(0)
				require.Equal(t, timecode.MaxValidTime, c.End)
			}

			written := 0
			for _, f := range stbox.Formats() {
				var out bytes.Buffer
				err := stbox.Write(&out, s, f, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 25}})
				if errors.Is(err, stbox.ErrUnsupportedFormat) {
					continue
				}
				require.NoError(t, err, f)
				written++

				text := out.String()
				back, _, err := stbox.Parse(&out, stbox.ReadOptions{Format: f, MicroDVD: microdvd.ReadOptions{FrameRate: 25}})
				require.NoError(t, err, "%s: %q", f, text)
				require.Equal(t, 1, back.Len(), f)
				c, _ := back.Cue(0)
				assert.Less(t, c.End, timecode.MaxTime, f)
				if f != stbox.SubRip {
					assert.GreaterOrEqual(t, c.End, timecode.MaxValidTime-time.Second, f)
				}
			}
			assert.Positive(t, written)
		})
	}
}
