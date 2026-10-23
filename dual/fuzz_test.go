package dual_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const maxFuzzCues = 64

// fuzzSubtitle builds a subtitle from data: 4 bytes make a cue. The first byte moves the start by tenths of a
// second, the second sets the length, the third picks the text and the fourth sets the forced flag and the alignment.
func fuzzSubtitle(data []byte) *model.Subtitle {
	var s model.Subtitle
	var start time.Duration
	for i := 0; i+4 <= len(data) && s.Len() < maxFuzzCues; i += 4 {
		start += time.Duration(data[i]) * 100 * time.Millisecond
		length := time.Duration(data[i+1]) * 100 * time.Millisecond
		from := min(int(data[i+2]), len(data))
		to := min(from+int(data[i+3]&0x0f), len(data))
		s.AddCue(model.Cue{
			Start: start, End: start + length, Lines: []string{string(data[from:to])},
			Forced: data[i+3]&0x80 != 0, Alignment: model.Alignment(data[i+3] >> 4 & 0x07),
		})
	}
	return &s
}

func FuzzFromPair(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("abcdefghijklmnop"), []byte("abcdefghijklmnoq"))
	f.Add([]byte{1, 20, 0, 5, 2, 20, 3, 5, 3, 20, 6, 5}, []byte{1, 20, 0, 5, 9, 20, 3, 5})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, primaryData, secondaryData []byte) {
		primary, secondary := fuzzSubtitle(primaryData), fuzzSubtitle(secondaryData)
		primaryBefore, secondaryBefore := primary.Clone(), secondary.Clone()
		for _, opts := range []dual.Options{
			{SecondaryStyle: "i"},
			{Mode: dual.ModeTopBottom, SecondaryStyle: `font color="#ffff00"`},
		} {
			result, err := dual.FromPair(primary, secondary, opts)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, result.Len(), primary.Len())
			assert.LessOrEqual(t, result.Len(), primary.Len()+secondary.Len())
			var last time.Duration
			for i, c := range result.All() {
				assert.GreaterOrEqual(t, c.Start, last, "cue %d", i)
				last = c.Start
			}
			assert.Equal(t, primaryBefore, primary)
			assert.Equal(t, secondaryBefore, secondary)
		}
	})
}
