package scc_test

import (
	"io"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// BenchmarkFitLongCJKCue ports testFitAnswersForALongCjkCueUnderATenthOfASecond of SccFormatterTest.php.
// The break search of the fit took over a second for this cue, which has a break point after almost every character.
func BenchmarkFitLongCJKCue(b *testing.B) {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 8 * time.Second, Lines: []string{string(testfiles.Read(b, "scc/fit/own_long_cjk.txt"))}})
	for b.Loop() {
		if _, err := scc.Write(io.Discard, s, model.CommonWriteOptions{}, scc.WriteOptions{Fit: true}); err == nil {
			b.Fatal("the cue fits")
		}
	}
}
