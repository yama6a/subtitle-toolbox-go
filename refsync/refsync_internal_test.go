package refsync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func fixtureSpans(tb testing.TB, name string) []span {
	tb.Helper()

	s, _, err := stbox.Load(testfiles.Path(tb, "sync/"+name), stbox.ReadOptions{})
	require.NoError(tb, err)
	return spans(s)
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	var s model.Subtitle
	s.AddCues(cues...)
	return &s
}

func sec(f float64) time.Duration {
	return time.Duration(f * float64(time.Second)).Round(time.Millisecond)
}

func cue(start, end float64, text string) model.Cue {
	return model.Cue{Start: sec(start), End: sec(end), Lines: []string{text}}
}

// Adapted from ReferenceSyncSplitTest::testApplyEndsTheEarlierCueOneMillisecondBeforeTheLaterPart and
// ReferenceSyncSplitTest::testApplySortsPartsThatSwapOrder.
func TestApplySegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cues      []model.Cue
		offsets   [2]float64
		wantTexts []string
		want      [][2]time.Duration
	}{
		{
			name:      "an earlier cue ends 1 ms before the later segment",
			cues:      []model.Cue{cue(10, 14, "one"), cue(20, 22, "two"), cue(25, 27, "three")},
			offsets:   [2]float64{5, -3},
			wantTexts: []string{"one", "two", "three"},
			want:      [][2]time.Duration{{sec(15), sec(16.999)}, {sec(17), sec(19)}, {sec(22), sec(24)}},
		},
		{
			name:      "segments that swap order swap their cues",
			cues:      []model.Cue{cue(10, 12, "one"), cue(20, 22, "two")},
			offsets:   [2]float64{15, -15},
			wantTexts: []string{"two", "one"},
			want:      [][2]time.Duration{{sec(5), sec(7)}, {sec(25), sec(27)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(tt.cues...)
			r := report(int64(tt.offsets[0]*1000), 1, 0.9, []segmentResult{
				{from: 0, offset: int64(tt.offsets[0] * 1000)}, {from: 20000, offset: int64(tt.offsets[1] * 1000)},
			})
			require.NoError(t, apply(s, r.Segments))
			var texts []string
			var got [][2]time.Duration
			for _, c := range s.All() {
				texts = append(texts, c.Text())
				got = append(got, [2]time.Duration{c.Start, c.End})
			}
			assert.Equal(t, tt.wantTexts, texts)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplyChecksTheTimeLimitFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		segments []segmentResult
		scale    float64
		msg      string
	}{
		{
			name:     "2 segments",
			segments: []segmentResult{{from: 0, offset: 1000}, {from: 359999000000, offset: 2000}},
			scale:    1,
			msg:      "The retimed cue 2 would end at 360000001.5 s. Cue times must be below 100000 hours.",
		},
		{
			name:     "a scale and then a shift",
			segments: []segmentResult{{offset: 60000}},
			scale:    1.000000001,
			msg:      "The retimed cue 2 would end at 360000059.86 s. Cue times must be below 100000 hours.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitle(cue(10, 12, "early"), model.Cue{
				Start: 359999999 * time.Second, End: 359999999500 * time.Millisecond, Lines: []string{"late"},
			})
			before := s.Cues()
			err := apply(s, report(tt.segments[0].offset, tt.scale, 1, tt.segments).Segments)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			require.EqualError(t, err, "stbox: invalid argument: "+tt.msg)
			assert.Equal(t, before, s.Cues())
		})
	}
}

func TestSpans(t *testing.T) {
	t.Parallel()

	s := subtitle(cue(3, 5, "c"), cue(1, 2, "a"), cue(2, 4, "b"), cue(7, 7, "empty"), cue(9, 8, "reversed"), cue(10, 11, "d"))
	assert.Equal(t, []span{{1, 5}, {10, 11}}, spans(s))
}

// Adapted from SpanOverlapTest::testOverlapPerOffsetMatchesOverlapAtEachGridOffset.
func TestOverlapPerOffsetMatchesTheSweep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		target, reference []span
		lo, hi            float64
	}{
		{"2 target spans, 1 reference span", []span{{0, 1}, {2, 4}}, []span{{0.5, 3}}, -5, 5},
		{
			"a long target span, touching reference spans, an offset range off the grid",
			[]span{{1, 9.5}},
			[]span{{0, 0.25}, {0.25, 2}, {7.75, 8}},
			-2.35, 1.15,
		},
		{
			"real files, German at 25 fps against English",
			fixtureSpans(t, "own_target_de_25fps.srt"), fixtureSpans(t, "own_reference_en.srt"), -60, 60,
		},
		{"one offset", []span{{0, 1}}, []span{{0.5, 3}}, 0.2, 0.2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perOffset := overlapPerOffset(tt.target, tt.reference, tt.lo, tt.hi)
			require.Len(t, perOffset, coarseCount(tt.lo, tt.hi)+1)
			for i, v := range perOffset {
				offset := tt.lo + float64(i)*coarseStep
				assert.InDelta(t, overlap(tt.target, tt.reference, offset), v, 1e-6, "offset %v", offset)
			}
		})
	}
}

// Adapted from SpanOverlapTest::testOverlapPerSpanAddsUpToOverlap.
func TestOverlapPerSpan(t *testing.T) {
	t.Parallel()

	target, reference := fixtureSpans(t, "own_target_de_25fps.srt"), fixtureSpans(t, "own_reference_en.srt")
	perSpan := overlapPerSpan(target, reference, 1.3)
	require.Len(t, perSpan, len(target))
	var sum float64
	for _, v := range perSpan {
		sum += v
	}
	assert.InDelta(t, overlap(target, reference, 1.3), sum, 1e-9)
	assert.Equal(t, []float64{0, 0}, overlapPerSpan([]span{{0, 1}, {5, 6}}, []span{{2, 3}}, 0))
	assert.InDelta(t, 3.5, totalTime([]span{{0, 1}, {2, 4.5}}), 0)
}
