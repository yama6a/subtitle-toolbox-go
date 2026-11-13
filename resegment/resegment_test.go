package resegment_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
)

var emDash = string(rune(0x2014))

const tensor = "The tensor operators are optimized heavily for Apple silicon CPUs. Depending on the computation " +
	"size, Arm Neon SIMD instrisics or CBLAS Accelerate framework routines are used."

func limits(cpl, lines int, minDuration time.Duration) model.CueLimits {
	return model.CueLimits{MaxCharactersPerLine: cpl, MaxLinesPerCue: lines, MinDuration: &minDuration}
}

func TestSplitLong(t *testing.T) {
	t.Parallel()

	const yes = "Yes. Then we walk along the river to the old mill and back."
	const gate = "We meet at the north gate. Then we walk to the lake."
	tests := []struct {
		name   string
		cue    model.Cue
		limits model.CueLimits
		want   []part
	}{
		{"fits", newCue(1, 3, "Short line\nthat fits."), model.CueLimits{}, []part{{1, 3, "Short line\nthat fits."}}},
		{"sentence and clause ends", newCue(0, 11.05, tensor), limits(42, 2, time.Second), []part{
			{0, 4.231, "The tensor operators are optimized\nheavily for Apple silicon CPUs."},
			{4.231, 6.441, "Depending on the computation size,"},
			{6.441, 11.05, "Arm Neon SIMD instrisics or CBLAS\nAccelerate framework routines are used."},
		}},
		{"sentence end first", newCue(0, 10, "One two three four five. Six seven eight nine ten"), limits(30, 1, 0), []part{
			{0, 5.102, "One two three four five."}, {5.102, 10, "Six seven eight nine ten"},
		}},
		{"comma", newCue(0, 10, "One two three four five six, seven eight nine ten eleven"), limits(30, 1, 0), []part{
			{0, 5.179, "One two three four five six,"}, {5.179, 10, "seven eight nine ten eleven"},
		}},
		{"middle space", newCue(0, 10, "One two three four five six seven eight nine ten eleven"), limits(30, 1, 0), []part{
			{0, 5.091, "One two three four five six"}, {5.091, 10, "seven eight nine ten eleven"},
		}},
		{"semicolon", newCue(0, 10, "Aaaa bbbb cccc dddd one; eeee ffff gggg hhhh iiii jjjj"), limits(30, 1, 0), []part{
			{0, 4.63, "Aaaa bbbb cccc dddd one;"}, {4.63, 10, "eeee ffff gggg hhhh iiii jjjj"},
		}},
		{"colon", newCue(0, 10, "Aaaa bbbb cccc dddd one: eeee ffff gggg hhhh iiii jjjj"), limits(30, 1, 0), []part{
			{0, 4.63, "Aaaa bbbb cccc dddd one:"}, {4.63, 10, "eeee ffff gggg hhhh iiii jjjj"},
		}},
		{"em dash", newCue(0, 10, "Aaaa bbbb cccc dddd one"+emDash+" eeee ffff gggg hhhh iiii jjjj"), limits(30, 1, 0), []part{
			{0, 4.63, "Aaaa bbbb cccc dddd one" + emDash}, {4.63, 10, "eeee ffff gggg hhhh iiii jjjj"},
		}},
		{"lone dash", newCue(0, 10, "Aaaa bbbb cccc dddd one - eeee ffff gggg hhhh iiii jjjj"), limits(30, 1, 0), []part{
			{0, 4.727, "Aaaa bbbb cccc dddd one -"}, {4.727, 10, "eeee ffff gggg hhhh iiii jjjj"},
		}},
		{"full stop before lower case", newCue(0, 10, "Bring a tool e.g. a hammer and nails for the roof of the shed"), limits(30, 1, 0), []part{
			{0, 5.082, "Bring a tool e.g. a hammer and"}, {5.082, 10, "nails for the roof of the shed"},
		}},
		{"repeat until fit", newCue(0, 9, "One. Two. Three. Four. Five. Six."), limits(10, 1, 0), []part{
			{0, 2.727, "One. Two."}, {2.727, 4.636, "Three."}, {4.636, 6.273, "Four."}, {6.273, 9, "Five. Six."},
		}},
		{"time by characters", newCue(10, 20, "<i>Aaaa bbbb.</i> Cccc dddd eeee ffff gggg."), model.CueLimits{MaxCharactersPerLine: 30, MaxLinesPerCue: 1}, []part{
			{10, 13.056, "<i>Aaaa bbbb.</i>"}, {13.056, 20, "Cccc dddd eeee ffff gggg."},
		}},
		{"min duration allows the sentence", newCue(0, 12, yes), model.CueLimits{MaxCharactersPerLine: 60, MaxLinesPerCue: 1, MaxDuration: 11 * time.Second}, []part{
			{0, 1.017, "Yes."}, {1.017, 12, "Then we walk along the river to the old mill and back."},
		}},
		{"min duration blocks the sentence", newCue(0, 6.5, yes), model.CueLimits{MaxCharactersPerLine: 60, MaxLinesPerCue: 1, MaxDuration: 6 * time.Second}, []part{
			{0, 3.085, "Yes. Then we walk along the"}, {3.085, 6.5, "river to the old mill and back."},
		}},
		{"min duration blocks every break", newCue(0, 1.5, yes), model.CueLimits{MaxCharactersPerLine: 30, MaxLinesPerCue: 1}, []part{{0, 1.5, yes}}},
		{"max duration equal", newCue(0, 8, gate), model.CueLimits{MaxDuration: 8 * time.Second}, []part{{0, 8, gate}}},
		{"max duration below", newCue(0, 8, gate), model.CueLimits{MaxDuration: 7 * time.Second}, []part{
			{0, 4.154, "We meet at the north gate."}, {4.154, 8, "Then we walk to the lake."},
		}},
		{"speed equal", newCue(0, 4, gate), model.CueLimits{MaxCharactersPerSecond: 13}, []part{{0, 4, gate}}},
		{"speed below", newCue(0, 4, gate), model.CueLimits{MaxCharactersPerSecond: 12}, []part{
			{0, 2.077, "We meet at the north gate."}, {2.077, 4, "Then we walk to the lake."},
		}},
		{"tags and speaker", newCue(0, 10, `<v Ann><i>We go <font color="#ff0000">now, but</font> slowly.</i> <b>Keep up.</b>`), limits(15, 1, 0), []part{
			{0, 3.548, `<v Ann><i>We go <font color="#ff0000">now,</font></i>`},
			{3.548, 7.419, `<v Ann><i><font color="#ff0000">but</font> slowly.</i>`},
			{7.419, 10, `<v Ann><b>Keep up.</b>`},
		}},
		{"words decide the time", newCue(0, 10, "<00:00:00.000>One <00:00:01.000>two <00:00:07.000>three. <00:00:08.000>Four"), limits(14, 1, 0), []part{
			{0, 7, "<00:00:00.000>One <00:00:01.000>two"}, {7, 8, "<00:00:07.000>three."}, {8, 10, "<00:00:08.000>Four"},
		}},
		{"one long word", newCue(0, 20, "Supercalifragilisticexpialidocious"), limits(10, 1, 0), []part{{0, 20, "Supercalifragilisticexpialidocious"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := build(tt.cue)
			report, err := resegment.SplitLong(s, resegment.Options{Limits: tt.limits})
			require.NoError(t, err)
			assert.Equal(t, tt.want, describe(s))
			assert.Equal(t, resegment.Report{CuesBefore: 1, CuesAfter: len(tt.want)}, report)
			for _, c := range s.All() {
				assert.NoError(t, c.ValidateWords())
			}
		})
	}
}

func TestSplitLongKeepsWordsApart(t *testing.T) {
	t.Parallel()

	c := newCue(0, 4, "<00:00:00.000>Aaaa <00:00:01.000>bbbb. <00:00:02.000>Cccc <00:00:03.000>dddd.")
	c.Words[1].End = 1500 * time.Millisecond
	s := build(c)
	_, err := resegment.SplitLong(s, resegment.Options{Limits: limits(10, 1, 0)})
	require.NoError(t, err)

	require.Equal(t, 2, s.Len())
	first, _ := s.Cue(0)
	second, _ := s.Cue(1)
	assert.Equal(t, c.Words[:2], first.Words)
	assert.Equal(t, c.Words[2:], second.Words)
}

func TestSplitLongCJK(t *testing.T) {
	t.Parallel()

	jp := cjkTexts(t)
	t.Run("punctuation", func(t *testing.T) {
		t.Parallel()

		s := build(newCue(0, 10, jp[0]))
		_, err := resegment.SplitLong(s, resegment.Options{Limits: limits(16, 1, time.Second)})
		require.NoError(t, err)
		var times [][2]float64
		for _, c := range s.All() {
			times = append(times, [2]float64{c.Start.Seconds(), c.End.Seconds()})
		}
		assert.Equal(t, [][2]float64{{0, 2.745}, {2.745, 5.49}, {5.49, 7.843}, {7.843, 10}}, times)
	})
	t.Run("words", func(t *testing.T) {
		t.Parallel()

		var c model.Cue
		c.Start, c.End = 0, 6*time.Second
		c.Lines = []string{strings.ReplaceAll(jp[1], "|", "")}
		for i, w := range strings.Split(jp[1], "|") {
			c.Words = append(c.Words, model.Word{Text: w, Start: secs([]float64{0, 1, 1.5, 2.5, 3.5, 4.5}[i]), End: secs([]float64{1, 1.5, 2.5, 3.5, 4.5, 6}[i])})
		}
		s := build(c)
		_, err := resegment.SplitLong(s, resegment.Options{Limits: limits(6, 1, time.Second)})
		require.NoError(t, err)
		require.Equal(t, 2, s.Len())
		first, _ := s.Cue(0)
		second, _ := s.Cue(1)
		assert.Equal(t, [2]time.Duration{0, 2500 * time.Millisecond}, [2]time.Duration{first.Start, first.End})
		assert.Equal(t, [2]time.Duration{2500 * time.Millisecond, 6 * time.Second}, [2]time.Duration{second.Start, second.End})
		assert.Len(t, first.Words, 3)
		assert.Len(t, second.Words, 3)
	})
}

func TestSplitLongRightToLeft(t *testing.T) {
	t.Parallel()

	s := readRTL(t)
	s.FixRightToLeft()
	before := s.Len()
	_, err := resegment.SplitLong(s, resegment.Options{Limits: limits(10, 1, 500*time.Millisecond)})
	require.NoError(t, err)

	require.Greater(t, s.Len(), before)
	want := strings.Split(strings.TrimSpace(string(testfiles.Read(t, "resegmenting/rtl_split_texts.txt"))), "\n")
	var got []string
	for i, c := range s.All() {
		if i >= 2 && i <= 4 {
			got = append(got, c.Text())
		}
	}
	assert.Equal(t, want, got)
}

func TestSplitLongKeepsIdentityAndComments(t *testing.T) {
	t.Parallel()

	long := newCue(1, 11.05, tensor)
	long.ID = "long"
	long.Position.Alignment = model.TopCenter
	long.Forced = true
	s := build(newCue(0, 1, "Before."), long, newCue(12, 13, "After."))
	require.NoError(t, s.AddComment("before long", 1))
	require.NoError(t, s.AddComment("before after", 2))

	report, err := resegment.SplitLong(s, resegment.Options{})
	require.NoError(t, err)

	assert.Equal(t, resegment.Report{CuesBefore: 3, CuesAfter: 5}, report)
	var ids []string
	for i, c := range s.All() {
		if i >= 1 && i <= 3 {
			ids = append(ids, c.ID)
			assert.Equal(t, model.TopCenter, c.Position.Alignment)
			assert.True(t, c.Forced)
		}
	}
	assert.Equal(t, []string{"long", "", ""}, ids)
	assert.Equal(t, []model.Comment{{Text: "before long", BeforeCue: 1}, {Text: "before after", BeforeCue: 4}}, s.Comments())
}

func TestSplitLongKeepsImageCues(t *testing.T) {
	t.Parallel()

	c := newCue(0, 20, tensor)
	c.Image = &model.Image{}
	s := build(c)
	report, err := resegment.SplitLong(s, resegment.Options{Limits: limits(10, 1, 0)})
	require.NoError(t, err)
	assert.Equal(t, resegment.Report{CuesBefore: 1, CuesAfter: 1}, report)
}

func TestByWords(t *testing.T) {
	t.Parallel()

	gap := 700 * time.Millisecond
	tests := []struct {
		name string
		cues []model.Cue
		opts resegment.Options
		want []part
	}{
		{"sentence across cues", []model.Cue{
			newCue(0, 2, "<00:00:00.000>One <00:00:00.500>two. <00:00:01.000>Three"),
			newCue(2, 4, "<00:00:02.000>four <00:00:03.000>five."),
		}, resegment.Options{}, []part{
			{0, 1, "<00:00:00.000>One <00:00:00.500>two."},
			{1, 4, "<00:00:01.000>Three <00:00:02.000>four <00:00:03.000>five."},
		}},
		{"gap of 0.6 s", []model.Cue{
			newCue(0, 2, "<00:00:00.000>One <00:00:01.000>two"), newCue(2.6, 4, "<00:00:02.600>three <00:00:03.000>four"),
		}, resegment.Options{}, []part{
			{0, 2, "<00:00:00.000>One <00:00:01.000>two"}, {2.6, 4, "<00:00:02.600>three <00:00:03.000>four"},
		}},
		{"gap of 0.7 s joins", []model.Cue{
			newCue(0, 2, "<00:00:00.000>One <00:00:01.000>two"), newCue(2.6, 4, "<00:00:02.600>three <00:00:03.000>four"),
		}, resegment.Options{MaxWordGap: &gap}, []part{
			{0, 4, "<00:00:00.000>One <00:00:01.000>two <00:00:02.600>three <00:00:03.000>four"},
		}},
		{"limit", []model.Cue{
			newCue(0, 4, "<00:00:00.000>Aaaa <00:00:01.000>bbbb <00:00:02.000>cccc <00:00:03.000>dddd"),
		}, resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: 10, MaxLinesPerCue: 1}}, []part{
			{0, 2, "<00:00:00.000>Aaaa <00:00:01.000>bbbb"}, {2, 4, "<00:00:02.000>cccc <00:00:03.000>dddd"},
		}},
		{"speakers and cues without words stay", []model.Cue{
			newCue(0, 1, "<v Ann><00:00:00.000>Hello <00:00:00.500>there"),
			newCue(1, 2, "<v Bob><00:00:01.000>Hi <00:00:01.500>Ann"),
			newCue(2, 3, "No timestamps"),
			newCue(3, 4, "<v Bob><00:00:03.000>Bye"),
		}, resegment.Options{}, []part{
			{0, 1, "<v Ann><00:00:00.000>Hello <00:00:00.500>there"},
			{1, 2, "<v Bob><00:00:01.000>Hi <00:00:01.500>Ann"},
			{2, 3, "No timestamps"},
			{3, 4, "<v Bob><00:00:03.000>Bye"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := build(tt.cues...)
			report, err := resegment.ByWords(s, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, describe(s))
			assert.Equal(t, resegment.Report{CuesBefore: len(tt.cues), CuesAfter: len(tt.want)}, report)
			for _, c := range s.All() {
				assert.NoError(t, c.ValidateWords())
			}
		})
	}
}

func TestByWordsKeepsIdentityAndMovesComments(t *testing.T) {
	t.Parallel()

	first := newCue(0, 2, "<00:00:00.000>One <00:00:01.000>two")
	first.ID = "first"
	second := newCue(2, 4, "<00:00:02.000>three. <00:00:03.000>Four.")
	second.ID = "second"
	s := build(first, second)
	require.NoError(t, s.AddComment("before second", 1))

	_, err := resegment.ByWords(s, resegment.Options{})
	require.NoError(t, err)

	var ids []string
	for _, c := range s.All() {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"first", ""}, ids)
	assert.Equal(t, []model.Comment{{Text: "before second", BeforeCue: 0}}, s.Comments())
}

func TestByWordsKeepsImageCues(t *testing.T) {
	t.Parallel()

	c := newCue(0, 20, "<00:00:00.000>One <00:00:01.000>two")
	c.Image = &model.Image{}
	s := build(c)
	report, err := resegment.ByWords(s, resegment.Options{})
	require.NoError(t, err)
	assert.Equal(t, resegment.Report{CuesBefore: 1, CuesAfter: 1}, report)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	negative := -time.Second
	tests := []struct {
		name string
		opts resegment.Options
		want string
	}{
		{"defaults", resegment.Options{}, ""},
		{"zero gap", resegment.Options{MaxWordGap: new(time.Duration)}, ""},
		{"negative gap", resegment.Options{MaxWordGap: &negative}, "The maximum word gap must not be negative, got -1."},
		{"limits", resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: -1}}, "The maximum characters per line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Contains(t, invalid.Msg, tt.want)

			s := build(newCue(0, 20, tensor))
			_, err = resegment.SplitLong(s, tt.opts)
			require.ErrorAs(t, err, &invalid)
			_, err = resegment.ByWords(s, tt.opts)
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, 1, s.Len())
		})
	}
}
