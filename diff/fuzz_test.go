package diff_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/diff"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const maxFuzzCues = 64

// fuzzSubtitle builds a subtitle from data: 4 bytes make a cue, the first for the start, the second for the
// length, the third for the text and the fourth for the flags. The texts are short pieces of data.
func fuzzSubtitle(data []byte) *model.Subtitle {
	var s model.Subtitle
	var start time.Duration
	for i := 0; i+4 <= len(data) && s.Len() < maxFuzzCues; i += 4 {
		start += time.Duration(data[i]) * 100 * time.Millisecond
		length := time.Duration(data[i+1]) * 100 * time.Millisecond
		from := min(int(data[i+2]), len(data))
		to := min(from+int(data[i+3]&0x0f), len(data))
		s.AddCue(model.Cue{
			Start: start, End: start + length, Lines: []string{string(data[from:to])}, Forced: data[i+3]&0x80 != 0,
		})
	}
	return &s
}

func FuzzCompare(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("abcdefghijklmnop"), []byte("abcdefghijklmnoq"))
	f.Add([]byte{1, 20, 0, 5, 2, 20, 3, 5, 3, 20, 6, 5}, []byte{1, 20, 0, 5, 9, 20, 3, 5})
	f.Add([]byte("0000000000000000"), []byte("0000000000000000000000000000"))
	f.Fuzz(func(t *testing.T, oldData, newData []byte) {
		oldSub, newSub := fuzzSubtitle(oldData), fuzzSubtitle(newData)
		for _, opts := range []diff.Options{{}, {IgnoreFormatting: true, IgnoreWhitespace: true, TextOnly: true}} {
			differences, err := diff.Compare(oldSub, newSub, opts)
			require.NoError(t, err)

			seenOld := make(map[int]bool)
			seenNew := make(map[int]bool)
			for _, d := range differences {
				if d.OldIndex >= 0 {
					assert.False(t, seenOld[d.OldIndex], "old index %d twice", d.OldIndex)
					seenOld[d.OldIndex] = true
					assert.Less(t, d.OldIndex, oldSub.Len())
					assert.NotNil(t, d.Old)
				} else {
					assert.Nil(t, d.Old)
				}
				if d.NewIndex >= 0 {
					assert.False(t, seenNew[d.NewIndex], "new index %d twice", d.NewIndex)
					seenNew[d.NewIndex] = true
					assert.Less(t, d.NewIndex, newSub.Len())
					assert.NotNil(t, d.New)
				} else {
					assert.Nil(t, d.New)
				}
			}
			_ = diff.Text(differences)

			same, err := diff.Compare(oldSub, oldSub, opts)
			require.NoError(t, err)
			assert.Empty(t, same)
		}
	})
}
