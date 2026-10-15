package model_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

type cueSpec struct {
	Start, End float64
	Text       string
}

func subtitleOf(specs ...cueSpec) *model.Subtitle {
	cues := make([]model.Cue, len(specs))
	for i, c := range specs {
		cues[i] = model.Cue{Start: sec(c.Start), End: sec(c.End)}
		cues[i].SetText(c.Text)
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func describe(s *model.Subtitle) []cueSpec {
	out := []cueSpec{}
	for _, c := range s.All() {
		out = append(out, cueSpec{Start: c.Start.Seconds(), End: c.End.Seconds(), Text: c.Text()})
	}
	return out
}

func withComments(tb testing.TB, s *model.Subtitle, comments ...model.Comment) *model.Subtitle {
	tb.Helper()

	for _, c := range comments {
		require.NoError(tb, s.AddComment(c.Text, c.BeforeCue))
	}
	return s
}

// commentsByCueText returns each comment text with the text of the cue it comes before, or "" after the last cue.
func commentsByCueText(s *model.Subtitle) [][2]string {
	comments := s.Comments()
	if len(comments) == 0 {
		return nil
	}
	out := make([][2]string, 0, len(comments))
	for _, c := range comments {
		cue, _ := s.Cue(c.BeforeCue)
		out = append(out, [2]string{c.Text, cue.Text()})
	}
	return out
}

func setCue(tb testing.TB, s *model.Subtitle, i int, change func(*model.Cue)) {
	tb.Helper()

	c, ok := s.Cue(i)
	require.True(tb, ok, i)
	change(&c)
	require.NoError(tb, s.SetCue(i, c))
}

func vttLine(tb testing.TB, value string) func(*model.Cue) {
	tb.Helper()

	return func(c *model.Cue) {
		require.NoError(tb, c.SetFormatData("vtt", model.NewData(model.Field{Key: "line", Value: value})))
	}
}

func TestCueEditingFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want string
		edit     func(*model.Subtitle) (*model.Subtitle, error)
	}{
		{
			name: "append the second film part at 52:10",
			in:   "editing/film_part1.srt", want: "editing/film_merged.srt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) {
				return s, s.Append(readFixture(t, "editing/film_part2.srt"), 3130*time.Second)
			},
		},
		{
			name: "remove the cue that HLS repeats across a segment boundary",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_deduplicated.vtt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) { return s, s.RemoveDuplicateCues(0) },
		},
		{
			name: "remove exact, overlapping and touching duplicates",
			in:   "editing/own_duplicate_cues.srt", want: "editing/own_duplicate_cues_deduplicated.srt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) { return s, s.RemoveDuplicateCues(0) },
		},
		{
			name: "split a cue of 2 lines",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_split.vtt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) { return s, s.SplitCue(2, 9500*time.Millisecond, 1) },
		},
		{
			name: "join 2 cues of one sentence",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_joined.vtt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) { return s, s.JoinCues(3, 4) },
		},
		{
			name: "slice and move to zero",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_slice.vtt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) { return s.Slice(6*time.Second, 16*time.Second, true) },
		},
		{
			name: "a time edit moves the cue and its comment",
			in:   "editing/harbour_tour.vtt", want: "editing/harbour_tour_start_edited.vtt",
			edit: func(s *model.Subtitle) (*model.Subtitle, error) {
				c, _ := s.Cue(3)
				c.Start, c.End = 19*time.Second, 20*time.Second
				return s, s.SetCue(3, c)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFixture(t, tt.in)
			got, err := tt.edit(s)
			require.NoError(t, err)
			want := readFixture(t, tt.want)
			assert.Equal(t, timesAndLines(want), timesAndLines(got))
			assert.Equal(t, want.Comments(), got.Comments())
		})
	}
}

func TestAppendKeepsMetadataCommentsAndFormatDataOfThisOnConflict(t *testing.T) {
	t.Parallel()

	s := withComments(t, subtitleOf(cueSpec{1, 2, "one"}, cueSpec{3, 4, "two"}),
		model.Comment{Text: "before two", BeforeCue: 1}, model.Comment{Text: "end of part one", BeforeCue: 2})
	s.SetMetadata(model.MetadataTitle, "Part one")
	require.NoError(t, s.SetFormatData("vtt", model.NewData(model.Field{Key: "header", Value: "first"})))
	other := withComments(t, subtitleOf(cueSpec{1, 2, "three"}),
		model.Comment{Text: "start of part two", BeforeCue: 0}, model.Comment{Text: "end of part two", BeforeCue: 1})
	other.SetMetadata(model.MetadataTitle, "Part two")
	other.SetMetadata(model.MetadataLanguage, "en")
	require.NoError(t, other.SetFormatData("vtt", model.NewData(model.Field{Key: "header", Value: "second"})))
	require.NoError(t, other.SetFormatData("ass", model.NewData(model.Field{Key: "scriptInfo", Value: []any{"Title: two"}})))

	require.NoError(t, s.Append(other, 10*time.Second))

	assert.Equal(t, []cueSpec{{1, 2, "one"}, {3, 4, "two"}, {11, 12, "three"}}, describe(s))
	assert.Equal(t, map[string]string{"title": "Part one", "language": "en"}, s.AllMetadata())
	vtt, _ := s.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "header", Value: "first"}), vtt)
	ass, _ := s.FormatData("ass")
	assert.Equal(t, model.NewData(model.Field{Key: "scriptInfo", Value: []any{"Title: two"}}), ass)
	assert.Equal(t, []model.Comment{
		{Text: "before two", BeforeCue: 1},
		{Text: "end of part one", BeforeCue: 2},
		{Text: "start of part two", BeforeCue: 2},
		{Text: "end of part two", BeforeCue: 3},
	}, s.Comments())
	assert.Equal(t, []cueSpec{{1, 2, "three"}}, describe(other))
	assert.Len(t, other.Comments(), 2)
}

func TestAppend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		s, other     *model.Subtitle
		offset       time.Duration
		want         []cueSpec
		wantComments [][2]string
	}{
		{
			name:         "sorts overlapping cues and keeps the comments with their cues",
			s:            withComments(t, subtitleOf(cueSpec{1, 2, "one"}, cueSpec{5, 6, "three"}), model.Comment{Text: "about three", BeforeCue: 1}),
			other:        withComments(t, subtitleOf(cueSpec{3, 4, "two"}), model.Comment{Text: "about two", BeforeCue: 0}),
			want:         []cueSpec{{1, 2, "one"}, {3, 4, "two"}, {5, 6, "three"}},
			wantComments: [][2]string{{"about two", "two"}, {"about three", "three"}},
		},
		{
			name:   "clamps negative times to 0",
			s:      &model.Subtitle{},
			other:  subtitleOf(cueSpec{1, 3, "one"}),
			offset: -2 * time.Second,
			want:   []cueSpec{{0, 1, "one"}},
		},
		{
			name: "moves word timestamps and rounds to milliseconds",
			s:    &model.Subtitle{},
			other: func() *model.Subtitle {
				s := &model.Subtitle{}
				s.AddCue(model.Cue{Start: sec(1), End: sec(3), Lines: []string{"<00:00:01.500>one <00:00:02.000>two"}})
				return s
			}(),
			offset: 1500*time.Millisecond + 500*time.Microsecond,
			want:   []cueSpec{{2.501, 4.501, "<00:00:03.001>one <00:00:03.501>two"}},
		},
		{
			name:         "a comment after the last cue stays at the end without copied cues",
			s:            withComments(t, subtitleOf(cueSpec{1, 2, "one"}), model.Comment{Text: "end", BeforeCue: 1}),
			other:        withComments(t, &model.Subtitle{}, model.Comment{Text: "empty other", BeforeCue: 0}),
			want:         []cueSpec{{1, 2, "one"}},
			wantComments: [][2]string{{"end", ""}, {"empty other", ""}},
		},
		{
			name:         "a comment after the last cue anchors to the first copied cue",
			s:            withComments(t, subtitleOf(cueSpec{5, 6, "late"}), model.Comment{Text: "end of part one", BeforeCue: 1}),
			other:        subtitleOf(cueSpec{1, 2, "early"}),
			want:         []cueSpec{{1, 2, "early"}, {5, 6, "late"}},
			wantComments: [][2]string{{"end of part one", "early"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, tt.s.Append(tt.other, tt.offset))
			assert.Equal(t, tt.want, describe(tt.s))
			assert.Equal(t, tt.wantComments, commentsByCueText(tt.s))
		})
	}
}

func TestAppendToItself(t *testing.T) {
	t.Parallel()

	s := withComments(t, subtitleOf(cueSpec{1, 2, "one"}), model.Comment{Text: "about one", BeforeCue: 0})

	require.NoError(t, s.Append(s, 10*time.Second))

	assert.Equal(t, []cueSpec{{1, 2, "one"}, {11, 12, "one"}}, describe(s))
	assert.Equal(t, []model.Comment{{Text: "about one", BeforeCue: 0}, {Text: "about one", BeforeCue: 1}}, s.Comments())
}

func TestAppendCopiesTheCueFields(t *testing.T) {
	t.Parallel()

	other := subtitleOf(cueSpec{1, 2, "one"})
	setCue(t, other, 0, func(c *model.Cue) {
		c.ID, c.Alignment, c.Forced = "intro", model.TopCenter, true
		vttLine(t, "0")(c)
	})
	s := &model.Subtitle{}

	require.NoError(t, s.Append(other, time.Second))

	got, _ := s.Cue(0)
	want, _ := other.Cue(0)
	want.Start, want.End = sec(2), sec(3)
	assert.Equal(t, want, got)
}

func TestSliceCutsCuesAtTheBoundsAndCopiesMetadataAndFormatData(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 3, "one"}, cueSpec{4, 6, "two"}, cueSpec{7, 9, "three"})
	s.SetMetadata(model.MetadataTitle, "Clip")
	require.NoError(t, s.SetFormatData("vtt", model.NewData(model.Field{Key: "header", Value: "clip"})))
	setCue(t, s, 1, func(c *model.Cue) {
		c.ID = "middle"
		vttLine(t, "0")(c)
	})
	before := s.Clone()

	slice, err := s.Slice(2*time.Second, 8*time.Second, false)
	require.NoError(t, err)

	assert.Equal(t, []cueSpec{{2, 3, "one"}, {4, 6, "two"}, {7, 8, "three"}}, describe(slice))
	assert.Equal(t, map[string]string{"title": "Clip"}, slice.AllMetadata())
	header, _ := slice.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "header", Value: "clip"}), header)
	middle, _ := slice.Cue(1)
	line, _ := middle.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "line", Value: "0"}), line)
	assert.Equal(t, "middle", middle.ID)
	assert.Equal(t, before.Cues(), s.Cues())
}

func TestSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		s            *model.Subtitle
		from, to     time.Duration
		moveToZero   bool
		want         []cueSpec
		wantComments []model.Comment
	}{
		{
			name: "drops cues that only touch the range",
			s:    subtitleOf(cueSpec{1, 2, "before"}, cueSpec{2, 3, "inside"}, cueSpec{3, 4, "after"}),
			from: 2 * time.Second, to: 3 * time.Second,
			want: []cueSpec{{2, 3, "inside"}},
		},
		{
			name: "an empty range gives no cues",
			s:    subtitleOf(cueSpec{1, 2, "before"}, cueSpec{2, 3, "after"}),
			from: 2 * time.Second, to: 2 * time.Second,
			want: []cueSpec{},
		},
		{
			name: "an empty range keeps a cue across it with a length of 0",
			s:    subtitleOf(cueSpec{1, 3, "across"}),
			from: 2 * time.Second, to: 2 * time.Second,
			want: []cueSpec{{2, 2, "across"}},
		},
		{
			name: "keeps a comment after the last cue when the last cue stays",
			s: withComments(t, subtitleOf(cueSpec{1, 2, "one"}, cueSpec{3, 4, "two"}),
				model.Comment{Text: "first", BeforeCue: 0}, model.Comment{Text: "last", BeforeCue: 2}),
			from: 2500 * time.Millisecond, to: 10 * time.Second, moveToZero: true,
			want:         []cueSpec{{0.5, 1.5, "two"}},
			wantComments: []model.Comment{{Text: "last", BeforeCue: 1}},
		},
		{
			name: "drops a comment after the last cue when the last cue drops",
			s: withComments(t, subtitleOf(cueSpec{1, 2, "one"}, cueSpec{3, 4, "two"}),
				model.Comment{Text: "first", BeforeCue: 0}, model.Comment{Text: "last", BeforeCue: 2}),
			from: 0, to: 2500 * time.Millisecond,
			want:         []cueSpec{{1, 2, "one"}},
			wantComments: []model.Comment{{Text: "first", BeforeCue: 0}},
		},
		{
			name: "moves word timestamps to zero and clamps them at 0",
			s:    subtitleOf(cueSpec{1, 4, "<00:00:01.000>a <00:00:02.500>b <00:00:03.500>c"}),
			from: 2 * time.Second, to: 3 * time.Second, moveToZero: true,
			want: []cueSpec{{0, 1, "<00:00:00.000>a <00:00:00.500>b <00:00:01.500>c"}},
		},
		{
			name: "keeps word timestamps without a move, also outside the cut cue",
			s:    subtitleOf(cueSpec{1, 4, "<00:00:01.000>a <00:00:03.500>c"}),
			from: 2 * time.Second, to: 3 * time.Second,
			want: []cueSpec{{2, 3, "<00:00:01.000>a <00:00:03.500>c"}},
		},
		{
			name: "rounds the bounds to milliseconds",
			s:    subtitleOf(cueSpec{1, 4, "one"}),
			from: 2*time.Second + 500*time.Microsecond, to: 3*time.Second + 400*time.Microsecond,
			want: []cueSpec{{2.001, 3, "one"}},
		},
		{
			name: "an empty subtitle drops its comments",
			s:    withComments(t, &model.Subtitle{}, model.Comment{Text: "alone", BeforeCue: 0}),
			from: 0, to: time.Second,
			want: []cueSpec{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := tt.s.Clone()
			got, err := tt.s.Slice(tt.from, tt.to, tt.moveToZero)
			require.NoError(t, err)
			assert.Equal(t, tt.want, describe(got))
			assert.Equal(t, tt.wantComments, got.Comments())
			assert.Equal(t, before.Cues(), tt.s.Cues())
			assert.Equal(t, before.Comments(), tt.s.Comments())
		})
	}
}

func TestSliceToZeroEqualsSliceThenShift(t *testing.T) {
	t.Parallel()

	s := readFixture(t, "editing/own_ferry_drift.vtt")
	from, to := 6*time.Second, 610*time.Second

	moved, err := s.Slice(from, to, true)
	require.NoError(t, err)
	shifted, err := s.Slice(from, to, false)
	require.NoError(t, err)
	require.NoError(t, shifted.Shift(-from))

	assert.Equal(t, shifted.Cues(), moved.Cues())
	assert.Equal(t, shifted.Comments(), moved.Comments())
}

// srtWithForcedSecondCue builds the cues of ForcedCuesTest::SRT of subtitle-toolbox-php with the second cue forced.
func srtWithForcedSecondCue(t *testing.T) *model.Subtitle {
	t.Helper()

	s := subtitleOf(cueSpec{1, 2, "Hello"}, cueSpec{3, 4, "EXIT"}, cueSpec{5, 6, "Bye"})
	setCue(t, s, 1, func(c *model.Cue) { c.Forced = true })
	return s
}

func TestForcedCuesOnlyReturnsACopyWithTheForcedCues(t *testing.T) {
	t.Parallel()

	s := withComments(t, srtWithForcedSecondCue(t),
		model.Comment{Text: "before cue 1", BeforeCue: 0},
		model.Comment{Text: "before cue 2", BeforeCue: 1},
		model.Comment{Text: "at the end", BeforeCue: 3},
	)
	s.SetMetadata(model.MetadataLanguage, "en")

	forced := s.ForcedCuesOnly()

	want, _ := s.Cue(1)
	assert.Equal(t, []model.Cue{want}, forced.Cues())
	assert.Equal(t, []model.Comment{{Text: "before cue 2", BeforeCue: 0}}, forced.Comments())
	assert.Equal(t, map[string]string{"language": "en"}, forced.AllMetadata())
	assert.Equal(t, 3, s.Len())
	assert.Len(t, s.Comments(), 3)
	assert.Equal(t, 0, (&model.Subtitle{}).ForcedCuesOnly().Len())
}

func TestForcedCuesOnlyKeepsTheLastCommentWhenTheLastCueIsForced(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 2, "Hello"}, cueSpec{3, 4, "EXIT"}, cueSpec{5, 6, "Bye"})
	setCue(t, s, 2, func(c *model.Cue) { c.Forced = true })
	withComments(t, s, model.Comment{Text: "at the end", BeforeCue: 3})

	assert.Equal(t, []model.Comment{{Text: "at the end", BeforeCue: 1}}, s.ForcedCuesOnly().Comments())
}

func TestSplitCueKeepsCommentsAndCueDataAndClearsTheSecondID(t *testing.T) {
	t.Parallel()

	s := withComments(t, subtitleOf(cueSpec{0, 6, "First sentence.\nSecond sentence."}, cueSpec{7, 8, "next"}),
		model.Comment{Text: "about the split cue", BeforeCue: 0}, model.Comment{Text: "about next", BeforeCue: 1})
	setCue(t, s, 0, func(c *model.Cue) {
		c.ID, c.Alignment, c.Forced = "intro", model.TopCenter, true
		vttLine(t, "0")(c)
	})

	require.NoError(t, s.SplitCue(0, 3*time.Second, 1))

	assert.Equal(t, []cueSpec{{0, 3, "First sentence."}, {3, 6, "Second sentence."}, {7, 8, "next"}}, describe(s))
	first, _ := s.Cue(0)
	second, _ := s.Cue(1)
	assert.Equal(t, []string{"intro", ""}, []string{first.ID, second.ID})
	assert.Equal(t, []model.Alignment{model.TopCenter, model.TopCenter}, []model.Alignment{first.Alignment, second.Alignment})
	assert.True(t, second.Forced)
	line, _ := second.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "line", Value: "0"}), line)
	assert.Equal(t, [][2]string{{"about the split cue", "First sentence."}, {"about next", "next"}}, commentsByCueText(s))
}

func TestSplitCue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		s              *model.Subtitle
		index          int
		at             time.Duration
		splitAfterLine int
		want           []cueSpec
		wantComments   [][2]string
	}{
		{
			name: "the second part takes its sorted place before a cue with the same start",
			s: withComments(t, subtitleOf(cueSpec{0, 6, "a\nb"}, cueSpec{1, 2, "early"}, cueSpec{3, 4, "same"}, cueSpec{5, 6, "late"}),
				model.Comment{Text: "about same", BeforeCue: 2}, model.Comment{Text: "end", BeforeCue: 4}),
			index: 0, at: 3 * time.Second, splitAfterLine: 1,
			want:         []cueSpec{{0, 3, "a"}, {1, 2, "early"}, {3, 6, "b"}, {3, 4, "same"}, {5, 6, "late"}},
			wantComments: [][2]string{{"about same", "same"}, {"end", ""}},
		},
		{
			name:  "word timestamps do not move and do not split",
			s:     subtitleOf(cueSpec{1, 4, "<00:00:01.000>a\n<00:00:03.000>b <00:00:03.500>c"}),
			index: 0, at: 2 * time.Second, splitAfterLine: 1,
			want: []cueSpec{{1, 2, "<00:00:01.000>a"}, {2, 4, "<00:00:03.000>b <00:00:03.500>c"}},
		},
		{
			name:  "keeps the first lines up to the line",
			s:     subtitleOf(cueSpec{1, 4, "a\nb\nc"}),
			index: 0, at: 2 * time.Second, splitAfterLine: 2,
			want: []cueSpec{{1, 2, "a\nb"}, {2, 4, "c"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, tt.s.SplitCue(tt.index, tt.at, tt.splitAfterLine))
			assert.Equal(t, tt.want, describe(tt.s))
			assert.Equal(t, tt.wantComments, commentsByCueText(tt.s))
		})
	}
}

func TestJoinCuesKeepsCommentsAndTheFirstCueData(t *testing.T) {
	t.Parallel()

	s := withComments(t,
		subtitleOf(cueSpec{0, 1, "zero"}, cueSpec{1, 2, "one"}, cueSpec{2, 4, "two"}, cueSpec{5, 6, "three"}, cueSpec{7, 8, "four"}),
		model.Comment{Text: "about one", BeforeCue: 1},
		model.Comment{Text: "about two", BeforeCue: 2},
		model.Comment{Text: "about three", BeforeCue: 3},
		model.Comment{Text: "about four", BeforeCue: 4},
		model.Comment{Text: "end", BeforeCue: 5},
	)
	setCue(t, s, 1, func(c *model.Cue) { c.ID, c.Alignment = "first", model.TopLeft })
	setCue(t, s, 2, func(c *model.Cue) { c.Alignment, c.Forced = model.TopRight, true })

	require.NoError(t, s.JoinCues(1, 3))

	assert.Equal(t, []cueSpec{{0, 1, "zero"}, {1, 6, "one\ntwo\nthree"}, {7, 8, "four"}}, describe(s))
	joined, _ := s.Cue(1)
	assert.Equal(t, "first", joined.ID)
	assert.Equal(t, model.TopLeft, joined.Alignment)
	assert.False(t, joined.Forced)
	assert.Equal(t, [][2]string{
		{"about one", "one\ntwo\nthree"},
		{"about two", "one\ntwo\nthree"},
		{"about three", "one\ntwo\nthree"},
		{"about four", "four"},
		{"end", ""},
	}, commentsByCueText(s))
}

func TestJoinCuesTakesTheLatestEnd(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 9, "long"}, cueSpec{2, 3, "short"}, cueSpec{4, 5, "next"})

	require.NoError(t, s.JoinCues(0, 1))

	assert.Equal(t, []cueSpec{{1, 9, "long\nshort"}, {4, 5, "next"}}, describe(s))
}

func TestRemoveDuplicateCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		in     []cueSpec
		maxGap time.Duration
		want   []cueSpec
	}{
		{"exact", []cueSpec{{1, 3, "Hello"}, {1, 3, "Hello"}}, 0, []cueSpec{{1, 3, "Hello"}}},
		{"overlapping", []cueSpec{{2, 4, "Bye"}, {2.5, 4, "Bye"}}, 0, []cueSpec{{2, 4, "Bye"}}},
		{"touching", []cueSpec{{1, 2, "Hi"}, {2, 3, "Hi"}}, 0, []cueSpec{{1, 3, "Hi"}}},
		{"gap within the maximum gap", []cueSpec{{1, 2, "Hi"}, {2.2, 3, "Hi"}}, 500 * time.Millisecond, []cueSpec{{1, 3, "Hi"}}},
		{"gap of exactly the maximum gap", []cueSpec{{1, 2, "Hi"}, {2.2, 3, "Hi"}}, 200 * time.Millisecond, []cueSpec{{1, 3, "Hi"}}},
		{"maximum gap rounded to milliseconds", []cueSpec{{1, 2, "Hi"}, {2.2, 3, "Hi"}}, 199500 * time.Microsecond, []cueSpec{{1, 3, "Hi"}}},
		{"gap above the maximum gap", []cueSpec{{1, 2, "Hi"}, {2.2, 3, "Hi"}}, 0, []cueSpec{{1, 2, "Hi"}, {2.2, 3, "Hi"}}},
		{"not neighbours", []cueSpec{{1, 2, "Hi"}, {2, 3, "Yes"}, {3, 4, "Hi"}}, 0, []cueSpec{{1, 2, "Hi"}, {2, 3, "Yes"}, {3, 4, "Hi"}}},
		{"earlier start later", []cueSpec{{2, 4, "Bye"}, {1, 3, "Bye"}}, 0, []cueSpec{{1, 4, "Bye"}}},
		{"contained", []cueSpec{{1, 5, "Bye"}, {2, 3, "Bye"}, {4.5, 6, "Bye"}}, 0, []cueSpec{{1, 6, "Bye"}}},
		{"tags count", []cueSpec{{1, 2, "<i>Hi</i>"}, {2, 3, "Hi"}}, 0, []cueSpec{{1, 2, "<i>Hi</i>"}, {2, 3, "Hi"}}},
		{"case counts", []cueSpec{{4, 5, "other"}, {5, 6, "Other"}}, 0, []cueSpec{{4, 5, "other"}, {5, 6, "Other"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in...)
			require.NoError(t, s.RemoveDuplicateCues(tt.maxGap))
			assert.Equal(t, tt.want, describe(s))
		})
	}
}

func TestRemoveDuplicateCuesJoinsOnlyTouchingCuesWithTheSameText(t *testing.T) {
	t.Parallel()

	s := withComments(t, subtitleOf(
		cueSpec{0, 1, "same"}, cueSpec{1, 2, "same"}, cueSpec{2, 3, "same"}, cueSpec{3.5, 4, "same"}, cueSpec{4, 5, "other"}, cueSpec{5, 6, "Other"},
	),
		model.Comment{Text: "second", BeforeCue: 1},
		model.Comment{Text: "third", BeforeCue: 2},
		model.Comment{Text: "fourth", BeforeCue: 3},
		model.Comment{Text: "end", BeforeCue: 6},
	)

	require.NoError(t, s.RemoveDuplicateCues(0))

	assert.Equal(t, []cueSpec{{0, 3, "same"}, {3.5, 4, "same"}, {4, 5, "other"}, {5, 6, "Other"}}, describe(s))
	assert.Equal(t, [][2]string{{"second", "same"}, {"third", "same"}, {"fourth", "same"}, {"end", ""}}, commentsByCueText(s))
	assert.Equal(t, []model.Comment{
		{Text: "second", BeforeCue: 0}, {Text: "third", BeforeCue: 0}, {Text: "fourth", BeforeCue: 1}, {Text: "end", BeforeCue: 4},
	}, s.Comments())
}

func TestRemoveDuplicateCuesKeepsTheFirstCueOfARun(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 3, "Hello"}, cueSpec{1, 4, "Hello"})
	setCue(t, s, 0, func(c *model.Cue) { c.ID = "first" })
	setCue(t, s, 1, func(c *model.Cue) { c.ID = "second" })

	require.NoError(t, s.RemoveDuplicateCues(0))

	c, _ := s.Cue(0)
	assert.Equal(t, model.Cue{Start: sec(1), End: sec(4), Lines: []string{"Hello"}, ID: "first"}, c)
}

func TestRemoveDuplicateCuesKeepsSameTextCuesThatCannotJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		first func(*model.Cue)
		other func(*model.Cue)
	}{
		{name: "other alignment", other: func(c *model.Cue) { c.Alignment = model.TopCenter }},
		{name: "forced", other: func(c *model.Cue) { c.Forced = true }},
		{name: "other format data", other: vttLine(t, "0")},
		{name: "other format data value", first: vttLine(t, "1"), other: vttLine(t, "0")},
		{name: "other format data key", first: vttLine(t, "0"), other: func(c *model.Cue) {
			vttLine(t, "0")(c)
			require.NoError(t, c.SetFormatData("srt", model.NewData(model.Field{Key: "coordinates", Value: model.NewData(
				model.Field{Key: "x1", Value: int64(1)}, model.Field{Key: "x2", Value: int64(2)},
				model.Field{Key: "y1", Value: int64(3)}, model.Field{Key: "y2", Value: int64(4)},
			)})))
		}},
		{
			name: "ASS style Glow and Default",
			first: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("ass", model.NewData(model.Field{Key: "fields", Value: model.NewData(model.Field{Key: "Style", Value: "Glow"})})))
			},
			other: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("ass", model.NewData(model.Field{Key: "fields", Value: model.NewData(model.Field{Key: "Style", Value: "Default"})})))
			},
		},
		{
			name: "other value type",
			first: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "n", Value: int64(1)})))
			},
			other: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "n", Value: 1.0})))
			},
		},
		{
			name: "other key order",
			first: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "a", Value: "1"}, model.Field{Key: "b", Value: "2"})))
			},
			other: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "b", Value: "2"}, model.Field{Key: "a", Value: "1"})))
			},
		},
		{
			name: "other list",
			first: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "l", Value: []any{"a", "b"}})))
			},
			other: func(c *model.Cue) {
				require.NoError(t, c.SetFormatData("x-tool", model.NewData(model.Field{Key: "l", Value: []any{"a"}})))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(cueSpec{1, 3, "Hello"}, cueSpec{1, 3, "Hello"})
			if tt.first != nil {
				setCue(t, s, 0, tt.first)
			}
			setCue(t, s, 1, tt.other)

			require.NoError(t, s.RemoveDuplicateCues(0))
			assert.Equal(t, 2, s.Len())
		})
	}
}

func TestRemoveDuplicateCuesJoinsEqualFormatData(t *testing.T) {
	t.Parallel()

	data := func(c *model.Cue) {
		require.NoError(t, c.SetFormatData("x-tool", model.NewData(
			model.Field{Key: "list", Value: []any{nil, true, int64(1), 1.5, "a", []any{"b"}, model.NewData(model.Field{Key: "c", Value: "d"})}},
		)))
		require.NoError(t, c.SetFormatData("ass", model.NewData(model.Field{Key: "fields", Value: model.NewData(model.Field{Key: "Style", Value: "Glow"})})))
	}
	s := subtitleOf(cueSpec{1, 3, "Hello"}, cueSpec{1, 3, "Hello"})
	setCue(t, s, 0, data)
	setCue(t, s, 1, data)

	require.NoError(t, s.RemoveDuplicateCues(0))

	assert.Equal(t, 1, s.Len())
}

func TestRemoveDuplicateCuesTreatsAnAlignmentOutsideTheGridAsBottomCenter(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 3, "Hello"}, cueSpec{1, 3, "Hello"})
	setCue(t, s, 1, func(c *model.Cue) { c.Alignment = model.Alignment(42) })

	require.NoError(t, s.RemoveDuplicateCues(0))

	assert.Equal(t, []cueSpec{{1, 3, "Hello"}}, describe(s))
}

func TestMergeSameTimeCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        []cueSpec
		tolerance time.Duration
		want      []cueSpec
	}{
		{"same times", []cueSpec{{1, 3, "- Where are you going?"}, {1, 3, "- Home."}}, 0, []cueSpec{{1, 3, "- Where are you going?\n- Home."}}},
		{"within the tolerance", []cueSpec{{1, 3, "A"}, {1.02, 2.99, "B"}}, 50 * time.Millisecond, []cueSpec{{1, 3, "A\nB"}}},
		{"tolerance rounded to milliseconds", []cueSpec{{1, 3, "A"}, {1.02, 2.99, "B"}}, 19500 * time.Microsecond, []cueSpec{{1, 3, "A\nB"}}},
		{"outside the tolerance", []cueSpec{{1, 3, "A"}, {1.02, 2.99, "B"}}, 0, []cueSpec{{1, 3, "A"}, {1.02, 2.99, "B"}}},
		{"other end", []cueSpec{{1, 3, "A"}, {1, 4, "B"}}, 0, []cueSpec{{1, 3, "A"}, {1, 4, "B"}}},
		{"same text", []cueSpec{{1, 3, "A"}, {1, 3, "A"}}, 0, []cueSpec{{1, 3, "A"}}},
		{"three cues", []cueSpec{{1, 3, "A"}, {1, 3, "B"}, {1, 3, "A"}, {4, 5, "C"}}, 0, []cueSpec{{1, 3, "A\nB"}, {4, 5, "C"}}},
		{"multi-line cues", []cueSpec{{1, 3, "A\nB"}, {1, 3, "C\nD"}}, 0, []cueSpec{{1, 3, "A\nB\nC\nD"}}},
		{"latest end", []cueSpec{{1, 3, "A"}, {1.01, 3.01, "B"}}, 10 * time.Millisecond, []cueSpec{{1, 3.01, "A\nB"}}},
		{
			"the first cue of the group is the reference",
			[]cueSpec{{1, 3, "A"}, {1.04, 3.04, "B"}, {1.08, 3.08, "C"}},
			50 * time.Millisecond,
			[]cueSpec{{1, 3.04, "A\nB"}, {1.08, 3.08, "C"}},
		},
		{"other speakers", []cueSpec{{1, 3, "<v Anna>Hi"}, {1, 3, "<v Ben>Hello"}}, 0, []cueSpec{{1, 3, "<v Anna>Hi\n<v Ben>Hello"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in...)
			require.NoError(t, s.MergeSameTimeCues(tt.tolerance))
			assert.Equal(t, tt.want, describe(s))
		})
	}
}

func TestMergeSameTimeCuesMovesCommentsToTheJoinedCue(t *testing.T) {
	t.Parallel()

	s := withComments(t, subtitleOf(cueSpec{1, 3, "A"}, cueSpec{1, 3, "B"}, cueSpec{4, 5, "C"}),
		model.Comment{Text: "first", BeforeCue: 0},
		model.Comment{Text: "second", BeforeCue: 1},
		model.Comment{Text: "third", BeforeCue: 2},
	)

	require.NoError(t, s.MergeSameTimeCues(0))

	assert.Equal(t, []cueSpec{{1, 3, "A\nB"}, {4, 5, "C"}}, describe(s))
	assert.Equal(t, [][2]string{{"first", "A\nB"}, {"second", "A\nB"}, {"third", "C"}}, commentsByCueText(s))
}

func TestMergeSameTimeCuesKeepsCuesWithAnotherAlignmentOrForcedFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		other func(*model.Cue)
	}{
		{"other alignment", func(c *model.Cue) { c.Alignment = model.TopCenter }},
		{"forced", func(c *model.Cue) { c.Forced = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(cueSpec{1, 3, "A"}, cueSpec{1, 3, "B"})
			setCue(t, s, 1, tt.other)
			require.NoError(t, s.MergeSameTimeCues(0))
			assert.Equal(t, 2, s.Len())
		})
	}
}

func TestMergeSameTimeCuesJoinsCuesWithOtherFormatData(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{1, 3, "A"}, cueSpec{1, 3, "B"})
	setCue(t, s, 0, func(c *model.Cue) {
		c.ID = "anna"
		vttLine(t, "0")(c)
	})
	setCue(t, s, 1, vttLine(t, "1"))

	require.NoError(t, s.MergeSameTimeCues(0))

	c, _ := s.Cue(0)
	assert.Equal(t, []string{"A", "B"}, c.Lines)
	assert.Equal(t, "anna", c.ID)
	line, _ := c.FormatData("vtt")
	assert.Equal(t, model.NewData(model.Field{Key: "line", Value: "0"}), line)
}

func TestCueEditingErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		edit     func(*model.Subtitle) error
		sentinel error
		want     string
	}{
		{
			name: "append beyond the time limit",
			edit: func(s *model.Subtitle) error {
				return s.Append(readFixture(t, "editing/film_part2.srt"), 100000*time.Hour)
			},
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: The retimed cue 1 would end at 360000004 s. Cue times must be below 100000 hours.",
		},
		{
			name: "append with the last cue beyond the time limit",
			edit: func(s *model.Subtitle) error {
				return s.Append(readFixture(t, "editing/film_part2.srt"), 100000*time.Hour-100*time.Second)
			},
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: The retimed cue 3 would end at 360000663 s. Cue times must be below 100000 hours.",
		},
		{
			name: "slice with the start after the end",
			edit: func(s *model.Subtitle) error {
				_, err := s.Slice(5*time.Second, 4*time.Second, false)
				return err //nolint:wrapcheck // the test compares the error that Slice returns
			},
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: The slice start 5 must not be after the slice end 4.",
		},
		{
			name:     "split cue 9",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(9, 3*time.Second, 1) },
			sentinel: model.ErrCueNotFound,
			want:     "stbox: cue not found: Cannot split cue 9: the cue does not exist.",
		},
		{
			name:     "split cue -1",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(-1, 3*time.Second, 1) },
			sentinel: model.ErrCueNotFound,
			want:     "stbox: cue not found: Cannot split cue -1: the cue does not exist.",
		},
		{
			name:     "split at the cue end",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(2, 12500*time.Millisecond, 1) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot split cue 2 at 12.5: the time must be after the cue start 6.5 and before the cue end 12.5.",
		},
		{
			name:     "split at the cue start",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(2, 6500*time.Millisecond, 1) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot split cue 2 at 6.5: the time must be after the cue start 6.5 and before the cue end 12.5.",
		},
		{
			name:     "split after the last line",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(2, 9*time.Second, 2) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot split cue 2 after line 2: the cue has 2 lines.",
		},
		{
			name:     "split after line 0",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(2, 9*time.Second, 0) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot split cue 2 after line 0: the cue has 2 lines.",
		},
		{
			name:     "split a cue of 1 line",
			edit:     func(s *model.Subtitle) error { return s.SplitCue(0, 2*time.Second, 1) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot split cue 0 after line 1: the cue has 1 lines.",
		},
		{
			name:     "join cues 0 to 9",
			edit:     func(s *model.Subtitle) error { return s.JoinCues(0, 9) },
			sentinel: model.ErrCueNotFound,
			want:     "stbox: cue not found: Cannot join cue 9: the cue does not exist.",
		},
		{
			name:     "join cues -1 to 2",
			edit:     func(s *model.Subtitle) error { return s.JoinCues(-1, 2) },
			sentinel: model.ErrCueNotFound,
			want:     "stbox: cue not found: Cannot join cue -1: the cue does not exist.",
		},
		{
			name:     "join cues 9 to 2",
			edit:     func(s *model.Subtitle) error { return s.JoinCues(9, 2) },
			sentinel: model.ErrCueNotFound,
			want:     "stbox: cue not found: Cannot join cue 9: the cue does not exist.",
		},
		{
			name:     "join cues 1 to 1",
			edit:     func(s *model.Subtitle) error { return s.JoinCues(1, 1) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot join cues 1 to 1: the first index must be lower than the last index.",
		},
		{
			name:     "join cues 2 to 1",
			edit:     func(s *model.Subtitle) error { return s.JoinCues(2, 1) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: Cannot join cues 2 to 1: the first index must be lower than the last index.",
		},
		{
			name:     "remove duplicates with a negative maximum gap",
			edit:     func(s *model.Subtitle) error { return s.RemoveDuplicateCues(-100 * time.Millisecond) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: The maximum gap must be 0 or more seconds, got -0.1.",
		},
		{
			name:     "merge same-time cues with a negative tolerance",
			edit:     func(s *model.Subtitle) error { return s.MergeSameTimeCues(-100 * time.Millisecond) },
			sentinel: model.ErrInvalidArgument,
			want:     "stbox: invalid argument: The time tolerance must be 0 or more seconds, got -0.1.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readFixture(t, "editing/harbour_tour.vtt")
			before := s.Clone()

			err := tt.edit(s)

			require.ErrorIs(t, err, tt.sentinel)
			require.EqualError(t, err, tt.want)
			assert.Equal(t, before.Cues(), s.Cues())
			assert.Equal(t, before.Comments(), s.Comments())
			assert.Equal(t, before.AllMetadata(), s.AllMetadata())
		})
	}
}

// pairs builds n cues in pairs with the same times and a comment before each, as tests/CueListSpeedTest.php of
// subtitle-toolbox-php does. Cue i gets the text of linesOf(i).
func pairs(tb testing.TB, n int, linesOf func(i int) string) *model.Subtitle {
	tb.Helper()

	cues := make([]model.Cue, n)
	for i := range cues {
		start := time.Duration(i/2*2) * time.Second
		cues[i] = model.Cue{Start: start, End: start + time.Second, Lines: []string{linesOf(i)}}
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	for i := range n {
		require.NoError(tb, s.AddComment(fmt.Sprintf("note %d", i), i))
	}
	return s
}

func duplicatePairs(tb testing.TB, n int) *model.Subtitle {
	tb.Helper()

	return pairs(tb, n, func(i int) string { return fmt.Sprintf("text%d", i/2) })
}

func sameTimePairs(tb testing.TB, n int) *model.Subtitle {
	tb.Helper()

	return pairs(tb, n, func(i int) string { return fmt.Sprintf("line%d", i) })
}

func commentIndexes(s *model.Subtitle) []int {
	comments := s.Comments()
	out := make([]int, 0, len(comments))
	for _, c := range comments {
		out = append(out, c.BeforeCue)
	}
	return out
}

// TestJoinPairsOfCues holds the result checks of testRemoveDuplicateCuesAndMergeSameTimeCuesJoinTwentyThousandCuesUnderFiveSeconds
// in tests/CueListSpeedTest.php of subtitle-toolbox-php, at 200 cues.
func TestJoinPairsOfCues(t *testing.T) {
	t.Parallel()

	duplicates := duplicatePairs(t, 200)
	sameTimes := sameTimePairs(t, 200)

	require.NoError(t, duplicates.RemoveDuplicateCues(0))
	require.NoError(t, sameTimes.MergeSameTimeCues(0))

	assert.Equal(t, []string{"text0", "text1"}, texts(duplicates)[:2])
	first, _ := sameTimes.Cue(0)
	assert.Equal(t, []string{"line0", "line1"}, first.Lines)
	for _, s := range []*model.Subtitle{duplicates, sameTimes} {
		assert.Equal(t, 100, s.Len())
		assert.Equal(t, []int{0, 0, 1, 1}, commentIndexes(s)[:4])
		assert.Equal(t, 99, commentIndexes(s)[199])
	}
}
