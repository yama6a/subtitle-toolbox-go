package diff_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/diff"
)

func BenchmarkCompare(b *testing.B) {
	b.Run("same-text/cues=3000", func(b *testing.B) {
		oldSub, newSub := musicCues(3000)
		b.ReportAllocs()
		for b.Loop() {
			_, err := diff.Compare(oldSub, newSub, diff.Options{})
			require.NoError(b, err)
		}
	})
	b.Run("long-cue-and-repeats/cues=5201", func(b *testing.B) {
		oldSub := repeatWithOverallCue(load(b, "own_original.srt"))
		newSub := repeatWithOverallCue(load(b, "own_edited.srt"))
		b.ReportAllocs()
		for b.Loop() {
			_, err := diff.Compare(oldSub, newSub, diff.Options{})
			require.NoError(b, err)
		}
	})
}
