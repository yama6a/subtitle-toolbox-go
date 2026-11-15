package textblock_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textblock"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func hasArrow(line string) bool { return strings.Contains(line, "-->") }

type join struct {
	block, line int
	lines       []string
}

func TestJoinCueTextBlocks(t *testing.T) {
	t.Parallel()

	isNote := func(lines []string) bool { return strings.HasPrefix(lines[0], "NOTE") }
	tests := []struct {
		name      string
		in        []textblock.Block
		isOther   func([]string) bool
		want      []textblock.Block
		wantJoins []join
	}{
		{name: "no blocks"},
		{
			name: "text after an empty line joins its cue",
			in: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 5, Lines: []string{"The ferry leaves."}},
				{Line: 7, Lines: []string{"at noon"}},
			},
			want: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "The ferry leaves.", "at noon"}},
			},
			wantJoins: []join{{1, 5, []string{"The ferry leaves."}}, {1, 7, []string{"at noon"}}},
		},
		{
			name: "text after a block without a timing line stays apart",
			in: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"stray"}},
			},
			want: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"stray"}},
			},
		},
		{
			name: "a block of another kind stays apart",
			in: []textblock.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 4, Lines: []string{"NOTE checked"}},
				{Line: 6, Lines: []string{"text"}},
			},
			isOther: isNote,
			want: []textblock.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 4, Lines: []string{"NOTE checked"}},
				{Line: 6, Lines: []string{"text"}},
			},
		},
		{
			name: "the block number counts the blocks before it",
			in: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 6, Lines: []string{"00:03.000 --> 00:04.000"}},
				{Line: 8, Lines: []string{"Two"}},
			},
			want: []textblock.Block{
				{Line: 1, Lines: []string{"WEBVTT"}},
				{Line: 3, Lines: []string{"00:01.000 --> 00:02.000", "One"}},
				{Line: 6, Lines: []string{"00:03.000 --> 00:04.000", "Two"}},
			},
			wantJoins: []join{{2, 8, []string{"Two"}}},
		},
		{
			name: "a broken timing line stays apart",
			in: []textblock.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 3, Lines: []string{"00:03.000 -> 00:04.000"}},
			},
			want: []textblock.Block{
				{Line: 1, Lines: []string{"00:01.000 --> 00:02.000"}},
				{Line: 3, Lines: []string{"00:03.000 -> 00:04.000"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rules := textblock.Rules{IsTimingLine: hasArrow, IsOtherBlock: tt.isOther}
			strict := parsing.NewState(model.SubRip, model.CommonReadOptions{})
			assert.Equal(t, tt.want, slices.Collect(textblock.JoinCueTextBlocks(slices.Values(tt.in), rules, strict)))
			assert.Empty(t, strict.Warnings())

			lenient := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: true})
			assert.Equal(t, tt.want, slices.Collect(textblock.JoinCueTextBlocks(slices.Values(tt.in), rules, lenient)))
			joins := make([]join, 0, len(lenient.Warnings()))
			for _, w := range lenient.Warnings() {
				assert.Equal(t, textblock.JoinWarning(w.Block, w.Line), w.Message)
				assert.Equal(t, model.Repaired, w.Action)
				joins = append(joins, join{w.Block, w.Line, w.Lines})
			}
			require.Len(t, joins, len(tt.wantJoins))
			for i, j := range tt.wantJoins {
				assert.Equal(t, j, joins[i])
			}
		})
	}
}

func TestJoinCueTextBlocksStopsEarly(t *testing.T) {
	t.Parallel()

	in := []textblock.Block{{Line: 1, Lines: []string{"a"}}, {Line: 3, Lines: []string{"b"}}}
	for b := range textblock.JoinCueTextBlocks(slices.Values(in), textblock.Rules{IsTimingLine: hasArrow}, parsing.NewState(model.SubRip, model.CommonReadOptions{})) {
		assert.Equal(t, 1, b.Line)
		break
	}
}

func TestJoinWarning(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Block #2 has an empty line before line 9 inside the cue. The parser kept the text after it in the cue.",
		textblock.JoinWarning(2, 9))
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

			assert.Equal(t, tt.want, textblock.IsNumeric(tt.in))
		})
	}
}
