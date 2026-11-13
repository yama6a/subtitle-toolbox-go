package speakers_test

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/speakers"
)

// manyCues returns n cues of about 100 bytes with labels, voice tags and style tags.
func manyCues(n int) *model.Subtitle {
	var s model.Subtitle
	cues := make([]model.Cue, n)
	for i := range cues {
		start := time.Duration(i) * 3 * time.Second
		c := model.Cue{Start: start, End: start + 2*time.Second}
		c.SetText("JOHN: Is the ferry on time today?\n<v Anna>It left <i>ten minutes</i> early. <v Ben>Then we take the bus.")
		c.ExtractWordTags()
		cues[i] = c
	}
	s.AddCues(cues...)
	return &s
}

// oneLongCue returns one cue of about n bytes with a change of speaker every 40 bytes.
func oneLongCue(n int) *model.Subtitle {
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		b.WriteString("<v S" + strconv.Itoa(i%50) + "><i>Hello there, again.</i> ")
	}
	var s model.Subtitle
	s.AddCue(model.Cue{End: time.Second, Lines: []string{b.String()}})
	return &s
}

// unclosedCue returns one cue of about n bytes whose style tags never close, with a change of speaker every 15 bytes.
func unclosedCue(n int) *model.Subtitle {
	var s model.Subtitle
	s.AddCue(model.Cue{End: time.Second, Lines: []string{strings.Repeat("<i>x<v A>y<v B>", n/15+1)}})
	return &s
}

var applyRows = []struct {
	name string
	opts speakers.Options
}{
	{"from labels", speakers.Options{FromLabels: true}},
	{"rename", speakers.Options{Rename: map[string]string{"Anna": "Zoë"}}},
	{"prefix", speakers.Options{Style: speakers.StylePrefix}},
	{"dashes", speakers.Options{Style: speakers.StyleDialogueDashes}},
	{"colors", speakers.Options{Style: speakers.StyleColors}},
}

// BenchmarkApply runs each step on 10,000 cues of about 100 bytes, and on one cue of 1 MB.
func BenchmarkApply(b *testing.B) {
	for _, row := range applyRows {
		b.Run("many cues/"+row.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				s := manyCues(10000)
				b.StartTimer()
				_, err := speakers.Apply(s, row.opts)
				require.NoError(b, err)
			}
		})
	}
	b.Run("one cue of 1 MB/prefix", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			s := oneLongCue(1 << 20)
			b.StartTimer()
			_, err := speakers.Apply(s, speakers.Options{Style: speakers.StylePrefix})
			require.NoError(b, err)
		}
	})
	b.Run("one cue of 1 MB with unclosed tags/prefix", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			b.StopTimer()
			s := unclosedCue(1 << 20)
			b.StartTimer()
			_, err := speakers.Apply(s, speakers.Options{Style: speakers.StylePrefix})
			require.NoError(b, err)
		}
	})
	b.Run("one cue of 1 MB with unclosed tags/list", func(b *testing.B) {
		b.ReportAllocs()
		s := unclosedCue(1 << 20)
		for b.Loop() {
			speakers.List(s)
		}
	})
	b.Run("many cues/list", func(b *testing.B) {
		b.ReportAllocs()
		s := manyCues(10000)
		for b.Loop() {
			speakers.List(s)
		}
	})
}

// TestApplyGrowth times each step at n and 10n, best of 3 each, and fails above a ratio of 30.
func TestApplyGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	best := func(build func() *model.Subtitle, run func(*model.Subtitle)) time.Duration {
		fastest := time.Duration(math.MaxInt64)
		for range 3 {
			s := build()
			begin := time.Now()
			run(s)
			fastest = min(fastest, time.Since(begin))
		}
		return fastest
	}
	type growthRow struct {
		name         string
		build        func(n int) *model.Subtitle
		small, large int
		run          func(*model.Subtitle)
	}
	rows := make([]growthRow, 0, 4+2*len(applyRows))
	rows = append(rows,
		growthRow{"list", manyCues, 2000, 20000, func(s *model.Subtitle) { speakers.List(s) }},
		growthRow{"list of one long cue", oneLongCue, 20000, 200000, func(s *model.Subtitle) { speakers.List(s) }},
		growthRow{"list of one cue with unclosed tags", unclosedCue, 20000, 200000, func(s *model.Subtitle) { speakers.List(s) }},
		growthRow{"prefix of one cue with unclosed tags", unclosedCue, 20000, 200000, func(s *model.Subtitle) {
			_, err := speakers.Apply(s, speakers.Options{Style: speakers.StylePrefix})
			require.NoError(t, err)
		}})
	for _, row := range applyRows {
		run := func(s *model.Subtitle) {
			_, err := speakers.Apply(s, row.opts)
			require.NoError(t, err)
		}
		rows = append(rows,
			growthRow{row.name, manyCues, 2000, 20000, run},
			growthRow{row.name + " of one long cue", oneLongCue, 20000, 200000, run})
	}
	for _, row := range rows {
		small := best(func() *model.Subtitle { return row.build(row.small) }, row.run)
		large := best(func() *model.Subtitle { return row.build(row.large) }, row.run)
		ratio := float64(large) / float64(max(small, time.Nanosecond))
		assert.LessOrEqual(t, ratio, 30.0, "%s: %d took %v, %d took %v", row.name, row.small, small, row.large, large)
	}
}
