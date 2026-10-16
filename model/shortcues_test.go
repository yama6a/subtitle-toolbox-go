package model_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func limits(l model.CueLimits) model.MergeShortCuesOptions {
	return model.MergeShortCuesOptions{Limits: l}
}

// Adapted from tests/ShortCueMergingTest.php of subtitle-toolbox-php.
func TestMergeShortCues(t *testing.T) {
	t.Parallel()

	bridge := []cueSpec{{0, 0.5, "The bridge opened"}, {0.5, 1, "last spring."}}
	dialogue := []cueSpec{{0, 0.5, "- Is it safe?"}, {0.5, 1, "- Yes."}}
	sameSpeaker := model.MergeShortCuesOptions{MergeSameSpeakerAnyDuration: true}
	keepSentenceEnds := model.MergeShortCuesOptions{KeepSentenceEnds: true}
	tests := []struct {
		name string
		in   []cueSpec
		opts model.MergeShortCuesOptions
		want []cueSpec
	}{
		{
			name: "three short cues with small gaps",
			in:   []cueSpec{{62.1, 62.6, "Wait."}, {62.64, 63.3, "Where are you"}, {63.32, 64.1, "going?"}},
			want: []cueSpec{{62.1, 64.1, "Wait. Where are you going?"}},
		},
		{
			name: "a cue of the minimum duration is not short",
			in:   []cueSpec{{0, 1, "One."}, {1.1, 2.1, "Two."}},
			want: []cueSpec{{0, 1, "One."}, {1.1, 2.1, "Two."}},
		},
		{
			name: "a cue below the minimum duration is short",
			in:   []cueSpec{{0, 1, "One."}, {1.1, 2.1, "Two."}},
			opts: limits(model.CueLimits{MinDuration: new(1001 * time.Millisecond)}),
			want: []cueSpec{{0, 2.1, "One. Two."}},
		},
		{
			name: "the minimum duration rounds to whole milliseconds",
			in:   []cueSpec{{0, 1, "One."}, {1.1, 2.1, "Two."}},
			opts: limits(model.CueLimits{MinDuration: new(1000400 * time.Microsecond)}),
			want: []cueSpec{{0, 1, "One."}, {1.1, 2.1, "Two."}},
		},
		{
			name: "a minimum duration of 0 makes no cue short",
			in:   []cueSpec{{0, 0, "One"}, {0, 0.5, "two"}},
			opts: limits(model.CueLimits{MinDuration: new(time.Duration(0))}),
			want: []cueSpec{{0, 0, "One"}, {0, 0.5, "two"}},
		},
		{
			name: "minimum characters off by default",
			in:   []cueSpec{{0, 2, "<i>Yes.</i>"}, {2.1, 4, "That is the plan."}},
			want: []cueSpec{{0, 2, "<i>Yes.</i>"}, {2.1, 4, "That is the plan."}},
		},
		{
			name: "4 visible characters are not below 4",
			in:   []cueSpec{{0, 2, "<i>Yes.</i>"}, {2.1, 4, "That is the plan."}},
			opts: model.MergeShortCuesOptions{MinCharacters: 4},
			want: []cueSpec{{0, 2, "<i>Yes.</i>"}, {2.1, 4, "That is the plan."}},
		},
		{
			name: "4 visible characters are below 5",
			in:   []cueSpec{{0, 2, "<i>Yes.</i>"}, {2.1, 4, "That is the plan."}},
			opts: model.MergeShortCuesOptions{MinCharacters: 5},
			want: []cueSpec{{0, 4, "<i>Yes.</i> That is the plan."}},
		},
		{
			name: "gap of the default maximum",
			in:   []cueSpec{{0, 0.5, "One"}, {0.75, 1.5, "two"}},
			want: []cueSpec{{0, 1.5, "One two"}},
		},
		{
			name: "gap above the maximum",
			in:   []cueSpec{{0, 0.5, "One"}, {0.75, 1.5, "two"}},
			opts: model.MergeShortCuesOptions{MaxGap: new(249 * time.Millisecond)},
			want: []cueSpec{{0, 0.5, "One"}, {0.75, 1.5, "two"}},
		},
		{
			name: "the maximum gap rounds to whole milliseconds",
			in:   []cueSpec{{0, 0.5, "One"}, {0.75, 1.5, "two"}},
			opts: model.MergeShortCuesOptions{MaxGap: new(249600 * time.Microsecond)},
			want: []cueSpec{{0, 1.5, "One two"}},
		},
		{
			name: "a maximum gap of 0 joins touching cues",
			in:   []cueSpec{{0, 0.5, "One"}, {0.5, 1, "two"}, {1.001, 1.5, "three"}},
			opts: model.MergeShortCuesOptions{MaxGap: new(time.Duration(0))},
			want: []cueSpec{{0, 1, "One two"}, {1.001, 1.5, "three"}},
		},
		{
			name: "an overlap passes the gap check",
			in:   []cueSpec{{0, 0.6, "One"}, {0.4, 0.9, "two"}},
			want: []cueSpec{{0, 0.9, "One two"}},
		},
		{name: "joined on one line", in: bridge, want: []cueSpec{{0, 1, "The bridge opened last spring."}}},
		{
			name: "joined on 2 lines",
			in:   bridge,
			opts: limits(model.CueLimits{MaxCharactersPerLine: 20}),
			want: []cueSpec{{0, 1, "The bridge opened\nlast spring."}},
		},
		{
			name: "too long for 1 line",
			in:   bridge,
			opts: limits(model.CueLimits{MaxCharactersPerLine: 20, MaxLinesPerCue: 1}),
			want: bridge,
		},
		{
			name: "a word too long for the line",
			in:   bridge,
			opts: limits(model.CueLimits{MaxCharactersPerLine: 16}),
			want: bridge,
		},
		{
			name: "the wrap closes and opens the tags again",
			in:   []cueSpec{{0, 0.5, "<i>The bridge opened"}, {0.5, 1, "last spring.</i>"}},
			opts: limits(model.CueLimits{MaxCharactersPerLine: 20}),
			want: []cueSpec{{0, 1, "<i>The bridge opened</i>\n<i>last spring.</i>"}},
		},
		{name: "a dialogue dash keeps its line", in: dialogue, want: []cueSpec{{0, 1, "- Is it safe?\n- Yes."}}},
		{
			name: "a dialogue dash after a tag keeps its line",
			in:   []cueSpec{{0, 0.5, "- Ready?"}, {0.5, 1, "<i>- Yes.</i>"}},
			want: []cueSpec{{0, 1, "- Ready?\n<i>- Yes.</i>"}},
		},
		{name: "dialogue on 1 line", in: dialogue, opts: limits(model.CueLimits{MaxLinesPerCue: 1}), want: dialogue},
		{name: "dialogue on 12 characters", in: dialogue, opts: limits(model.CueLimits{MaxCharactersPerLine: 12}), want: dialogue},
		{
			name: "duration of the default maximum",
			in:   []cueSpec{{0, 6.5, "The first part of the bridge"}, {6.5, 7, "opened."}},
			want: []cueSpec{{0, 7, "The first part of the bridge opened."}},
		},
		{
			name: "duration above the maximum",
			in:   []cueSpec{{0, 6.5, "The first part of the bridge"}, {6.5, 7, "opened."}},
			opts: limits(model.CueLimits{MaxDuration: 6999 * time.Millisecond}),
			want: []cueSpec{{0, 6.5, "The first part of the bridge"}, {6.5, 7, "opened."}},
		},
		{
			name: "the maximum duration rounds to whole milliseconds",
			in:   []cueSpec{{0, 6.5, "The first part of the bridge"}, {6.5, 7, "opened."}},
			opts: limits(model.CueLimits{MaxDuration: 6999600 * time.Microsecond}),
			want: []cueSpec{{0, 7, "The first part of the bridge opened."}},
		},
		{
			name: "characters per second at the maximum",
			in:   []cueSpec{{0, 0.5, "Wait."}, {0.5, 1, "Where are you going?"}},
			opts: limits(model.CueLimits{MaxCharactersPerSecond: 26}),
			want: []cueSpec{{0, 1, "Wait. Where are you going?"}},
		},
		{
			name: "characters per second above the maximum",
			in:   []cueSpec{{0, 0.5, "Wait."}, {0.5, 1, "Where are you going?"}},
			opts: limits(model.CueLimits{MaxCharactersPerSecond: 25}),
			want: []cueSpec{{0, 0.5, "Wait."}, {0.5, 1, "Where are you going?"}},
		},
		{
			name: "a duration of 0 has infinite characters per second",
			in:   []cueSpec{{0, 0, "Wait."}, {0, 0, "Now."}},
			opts: limits(model.CueLimits{MaxCharactersPerSecond: 1000}),
			want: []cueSpec{{0, 0, "Wait."}, {0, 0, "Now."}},
		},
		{
			name: "tries the previous cue when the next cue does not fit",
			in:   []cueSpec{{0, 2, "We start"}, {2.1, 2.5, "now."}, {4, 5, "Next part."}},
			want: []cueSpec{{0, 2.5, "We start now."}, {4, 5, "Next part."}},
		},
		{
			name: "the previous cue does not fit either",
			in:   []cueSpec{{0, 2, "We start"}, {2.1, 2.5, "now."}},
			opts: limits(model.CueLimits{MaxDuration: 2 * time.Second}),
			want: []cueSpec{{0, 2, "We start"}, {2.1, 2.5, "now."}},
		},
		{
			name: "joins again while the joined cue is short",
			in:   []cueSpec{{0, 0.3, "One"}, {0.3, 0.6, "two"}, {0.7, 1.2, "three"}},
			want: []cueSpec{{0, 1.2, "One two three"}},
		},
		{
			name: "different speakers",
			in:   []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "<v Ben>Hello."}},
			want: []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "<v Ben>Hello."}},
		},
		{
			name: "a speaker and no speaker",
			in:   []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "Hello."}},
			want: []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "Hello."}},
		},
		{
			name: "one speaker keeps the first voice tag",
			in:   []cueSpec{{0, 0.5, "<v.loud Anna>Hi.</v>"}, {0.5, 1, "<v Anna>Hello.</v>"}},
			want: []cueSpec{{0, 1, "<v.loud Anna>Hi. Hello.</v>"}},
		},
		{
			name: "one speaker removes the voice tags in one pass",
			in:   []cueSpec{{0, 0.5, "<v Anna>Hi </<v Anna>v>"}, {0.5, 1, "<v Anna>there.</V>"}},
			want: []cueSpec{{0, 1, "<v Anna>Hi </v> there.</v>"}},
		},
		{
			name: "same speaker at any duration",
			in:   []cueSpec{{0, 5, "<v Anna>I grew up near the coast"}, {5.1, 9, "<v Anna>and my parents"}},
			opts: sameSpeaker,
			want: []cueSpec{{0, 9, "<v Anna>I grew up near the coast and my parents"}},
		},
		{
			name: "same speaker needs a speaker",
			in:   []cueSpec{{0, 0.5, "Hi."}, {0.5, 1, "Hello."}},
			opts: sameSpeaker,
			want: []cueSpec{{0, 0.5, "Hi."}, {0.5, 1, "Hello."}},
		},
		{
			name: "same speaker needs the same speaker",
			in:   []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "<v Ben>Hello."}},
			opts: sameSpeaker,
			want: []cueSpec{{0, 0.5, "<v Anna>Hi."}, {0.5, 1, "<v Ben>Hello."}},
		},
		{
			name: "keeps a sentence end",
			in:   []cueSpec{{0, 0.5, "Wait."}, {0.5, 1, "Where are you"}, {1, 1.5, "going?"}},
			opts: keepSentenceEnds,
			want: []cueSpec{{0, 0.5, "Wait."}, {0.5, 1.5, "Where are you going?"}},
		},
		{
			name: "keeps a sentence end inside a tag",
			in:   []cueSpec{{0, 0.5, "<i>Stop!</i>"}, {0.5, 1, "Now."}},
			opts: keepSentenceEnds,
			want: []cueSpec{{0, 0.5, "<i>Stop!</i>"}, {0.5, 1, "Now."}},
		},
		{name: "empty", in: nil, want: []cueSpec{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(tt.in...)
			require.NoError(t, s.MergeShortCues(tt.opts))
			assert.Equal(t, tt.want, describe(s))
		})
	}
}

// Adapted from testWalksTheCuesInTimeOrder in tests/ShortCueMergingTest.php of subtitle-toolbox-php.
func TestMergeShortCuesWalksTheCuesInTimeOrder(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	s.AddCue(model.Cue{Start: 500 * time.Millisecond, End: time.Second, Lines: []string{"two"}})
	s.AddCue(model.Cue{Start: 0, End: 500 * time.Millisecond, Lines: []string{"One"}})

	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

	assert.Equal(t, []cueSpec{{0, 1, "One two"}}, describe(&s))
}

// Adapted from testNeverJoinsDifferentAlignmentsForcedFlagsOrImageCues in tests/ShortCueMergingTest.php of subtitle-toolbox-php.
func TestMergeShortCuesJoinCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(*model.Cue)
		want   int
	}{
		{"another alignment", func(c *model.Cue) { c.Alignment = model.TopCenter }, 2},
		{"the same alignment", func(c *model.Cue) { c.Alignment = model.BottomCenter }, 1},
		{"another forced flag", func(c *model.Cue) { c.Forced = true }, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := subtitleOf(cueSpec{0, 0.5, "One"}, cueSpec{0.5, 1, "two"})
			setCue(t, s, 1, tt.change)

			require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

			assert.Equal(t, tt.want, s.Len())
		})
	}
}

// Adapted from testJoinedCueKeepsIdentifierFormatDataAndComments in tests/ShortCueMergingTest.php of subtitle-toolbox-php.
func TestMergeShortCuesKeepsTheFirstCueAndTheComments(t *testing.T) {
	t.Parallel()

	s := subtitleOf(cueSpec{0, 0.5, "One"}, cueSpec{0.5, 1, "two"}, cueSpec{3, 5, "Three."})
	setCue(t, s, 0, func(c *model.Cue) { c.ID = "a"; vttLine(t, "0")(c) })
	setCue(t, s, 1, func(c *model.Cue) { c.ID = "b"; vttLine(t, "5")(c) })
	withComments(t, s, model.Comment{Text: "before one", BeforeCue: 0}, model.Comment{Text: "before two", BeforeCue: 1},
		model.Comment{Text: "before three", BeforeCue: 2}, model.Comment{Text: "at the end", BeforeCue: 3})

	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

	assert.Equal(t, []cueSpec{{0, 1, "One two"}, {3, 5, "Three."}}, describe(s))
	first, _ := s.Cue(0)
	assert.Equal(t, "a", first.ID)
	d, ok := first.FormatData("vtt")
	require.True(t, ok)
	line, _ := d.Get("line")
	assert.Equal(t, "0", line)
	assert.Equal(t, []model.Comment{
		{Text: "before one", BeforeCue: 0},
		{Text: "before two", BeforeCue: 0},
		{Text: "before three", BeforeCue: 1},
		{Text: "at the end", BeforeCue: 2},
	}, s.Comments())
}

// Adapted from testMergedLinesGetOneRightToLeftEmbedding in tests/ShortCueMergingTest.php of subtitle-toolbox-php.
func TestMergeShortCuesKeepsOneRightToLeftEmbedding(t *testing.T) {
	t.Parallel()

	var row struct {
		Cues []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"cues"`
		Want []string `json:"want"`
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "short-cue-rows/rtl.json"), &row))
	s := &model.Subtitle{}
	for _, c := range row.Cues {
		s.AddCue(model.Cue{Start: sec(c.Start), End: sec(c.End), Lines: []string{c.Text}})
	}
	s.FixRightToLeft()

	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

	assert.Equal(t, row.Want, texts(s))
}

func readWebVTT(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	s, warnings, err := webvtt.Read(bytes.NewReader(testfiles.Read(tb, name)), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(tb, err)
	require.Empty(tb, warnings)
	return s
}

// Adapted from testRealSpeechToTextFile and testRealInterviewFileWithSameSpeakerOnly in tests/ShortCueMergingTest.php
// of subtitle-toolbox-php.
func TestMergeShortCuesFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in, want string
		opts     model.MergeShortCuesOptions
	}{
		{name: "speech to text", in: "short-cues/own_speech_to_text.srt", want: "short-cues/own_speech_to_text_merged.srt"},
		{name: "interview, defaults", in: "short-cues/own_interview.vtt", want: "short-cues/own_interview.vtt"},
		{
			name: "interview, same speaker", in: "short-cues/own_interview.vtt", want: "short-cues/own_interview_merged.vtt",
			opts: model.MergeShortCuesOptions{MergeSameSpeakerAnyDuration: true},
		},
	}
	read := func(tb testing.TB, name string) *model.Subtitle {
		tb.Helper()
		if name[len(name)-4:] == ".vtt" {
			return readWebVTT(tb, name)
		}
		return readSubRip(tb, name)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := read(t, tt.in)
			require.NoError(t, s.MergeShortCues(tt.opts))
			want := read(t, tt.want)
			assert.Equal(t, timesAndLines(want), timesAndLines(s))
			assert.Equal(t, want.Comments(), s.Comments())
		})
	}
}

func TestMergeShortCuesOfTheSpeechToTextFile(t *testing.T) {
	t.Parallel()

	s := readSubRip(t, "short-cues/own_speech_to_text.srt")
	require.Equal(t, 10, s.Len())

	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

	cues := describe(s)
	require.Len(t, cues, 7)
	assert.Equal(t, cueSpec{0.5, 1.76, "Good morning. Today we look at"}, cues[0])
	assert.Equal(t, cueSpec{4, 4.4, "Okay."}, cues[2])
	assert.Equal(t, cueSpec{10.64, 12.1, "- Is it safe?\n- Yes, it is. <i>Thanks.</i>"}, cues[5])
	assert.Equal(t, cueSpec{15, 15.4, "Bye."}, cues[6])
}

func TestCueLimitsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		limits model.CueLimits
		option string
		want   string
	}{
		{name: "zero value"},
		{name: "minimum duration of 0", limits: model.CueLimits{MinDuration: new(time.Duration(0))}},
		{name: "minimum duration equal to the maximum", limits: model.CueLimits{MinDuration: new(2 * time.Second), MaxDuration: 2 * time.Second}},
		{
			name: "negative characters per line", limits: model.CueLimits{MaxCharactersPerLine: -1}, option: "CueLimits.MaxCharactersPerLine",
			want: "The maximum characters per line and the maximum lines must be at least 1, got -1 and 2.",
		},
		{
			name: "negative lines", limits: model.CueLimits{MaxLinesPerCue: -3}, option: "CueLimits.MaxLinesPerCue",
			want: "The maximum characters per line and the maximum lines must be at least 1, got 42 and -3.",
		},
		{
			name: "negative minimum duration", limits: model.CueLimits{MinDuration: new(-time.Millisecond)}, option: "CueLimits.MinDuration",
			want: "The minimum duration must not be negative, got -0.001.",
		},
		{
			name: "negative maximum duration", limits: model.CueLimits{MaxDuration: -time.Second}, option: "CueLimits.MaxDuration",
			want: "The maximum duration and the maximum characters per second must be greater than 0, got -1 and no limit.",
		},
		{
			name: "negative characters per second", limits: model.CueLimits{MaxCharactersPerSecond: -1}, option: "CueLimits.MaxCharactersPerSecond",
			want: "The maximum duration and the maximum characters per second must be greater than 0, got 7 and -1.",
		},
		{
			name: "NaN characters per second", limits: model.CueLimits{MaxCharactersPerSecond: math.NaN()}, option: "CueLimits.MaxCharactersPerSecond",
			want: "The maximum duration and the maximum characters per second must be greater than 0, got 7 and NAN.",
		},
		{
			name: "infinite characters per second", limits: model.CueLimits{MaxCharactersPerSecond: math.Inf(1)}, option: "CueLimits.MaxCharactersPerSecond",
			want: "The maximum duration and the maximum characters per second must be greater than 0, got 7 and INF.",
		},
		{
			name: "minimum duration above the default maximum", limits: model.CueLimits{MinDuration: new(8 * time.Second)}, option: "CueLimits.MinDuration",
			want: "The minimum duration must not be greater than the maximum duration, got 8 and 7.",
		},
		{
			name: "default minimum duration above the maximum", limits: model.CueLimits{MaxDuration: 500 * time.Millisecond}, option: "CueLimits.MinDuration",
			want: "The minimum duration must not be greater than the maximum duration, got 1 and 0.5.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.limits.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			assertInvalidOption(t, err, tt.option, tt.want)
		})
	}
}

func TestMergeShortCuesOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		opts   model.MergeShortCuesOptions
		option string
		want   string
	}{
		{name: "zero value"},
		{name: "maximum gap of 0", opts: model.MergeShortCuesOptions{MaxGap: new(time.Duration(0))}},
		{
			name: "limits", opts: limits(model.CueLimits{MaxDuration: -time.Second}), option: "MergeShortCuesOptions.Limits.MaxDuration",
			want: "The maximum duration and the maximum characters per second must be greater than 0, got -1 and no limit.",
		},
		{
			name: "negative maximum gap", opts: model.MergeShortCuesOptions{MaxGap: new(-time.Millisecond)}, option: "MergeShortCuesOptions.MaxGap",
			want: "The maximum gap must not be negative, got -0.001.",
		},
		{
			name: "negative minimum characters", opts: model.MergeShortCuesOptions{MinCharacters: -1}, option: "MergeShortCuesOptions.MinCharacters",
			want: "The minimum characters must not be negative, got -1.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			assertInvalidOption(t, err, tt.option, tt.want)

			s := subtitleOf(cueSpec{0, 0.5, "One"}, cueSpec{0.5, 1, "two"})
			require.ErrorIs(t, s.MergeShortCues(tt.opts), model.ErrInvalidArgument)
			assert.Equal(t, 2, s.Len())
		})
	}
}

func assertInvalidOption(t *testing.T, err error, option, msg string) {
	t.Helper()

	require.ErrorIs(t, err, model.ErrInvalidArgument)
	var e *model.InvalidArgumentError
	require.ErrorAs(t, err, &e)
	assert.Equal(t, option, e.Option)
	assert.Equal(t, msg, e.Msg)
}

// wordsEvery600ms builds n cues "wordN" of 0.5 s every 0.6 s with a comment before each, as
// tests/CueListSpeedTest.php of subtitle-toolbox-php does.
func wordsEvery600ms(tb testing.TB, n int) *model.Subtitle {
	tb.Helper()

	cues := make([]model.Cue, n)
	for i := range cues {
		start := time.Duration(i) * 600 * time.Millisecond
		cues[i] = model.Cue{Start: start, End: start + 500*time.Millisecond, Lines: []string{fmt.Sprintf("word%d", i)}}
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	for i := range n {
		require.NoError(tb, s.AddComment(fmt.Sprintf("note %d", i), i))
	}
	return s
}

// TestMergeShortCuesInPairs holds the result checks of testMergeShortCuesJoinsTwentyThousandCuesUnderTenSeconds
// in tests/CueListSpeedTest.php of subtitle-toolbox-php, at 200 cues.
func TestMergeShortCuesInPairs(t *testing.T) {
	t.Parallel()

	s := wordsEvery600ms(t, 200)

	require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{}))

	assert.Equal(t, 100, s.Len())
	first, _ := s.Cue(0)
	assert.Equal(t, []string{"word0 word1"}, first.Lines)
	assert.Equal(t, []int{0, 0, 1, 1}, commentIndexes(s)[:4])
	assert.Equal(t, 99, commentIndexes(s)[199])
}

func TestMergeShortCuesGrowth(t *testing.T) {
	t.Parallel()

	assertGrowth(t, 2000, func(n int) func() {
		s := wordsEvery600ms(t, n)
		return func() { require.NoError(t, s.MergeShortCues(model.MergeShortCuesOptions{})) }
	})
}

func BenchmarkMergeShortCues(b *testing.B) {
	b.Run("cues=20000", func(b *testing.B) {
		benchmarkJoin(b, wordsEvery600ms(b, 20000), func(s *model.Subtitle) error {
			return s.MergeShortCues(model.MergeShortCuesOptions{})
		})
	})
}
