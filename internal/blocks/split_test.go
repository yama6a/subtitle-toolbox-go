package blocks_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
)

func TestSplitAtEmptyLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []blocks.Block
	}{
		{name: "no lines"},
		{name: "only empty lines", lines: []string{"", " ", "\t"}},
		{
			name:  "blocks keep the number of their first line",
			lines: []string{"", "1", "00:00:01,000 --> 00:00:02,000", "", "", "2", "Text"},
			want: []blocks.Block{
				{Line: 2, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 6, Lines: []string{"2", "Text"}},
			},
		},
		{
			name:  "BOMs at the start go, then runs of tabs and spaces become 1 space and the line is trimmed",
			lines: []string{"\ufeff\ufeff1", " a \t\t b  c\x00", "\ufeff", " \ufeffd"},
			want: []blocks.Block{
				{Line: 1, Lines: []string{"1", "a b c"}},
				{Line: 4, Lines: []string{"\ufeffd"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := slices.Collect(blocks.SplitAtEmptyLines(slices.All(tt.lines)))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitAtEmptyLinesStopsEarly(t *testing.T) {
	t.Parallel()

	for b := range blocks.SplitAtEmptyLines(slices.All([]string{"a", "", "b"})) {
		assert.Equal(t, 1, b.Line)
		break
	}
}

// Ports the split tests of tests/Parsers/CueBlockRulesTest.php of subtitle-toolbox-php.
func TestSplitAtTimingLines(t *testing.T) {
	t.Parallel()

	subRip := blocks.Rules{IsTimingLine: subRipTiming, HasCueNumbers: true}
	webVTT := blocks.Rules{IsTimingLine: hasArrow}
	tests := []struct {
		name  string
		rules blocks.Rules
		lines []string
		want  []blocks.Block
	}{
		{
			name:  "2 cues and a cue without a number, without empty lines",
			rules: subRip,
			lines: []string{
				"1", "00:00:01,000 --> 00:00:02,500", "The wind is cold today.",
				"2", "00:00:03,000 --> 00:00:05,000", "Snow falls in the hills.",
				"00:00:05,500 --> 00:00:07,000", "The roads are closed.",
			},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,500", "The wind is cold today."}},
				{Line: 3, Lines: []string{"2", "00:00:03,000 --> 00:00:05,000", "Snow falls in the hills."}},
				{Line: 6, Lines: []string{"00:00:05,500 --> 00:00:07,000", "The roads are closed."}},
			},
		},
		{
			name:  "the cue number goes with the timing line after it",
			rules: subRip,
			lines: []string{"7", "00:00:01,000 --> 00:00:02,000", "Hello", "8", "00:00:03,000 --> 00:00:04,000", "World"},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"7", "00:00:01,000 --> 00:00:02,000", "Hello"}},
				{Line: 3, Lines: []string{"8", "00:00:03,000 --> 00:00:04,000", "World"}},
			},
		},
		{
			name:  "a number right after a timing line starts the next cue when a timing line follows",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "00:00:03,000 --> 00:00:04,000", "Text"},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 2, Lines: []string{"42", "00:00:03,000 --> 00:00:04,000", "Text"}},
			},
		},
		{
			name:  "a number line of cue text stays",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "Text"},
			want:  []blocks.Block{{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "42", "Text"}}},
		},
		{
			name:  "2 timing lines in a row",
			rules: subRip,
			lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "00:00:03,000 --> 00:00:04,000", "Text"},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 2, Lines: []string{"00:00:03,000 --> 00:00:04,000", "Text"}},
			},
		},
		{
			name:  "text before the first timing line stays in the first part",
			rules: subRip,
			lines: []string{"Banner", "1", "00:00:01,000 --> 00:00:02,000", "Text"},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"Banner"}},
				{Line: 1, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000", "Text"}},
			},
		},
		{
			name:  "no cue numbers in WebVTT",
			rules: webVTT,
			lines: []string{"00:00:01.000 --> 00:00:02.500", "The wind is cold today.", "2", "00:00:03.000 --> 00:00:05.000", "Snow"},
			want: []blocks.Block{
				{Line: 0, Lines: []string{"00:00:01.000 --> 00:00:02.500", "The wind is cold today.", "2"}},
				{Line: 3, Lines: []string{"00:00:03.000 --> 00:00:05.000", "Snow"}},
			},
		},
		{
			name:  "no timing line",
			rules: subRip,
			lines: []string{"a", "b"},
			want:  []blocks.Block{{Line: 0, Lines: []string{"a", "b"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.rules.SplitAtTimingLines(tt.lines))
		})
	}
}
