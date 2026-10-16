package blocks_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/model"
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

func TestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{""}},
		{"LF", "a\nb", []string{"a", "b"}},
		{"CR LF", "a\r\nb\r\n", []string{"a", "b", ""}},
		{"2 CRs before LF are one line end", "a\r\r\nb", []string{"a", "b"}},
		{"lone CR", "a\rb", []string{"a", "b"}},
		{"2 lone CRs", "a\r\rb", []string{"a", "", "b"}},
		{"mixed", "a\r\nb\nc\r\r\nd\re", []string{"a", "b", "c", "d", "e"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			for n, line := range blocks.Lines(tt.in) {
				assert.Equal(t, len(got), n)
				got = append(got, line)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLinesStopsEarly(t *testing.T) {
	t.Parallel()

	for range blocks.Lines("a\nb\nc") {
		break
	}
}

func TestSplitWarning(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Block #2 has no empty line before line 9. The parser split the block there.", blocks.SplitWarning(2, 9))
}

func TestReadParts(t *testing.T) {
	t.Parallel()

	rules := blocks.Rules{IsTimingLine: hasArrow}
	block := blocks.Block{Line: 5, Lines: []string{"a --> b", "One", "c --> d", "bad", "e --> f"}}
	errBoom := errors.New("boom")
	tests := []struct {
		name      string
		lenient   bool
		parse     func(lines []string, line int) (model.Cue, error)
		fail      func(err *model.ParseError, line int, lines []string) error
		want      []string
		wantErr   error
		wantSplit []int
		wantFails []int
	}{
		{
			name:      "lenient mode warns for each split and skips a part that fails",
			lenient:   true,
			parse:     parseExcept("bad", 0),
			fail:      func(*model.ParseError, int, []string) error { return nil },
			want:      []string{"a --> b", "e --> f"},
			wantSplit: []int{7, 9},
			wantFails: []int{7},
		},
		{
			name:      "strict mode returns the error of Fail",
			parse:     parseExcept("bad", 0),
			fail:      func(err *model.ParseError, _ int, _ []string) error { return err },
			want:      nil,
			wantErr:   &model.ParseError{Line: 0, Msg: "bad"},
			wantFails: []int{7},
		},
		{
			name:      "the line of the error wins",
			lenient:   true,
			parse:     parseExcept("bad", 42),
			fail:      func(*model.ParseError, int, []string) error { return nil },
			want:      []string{"a --> b", "e --> f"},
			wantSplit: []int{7, 9},
			wantFails: []int{42},
		},
		{
			name:    "another error ends the read",
			lenient: true,
			parse: func([]string, int) (model.Cue, error) {
				return model.Cue{}, errBoom
			},
			fail:      func(*model.ParseError, int, []string) error { return nil },
			wantErr:   errBoom,
			wantSplit: []int{7, 9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var split, fails []int
			got, err := rules.ReadParts(block, 3, blocks.PartReader{
				Lenient: tt.lenient,
				Split: func(msg string, line int) {
					assert.Equal(t, blocks.SplitWarning(3, line), msg)
					split = append(split, line)
				},
				Parse: tt.parse,
				Fail: func(err *model.ParseError, line int, lines []string) error {
					fails = append(fails, line)
					return tt.fail(err, line, lines)
				},
			})
			if tt.wantErr != nil {
				var perr *model.ParseError
				if errors.As(tt.wantErr, &perr) {
					require.ErrorAs(t, err, &perr)
					assert.Equal(t, "bad", perr.Msg)
				} else {
					require.ErrorIs(t, err, tt.wantErr)
				}
			} else {
				require.NoError(t, err)
			}
			var texts []string
			for _, c := range got {
				texts = append(texts, c.Lines[0])
			}
			assert.Equal(t, tt.want, texts)
			assert.Equal(t, tt.wantSplit, split)
			assert.Equal(t, tt.wantFails, fails)
		})
	}
}

// parseExcept returns a part parser that fails with msg on a part whose second line is msg.
func parseExcept(msg string, errLine int) func(lines []string, line int) (model.Cue, error) {
	return func(lines []string, _ int) (model.Cue, error) {
		if len(lines) > 1 && lines[1] == msg {
			return model.Cue{}, &model.ParseError{Line: errLine, Msg: msg}
		}
		return model.Cue{Lines: []string{lines[0]}}, nil
	}
}

func TestBlocks(t *testing.T) {
	t.Parallel()

	rules := blocks.Rules{IsTimingLine: hasArrow}
	content := "a --> b\nOne\n\nTwo\n\nc --> d\n"
	type join struct {
		msg         string
		line, block int
	}
	var joins []join
	got := slices.Collect(rules.Blocks(content, func(msg string, line, block int, _ []string) {
		joins = append(joins, join{msg, line, block})
	}))
	assert.Equal(t, []blocks.Block{
		{Line: 1, Lines: []string{"a --> b", "One", "Two"}},
		{Line: 6, Lines: []string{"c --> d"}},
	}, got)
	assert.Equal(t, []join{{blocks.JoinWarning(0, 4), 4, 0}}, joins)
	assert.Equal(t, got, slices.Collect(rules.Blocks(content, nil)))
}
