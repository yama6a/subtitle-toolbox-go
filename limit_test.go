package stbox_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
)

// TestCappedLastCueEndReadsBack reads a file whose last cue end the reader caps at 99999:59:59.999, writes it in
// every format that has a writer and a reader, and reads that output.
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
		// MPL2 and MPSub have no cue without an end, so their rows end just below the limit.
		{"LRC start near the limit", stbox.LRC, "[offset:-359999987000]\n[00:12.00]last\n"},
		{"MPL2 end at the limit", stbox.MPL2, "[3599999990][3599999999]last\n"},
		{"MPSub end at the limit", stbox.MPSub, "FORMAT=TIME\n\n359999999 0.999\nlast\n"},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			t.Parallel()

			opts := stbox.ReadOptions{Format: in.format, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}
			s, _, err := parseBytes([]byte(in.content), opts)
			require.NoError(t, err)
			require.Equal(t, 1, s.Len())
			if in.format != stbox.MPL2 && in.format != stbox.MPSub {
				c, _ := s.Cue(0)
				require.Equal(t, timecode.MaxValidTime, c.End)
			}

			written := 0
			for _, f := range stbox.Formats() {
				if !f.CanRead() {
					continue
				}
				var out bytes.Buffer
				_, err := stbox.Write(&out, s, f, stbox.WriteOptions{
					MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}},
					ITT:      itt.WriteOptions{FrameRate: stbox.FrameRate25},
				})
				if errors.Is(err, stbox.ErrUnsupportedFormat) {
					continue
				}
				require.NoError(t, err, f)
				written++

				text := out.String()
				back, _, err := parseBytes(out.Bytes(), stbox.ReadOptions{Format: f, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}})
				require.NoError(t, err, "%s: %q", f, text)
				require.Equal(t, 1, back.Len(), f)
				c, _ := back.Cue(0)
				assert.Less(t, c.End, timecode.MaxTime, f)
				// SubRip, LRC and EBU STL clamp at the largest time they hold: 999:59:59.999, 999:59.99 and 255 hours.
				// Cavena 890 caps a frame number at FFFFFFh, which is 186 hours at 25 fps.
				// PAC clamps at 654 hours, because its time code holds hours * 100 + minutes in 2 bytes.
				if f != stbox.SubRip && f != stbox.LRC && f != stbox.EBUSTL && f != stbox.PAC && f != stbox.Cavena890 {
					assert.GreaterOrEqual(t, c.End, timecode.MaxValidTime-time.Second, f)
				}
			}
			assert.Positive(t, written)
		})
	}
}
