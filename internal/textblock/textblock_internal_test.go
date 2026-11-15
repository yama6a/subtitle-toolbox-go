package textblock

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func hasArrow(line string) bool { return strings.Contains(line, "-->") }

func subRipTiming(line string) bool {
	return strings.Contains(line, ",") && strings.Contains(line, " --> ")
}

// Ports testWebVttRulesHaveNoCueNumbers, testFindsCueTextBlocks and testFindsATimingLine
// of tests/Parsers/CueBlockRulesTest.php of subtitle-toolbox-php.
func TestIsCueTextBlock(t *testing.T) {
	t.Parallel()

	subRip := Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	webVTT := Rules{IsTimingLine: hasArrow}
	tests := []struct {
		name  string
		rules Rules
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

			assert.Equal(t, tt.want, tt.rules.isCueTextBlock(tt.lines))
		})
	}
}

func TestHasTimingLine(t *testing.T) {
	t.Parallel()

	r := Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	assert.True(t, r.hasTimingLine([]string{"1", "00:00:01,000 --> 00:00:02,000"}))
	assert.False(t, r.hasTimingLine([]string{"1", "Text"}))
	assert.False(t, r.hasTimingLine(nil))
}

// Ports the split tests of tests/Parsers/CueBlockRulesTest.php of subtitle-toolbox-php.
func TestSplitAtTimingLines(t *testing.T) {
	t.Parallel()

	subRip := Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	webVTT := Rules{IsTimingLine: hasArrow}
	tests := []struct {
		name  string
		rules Rules
		lines []string
		want  []Block
	}{
		{
			name:  "2 cues and a cue without a number, without empty lines",
			rules: subRip,
			lines: []string{
				"1", "00:00:01,000 --> 00:00:02,500", "The wind is cold today.",
				"2", "00:00:03,000 --> 00:00:05,000", "Snow falls in the hills.",
				"00:00:05,500 --> 00:00:07,000", "The roads are closed.",
			},
			want: []Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,500", "The wind is cold today."}},
				{Line: 3, Lines: []string{"2", "00:00:03,000 --> 00:00:05,000", "Snow falls in the hills."}},
				{Line: 6, Lines: []string{"00:00:05,500 --> 00:00:07,000", "The roads are closed."}},
			},
		},
		{
			name:  "the cue number goes with the timing line after it",
			rules: subRip,
			lines: []string{"7", "00:00:01,000 --> 00:00:02,000", "Hello", "8", "00:00:03,000 --> 00:00:04,000", "World"},
			want: []Block{
				{Line: 0, Lines: []string{"7", "00:00:01,000 --> 00:00:02,000", "Hello"}},
				{Line: 3, Lines: []string{"8", "00:00:03,000 --> 00:00:04,000", "World"}},
			},
		},
		{
			name:  "a number right after a timing line starts the next cue when a timing line follows",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "00:00:03,000 --> 00:00:04,000", "Text"},
			want: []Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 2, Lines: []string{"42", "00:00:03,000 --> 00:00:04,000", "Text"}},
			},
		},
		{
			name:  "a number line of cue text stays",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "Text"},
			want:  []Block{{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "Text"}}},
		},
		{
			name:  "2 timing lines in a row",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "00:00:03,000 --> 00:00:04,000", "Text"},
			want: []Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 2, Lines: []string{"00:00:03,000 --> 00:00:04,000", "Text"}},
			},
		},
		{
			name:  "text before the first timing line stays in the first part",
			rules: subRip,
			lines: []string{"Banner", "1", "00:00:01,000 --> 00:00:02,000", "Text"},
			want: []Block{
				{Line: 0, Lines: []string{"Banner"}},
				{Line: 1, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "Text"}},
			},
		},
		{
			name:  "no cue numbers in WebVTT",
			rules: webVTT,
			lines: []string{"00:00:01.000 --> 00:00:02.500", "The wind is cold today.", "2", "00:00:03.000 --> 00:00:05.000", "Snow"},
			want: []Block{
				{Line: 0, Lines: []string{"00:00:01.000 --> 00:00:02.500", "The wind is cold today.", "2"}},
				{Line: 3, Lines: []string{"00:00:03.000 --> 00:00:05.000", "Snow"}},
			},
		},
		{
			name:  "no timing line",
			rules: subRip,
			lines: []string{"a", "b"},
			want:  []Block{{Line: 0, Lines: []string{"a", "b"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.rules.splitAtTimingLines(tt.lines))
		})
	}
}

func TestSplitWarning(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Block #2 has no empty line before line 9. The parser split the block there.", splitWarning(2, 9))
}
