package timing_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/timing"
)

func FuzzShotChanges(f *testing.F) {
	f.Add([]byte("12.5\n00:01:02.500\n80.000\n"))
	f.Add([]byte("\xef\xbb\xbf1.5\r\n\r\n 2 \r\n0:00:01"))
	f.Add([]byte("frame:0 pts_time:8.5 pts_time:3 pts_time: 8.5\nn:1 pts_time:99999999999"))
	f.Add([]byte("1.5\n\n00:01:2x"))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, read := range []func([]byte) ([]time.Duration, error){timing.ShotChangesFromFFmpegLog, timing.ShotChangesFromText} {
			got, err := read(data)
			if err != nil {
				require.ErrorIs(t, err, model.ErrParse)
				continue
			}
			assert.True(t, slices.IsSorted(got))
			assert.Equal(t, got, slices.Compact(slices.Clone(got)))
		}
	})
}
