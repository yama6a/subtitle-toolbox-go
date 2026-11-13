package timing_test

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/timing"
)

func BenchmarkApply(b *testing.B) {
	const copies = 1820
	r := rate(b, 24)
	garden := readSubRip(b, "shot-changes/own_garden_24fps.srt").Cues()
	timed := readSubRip(b, "shot-changes/own_garden_24fps_timed.srt").Cues()
	log, err := timing.ShotChangesFromFFmpegLog(testfiles.Read(b, "shot-changes/own_ffmpeg_showinfo.log"))
	require.NoError(b, err)
	// Shots in the 18 s between 2 copies, 2 s or more away from any cue.
	gardenShots := append(slices.Clone(log), 93*time.Second, 95*time.Second, 97*time.Second, 99*time.Second, 101*time.Second, 103*time.Second)

	cues := make([]model.Cue, 0, copies*len(garden))
	want := make([]model.Cue, 0, copies*len(garden))
	shots := make([]time.Duration, 0, copies*len(gardenShots))
	for i := range copies {
		shift := time.Duration(i) * 100 * time.Second
		for j, c := range garden {
			c.Start, c.End = c.Start+shift, c.End+shift
			cues = append(cues, c)
			w := timed[j]
			w.Start, w.End = w.Start+shift, w.End+shift
			want = append(want, w)
		}
		for _, d := range gardenShots {
			shots = append(shots, d+shift)
		}
	}
	input := &model.Subtitle{}
	input.AddCues(cues...)
	opts := timing.Options{FrameRate: r, ShotChanges: shots}

	b.Run("cues="+strconv.Itoa(len(cues)), func(b *testing.B) {
		b.ReportAllocs()
		var report timing.Report
		var s *model.Subtitle
		for b.Loop() {
			b.StopTimer()
			s = input.Clone()
			b.StartTimer()
			report, err = timing.Apply(s, opts)
		}
		require.NoError(b, err)
		assert.Equal(b, timing.Report{MovedStarts: 3 * copies, MovedEnds: 5 * copies}, report)
		assert.Equal(b, want, s.Cues())
	})
}
