package textblock_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/textblock"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestSplitAtEmptyLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []textblock.Block
	}{
		{name: "no lines"},
		{name: "only empty lines", lines: []string{"", " ", "\t"}},
		{
			name:  "blocks keep the number of their first line",
			lines: []string{"", "1", "00:00:01,000 --> 00:00:02,000", "", "", "2", "Text"},
			want: []textblock.Block{
				{Line: 2, Lines: []string{"1", "00:00:01,000 --> 00:00:02,000"}},
				{Line: 6, Lines: []string{"2", "Text"}},
			},
		},
		{
			name:  "BOMs at the start go, then runs of tabs and spaces become 1 space and the line is trimmed",
			lines: []string{"\ufeff\ufeff1", " a \t\t b  c\x00", "\ufeff", " \ufeffd"},
			want: []textblock.Block{
				{Line: 1, Lines: []string{"1", "a b c"}},
				{Line: 4, Lines: []string{"\ufeffd"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := slices.Collect(textblock.SplitAtEmptyLines(slices.All(tt.lines)))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitAtEmptyLinesStopsEarly(t *testing.T) {
	t.Parallel()

	for b := range textblock.SplitAtEmptyLines(slices.All([]string{"a", "", "b"})) {
		assert.Equal(t, 1, b.Line)
		break
	}
}

func TestReadParts(t *testing.T) {
	t.Parallel()

	rules := textblock.Rules{IsTimingLine: hasArrow}
	block := textblock.Block{Line: 5, Lines: []string{"a --> b", "One", "c --> d", "bad", "e --> f"}}
	errBoom := errors.New("boom")
	tests := []struct {
		name         string
		lenient      bool
		parse        func(lines []string, index, line int) (model.Cue, error)
		want         []string
		wantErr      error
		wantWarnings []model.ParseWarning
	}{
		{
			name:    "lenient mode warns for each split and skips a part that fails",
			lenient: true,
			parse:   parseExcept("bad", 0),
			want:    []string{"a --> b", "e --> f"},
			wantWarnings: []model.ParseWarning{
				{Message: "Block #3 has no empty line before line 7. The parser split the block there.", Line: 7, Block: 3, Lines: block.Lines, Action: model.Repaired},
				{Message: "Block #3 has no empty line before line 9. The parser split the block there.", Line: 9, Block: 3, Lines: block.Lines, Action: model.Repaired},
				{Message: "Bad.", Line: 7, Block: 3, Lines: []string{"c --> d", "bad"}, Action: model.Skipped},
			},
		},
		{
			name:    "strict mode returns the error",
			parse:   parseExcept("bad", 0),
			wantErr: &model.ParseError{Format: model.SubRip, Line: 0, Msg: "bad"},
		},
		{
			name:    "the line of the error wins",
			lenient: true,
			parse:   parseExcept("bad", 42),
			want:    []string{"a --> b", "e --> f"},
			wantWarnings: []model.ParseWarning{
				{Message: "Block #3 has no empty line before line 7. The parser split the block there.", Line: 7, Block: 3, Lines: block.Lines, Action: model.Repaired},
				{Message: "Block #3 has no empty line before line 9. The parser split the block there.", Line: 9, Block: 3, Lines: block.Lines, Action: model.Repaired},
				{Message: "Bad.", Line: 42, Block: 3, Lines: []string{"c --> d", "bad"}, Action: model.Skipped},
			},
		},
		{
			name:    "another error ends the read",
			lenient: true,
			parse: func([]string, int, int) (model.Cue, error) {
				return model.Cue{}, errBoom
			},
			wantErr: errBoom,
			wantWarnings: []model.ParseWarning{
				{Message: "Block #3 has no empty line before line 7. The parser split the block there.", Line: 7, Block: 3, Lines: block.Lines, Action: model.Repaired},
				{Message: "Block #3 has no empty line before line 9. The parser split the block there.", Line: 9, Block: 3, Lines: block.Lines, Action: model.Repaired},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: tt.lenient})
			got, err := rules.ReadParts(st, block, 3, tt.parse)
			if tt.wantErr != nil {
				var perr *model.ParseError
				if errors.As(tt.wantErr, &perr) {
					require.ErrorAs(t, err, &perr)
					assert.Equal(t, tt.wantErr, perr)
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
			assert.Equal(t, tt.wantWarnings, st.Warnings())
		})
	}
}

// parseExcept returns a part parser that fails with msg on a part whose second line is msg.
func parseExcept(msg string, errLine int) func(lines []string, index, line int) (model.Cue, error) {
	return func(lines []string, _, _ int) (model.Cue, error) {
		if len(lines) > 1 && lines[1] == msg {
			return model.Cue{}, &model.ParseError{Line: errLine, Msg: msg}
		}
		return model.Cue{Lines: []string{lines[0]}}, nil
	}
}
