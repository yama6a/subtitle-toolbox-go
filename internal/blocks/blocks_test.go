package blocks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
)

func hasArrow(line string) bool { return strings.Contains(line, "-->") }

func subRipTiming(line string) bool {
	return strings.Contains(line, ",") && strings.Contains(line, " --> ")
}

// Ports testWebVttRulesHaveNoCueNumbers, testFindsCueTextBlocks and testFindsATimingLine
// of tests/Parsers/CueBlockRulesTest.php of subtitle-toolbox-php.
func TestIsCueTextBlock(t *testing.T) {
	t.Parallel()

	subRip := blocks.Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	webVTT := blocks.Rules{IsTimingLine: hasArrow}
	tests := []struct {
		name  string
		rules blocks.Rules
		lines []string
		want  bool
	}{
		{"text", subRip, []string{"Tickets are sold at the door."}, true},
		{"timing line", subRip, []string{"00:00:04,000 --> 00:00:06,000", "Text"}, false},
		{"short arrow", subRip, []string{"00:00:04 -> 00:00:06"}, false},
		{"broken time", subRip, []string{"00 :00:04,000 to 00:00:06,000"}, false},
		{"full-width colon", subRip, []string{"00\uff1a00:04,000"}, false},
		{"full-width digits", subRip, []string{"\uff11\uff12\uff1a\uff13\uff14 text"}, false},
		{"ideographic space before the colon", subRip, []string{"12\u3000:34 x"}, false},
		{"full-width digits without a colon", subRip, []string{"\uff11\uff12 text"}, true},
		{"number after text", subRip, []string{"Text", "3"}, true},
		{"cue number", subRip, []string{"2"}, false},
		{"no cue numbers in WebVTT", webVTT, []string{"2"}, true},
		{"time in a later line", webVTT, []string{"Text", "00:01 is late"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.rules.IsCueTextBlock(tt.lines))
		})
	}
}

func TestHasTimingLine(t *testing.T) {
	t.Parallel()

	r := blocks.Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	assert.True(t, r.HasTimingLine([]string{"1", "00:00:01,000 --> 00:00:02,000"}))
	assert.False(t, r.HasTimingLine([]string{"1", "Text"}))
	assert.False(t, r.HasTimingLine(nil))
}

type join struct {
	block, line int
	lines       []string
}

func TestJoinCueTextBlocks(t *testing.T) {
	t.Parallel()

	rules := blocks.Rules{IsTimingLine: hasArrow}
	isNote := func(lines []string) bool { return strings.HasPrefix(lines[0], "NOTE") }
	tests := []struct {
		name      string
		in        []blocks.Block
		isOther   func([]string) bool
		want      []blocks.Block
		wantJoins []join
	}{
		{name: "no blocks"},
		{
			name: "text after an empty line joins its cue",
			in: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 5, Lines: []string{"The ferry leaves."}},
				{Line: 7, Lines: []string{"at noon"}},
			},
			want: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "The ferry leaves.", "at noon"}},
			},
			wantJoins: []join{{1, 5, []string{"The ferry leaves."}}, {1, 7, []string{"at noon"}}},
		},
		{
			name: "text after a block without a timing line stays apart",
			in: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"stray"}},
			},
			want: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"stray"}},
			},
		},
		{
			name: "a block of another kind stays apart",
			in: []blocks.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 4, Lines: []string{"NOTE checked"}},
				{Line: 6, Lines: []string{"text"}},
			},
			isOther: isNote,
			want: []blocks.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 4, Lines: []string{"NOTE checked"}},
				{Line: 6, Lines: []string{"text"}},
			},
		},
		{
			name: "the block number counts the blocks before it",
			in: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 6, Lines: []string{"00:03.000 --> 00:04.000"}},
				{Line: 8, Lines: []string{"Two"}},
			},
			want: []blocks.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 6, Lines: []string{"00:03.000 --> 00:04.000", "Two"}},
			},
			wantJoins: []join{{2, 8, []string{"Two"}}},
		},
		{
			name: "a broken timing line stays apart",
			in: []blocks.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 3, Lines: []string{"00:03.000 -> 00:04.000"}},
			},
			want: []blocks.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 3, Lines: []string{"00:03.000 -> 00:04.000"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var joins []join
			onJoin := func(block, line int, lines []string) { joins = append(joins, join{block, line, lines}) }
			got := slices.Collect(blocks.JoinCueTextBlocks(slices.Values(tt.in), rules, tt.isOther, onJoin))
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantJoins, joins)
		})
	}
}

func TestJoinCueTextBlocksStopsEarly(t *testing.T) {
	t.Parallel()

	in := []blocks.Block{{Line: 1, Lines: []string{"a"}}, {Line: 3, Lines: []string{"b"}}}
	for b := range blocks.JoinCueTextBlocks(slices.Values(in), blocks.Rules{IsTimingLine: hasArrow}, nil, nil) {
		assert.Equal(t, 1, b.Line)
		break
	}
}

func TestJoinWarning(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Block #2 has an empty line before line 9 inside the cue. The parser kept the text after it in the cue.",
		blocks.JoinWarning(2, 9))
}

// Ports the forms of the cue number decision of https://github.com/yama6a/subtitle-toolbox-go/issues/20.
func TestIsNumeric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{"1", true},
		{"+1", true},
		{"-3", true},
		{"1.5", true},
		{".5", true},
		{"1.", true},
		{"1e3", true},
		{"1E-3", true},
		{" 12\t", true},
		{"\n7\v\f\r", true},
		{"", false},
		{" ", false},
		{".", false},
		{"+", false},
		{"1e", false},
		{"e3", false},
		{"1 2", false},
		{"0x1A", false},
		{"12a", false},
		{"--1", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, blocks.IsNumeric(tt.in))
		})
	}
}
