package cea708_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
)

const (
	rate = 30
	// A frame at 29.97 fps holds 18 triplets, 36 bytes. A packet header and a block header take 2.
	capacity = 34
	second   = time.Second
)

func frameOf(d time.Duration) int64 {
	return (int64(d)*rate + int64(second)/2) / int64(second)
}

// decodeBlocks feeds each block as a packet of service 1 to a decoder.
func decodeBlocks(t *testing.T, blocks []cea708.Block) []cea708.State {
	t.Helper()
	frames := make([]cea708.Frame, 0, len(blocks))
	for _, b := range blocks {
		body := append([]byte{0, byte(1<<5 | len(b.Data))}, b.Data...)
		if len(body)%2 == 1 {
			body = append(body, 0)
		}
		body[0] = byte(len(body) / 2)
		var triplets []cea708.Triplet
		for i := 0; i < len(body); i += 2 {
			typ := byte(cea708.DTVCCData)
			if i == 0 {
				typ = cea708.DTVCCStart
			}
			triplets = append(triplets, cea708.Triplet{Type: typ, Data: [2]byte{body[i], body[i+1]}})
		}
		frames = append(frames, cea708.Frame{Time: time.Duration(b.Frame) * second / rate, Triplets: triplets})
	}
	return cea708.NewDecoder(1).Decode(frames)
}

func TestBlocks(t *testing.T) {
	t.Parallel()
	long := strings.Repeat
	tests := []struct {
		name     string
		captions []cea708.Caption
		want     []string // see describe
	}{
		{
			name: "styles, special characters, anchor point and a character that CEA-708 lacks",
			captions: []cea708.Caption{
				{Start: second, End: 2 * second, Lines: []string{"<i>Caf\u00e9</i> at the <font color=\"#ffff00\">corner</font>", "\u2026 \u266a"}, AnchorPoint: 7},
				{Start: 2 * second, End: 3 * second, Lines: []string{"<u>Next</u> stop"}, AnchorPoint: 0},
				{Start: 4 * second, End: 5 * second, Lines: []string{"\u0150rs"}, AnchorPoint: 7},
			},
			want: []string{
				"1|7|<i>Caf\u00e9</i> at the <font color=\"#ffff00\">corner</font>/\u2026 \u266a",
				"2|0|<u>Next</u> stop",
				"3|-1|",
				"4|7|Ors",
				"5|-1|",
			},
		},
		{
			name: "a caption shows late when the frames before it are too few",
			captions: []cea708.Caption{
				{Start: 0, End: second, Lines: []string{long("A", 30)}, AnchorPoint: 7},
				{Start: 100 * time.Millisecond, End: 2 * second, Lines: []string{long("B", 32), long("C", 32), long("D", 32), long("E", 32)}, AnchorPoint: 7},
			},
			want: []string{
				"0.066666666|7|" + long("A", 30),
				"0.3|7|" + long("B", 32) + "/" + long("C", 32) + "/" + long("D", 32) + "/" + long("E", 32),
				"2|-1|",
			},
		},
		{
			name:     "a line over 42 characters wraps once",
			captions: []cea708.Caption{{Start: second, End: 2 * second, Lines: []string{"The coastal train to the old harbour leaves from platform two."}, AnchorPoint: 7}},
			want:     []string{"1|7|The coastal train to the old/harbour leaves from platform two.", "2|-1|"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, describe(decodeBlocks(t, cea708.Blocks(tt.captions, frameOf, capacity))))
		})
	}
}

func TestBlocksCapacity(t *testing.T) {
	t.Parallel()
	captions := []cea708.Caption{{Start: 500 * time.Millisecond, End: 2 * second, Lines: []string{strings.Repeat("x", 40), strings.Repeat("y", 40)}, AnchorPoint: 7}}
	blocks := cea708.Blocks(captions, frameOf, 14)
	for _, b := range blocks {
		assert.LessOrEqual(t, len(b.Data), 14)
	}
	var show cea708.Block
	for _, b := range blocks {
		if len(b.Data) >= 2 && b.Data[len(b.Data)-2] == 0x89 {
			show = b
		}
	}
	require.NotZero(t, show.Frame)
	assert.EqualValues(t, 15, show.Frame)
}

func TestBlocksCapacityFloor(t *testing.T) {
	t.Parallel()
	captions := []cea708.Caption{{Start: second, End: 2 * second, Lines: []string{"Hi"}, AnchorPoint: 7}}
	want := cea708.Blocks(captions, frameOf, 7)
	require.NotEmpty(t, want)
	for _, c := range []int{0, 6} {
		assert.Equal(t, want, cea708.Blocks(captions, frameOf, c), c)
	}
}
