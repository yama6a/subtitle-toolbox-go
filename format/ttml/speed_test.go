package ttml_test

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// TestDuplicateCueIDsGrowth writes cues that all have the ID c1, which come out as c1, c1_2, c1_3 and so on.
func TestDuplicateCueIDsGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	best := func(n int) time.Duration {
		s := &model.Subtitle{}
		for i := range n {
			s.AddCue(model.Cue{Start: time.Duration(i) * time.Second, End: time.Duration(i)*time.Second + time.Second, Lines: []string{"x"}, ID: "c1"})
		}
		fastest := time.Duration(1<<63 - 1)
		for range 3 {
			begin := time.Now()
			require.NoError(t, ttml.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, ttml.WriteOptions{}))
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	small, large := best(2000), best(20000)
	ratio := float64(large) / float64(max(small, time.Nanosecond))
	assert.LessOrEqual(t, ratio, 30.0, "n=2000 took %v, 10n took %v", small, large)
}
