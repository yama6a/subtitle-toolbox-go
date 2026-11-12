package caption_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// pairs returns a frame for each hex word of the field 1 pairs, one frame after another from the frame start.
func pairs(t *testing.T, start int64, words string) []cea708.Frame {
	t.Helper()
	fields := strings.Fields(words)
	frames := make([]cea708.Frame, 0, len(fields))
	for i, w := range fields {
		v, err := strconv.ParseUint(w, 16, 16)
		require.NoError(t, err)
		frames = append(frames, cea708.Frame{
			Time:     model.FrameRate2997.Duration(start + int64(i)),
			Triplets: []cea708.Triplet{{Type: cea708.Field1, Data: [2]byte{byte(v >> 8), byte(v)}}},
		})
	}
	return frames
}

// service1 returns a frame at the time with one packet that holds one service 1 block.
func service1(at time.Duration, block ...byte) cea708.Frame {
	body := append([]byte{0, byte(1<<5 | len(block))}, block...)
	if len(body)%2 == 1 {
		body = append(body, 0)
	}
	body[0] = byte(len(body) / 2)
	f := cea708.Frame{Time: at}
	for i := 0; i < len(body); i += 2 {
		typ := byte(cea708.DTVCCData)
		if i == 0 {
			typ = cea708.DTVCCStart
		}
		f.Triplets = append(f.Triplets, cea708.Triplet{Type: typ, Data: [2]byte{body[i], body[i+1]}})
	}
	return f
}

func TestCaptionDataCues(t *testing.T) {
	t.Parallel()
	window1Top := []byte{0x99, 0x38, 0x0A, 0x0A, 0x00, 0x1F, 0x09, 'H', 'i'} // window 1, visible, top left, "Hi"
	hide := []byte{0x8A, 0x02}
	popOn := "9420 9420 9470 9470 c8e9 942f 942f"
	erase := "942c 942c"
	tests := []struct {
		name    string
		frames  func(t *testing.T) []cea708.Frame
		service int
		want    []string // start-end alignment text
	}{
		{
			name: "a CEA-708 service with cues wins over the CEA-608 pairs",
			frames: func(t *testing.T) []cea708.Frame {
				t.Helper()
				return append(pairs(t, 0, popOn), service1(time.Second, window1Top...), service1(2*time.Second, hide...))
			},
			service: 1,
			want:    []string{"1s-2s TopLeft Hi"},
		},
		{
			name: "the last CEA-708 cue lasts the last cue duration",
			frames: func(t *testing.T) []cea708.Frame {
				t.Helper()
				return []cea708.Frame{service1(time.Second, window1Top...)}
			},
			service: 1,
			want:    []string{"1s-6s TopLeft Hi"},
		},
		{
			name: "a service without cues falls back to the CEA-608 pairs",
			frames: func(t *testing.T) []cea708.Frame {
				t.Helper()
				return append(pairs(t, 0, popOn), service1(time.Second, window1Top...))
			},
			service: 2,
			want:    []string{"167ms-5.167s BottomCenter Hi"},
		},
		{
			name: "service 0 reads CEA-608 only",
			frames: func(t *testing.T) []cea708.Frame {
				t.Helper()
				return append(pairs(t, 0, popOn), service1(time.Second, window1Top...))
			},
			want: []string{"167ms-5.167s BottomCenter Hi"},
		},
		{
			name: "a null pair and a gap of 2 frames end a code line",
			frames: func(t *testing.T) []cea708.Frame {
				t.Helper()
				// Without the split at the null pair, the second caption would show at frame 12.
				first := pairs(t, 0, popOn+" 8080 "+strings.Replace(popOn, "c8e9", "c1c2", 1)+" "+erase)
				return append(append(first, pairs(t, 30, popOn)...), pairs(t, 40, erase)...)
			},
			want: []string{
				"167ms-434ms BottomCenter Hi",
				"434ms-501ms BottomCenter AB",
				"1.168s-1.335s BottomCenter Hi",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cues := caption.CaptionDataCues(tt.frames(t), tt.service, caption.CEA608Options{Channel: 1}, 5*time.Second)
			var got []string
			for _, c := range cues {
				assert.Nil(t, c.Position.Offset)
				got = append(got, fmt.Sprintf("%v-%v %v %s", c.Start, c.End, c.Position.Alignment, c.Text()))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCEA708Captions(t *testing.T) {
	t.Parallel()
	for point, a := range []model.Alignment{
		model.TopLeft, model.TopCenter, model.TopRight, model.MiddleLeft, model.MiddleCenter,
		model.MiddleRight, model.BottomLeft, model.BottomCenter, model.BottomRight,
	} {
		cue := model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"x"}}
		cue.Position.Alignment = a
		got := caption.CEA708Captions([]model.Cue{cue})
		require.Len(t, got, 1)
		assert.Equal(t, point, got[0].AnchorPoint, a.String())
	}
}
