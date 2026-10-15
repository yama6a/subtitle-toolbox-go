package parsing_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  *time.Duration
		want time.Duration
	}{
		{"nil is 5 s", nil, 5 * time.Second},
		{"0 stays", new(time.Duration(0)), 0},
		{"7 s", new(7 * time.Second), 7 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := parsing.NewState(model.SubRip, model.CommonReadOptions{LastCueDuration: tt.opt})
			assert.Equal(t, tt.want, s.LastCueDuration())
		})
	}
}

// Ports testALastCueWithoutEndLastsFiveSecondsByDefault and testALastCueDurationOf0KeepsTheLastCue
// of tests/ReadOptionsTest.php of subtitle-toolbox-php, which no 1.0 reader runs.
func TestEndAtNextStart(t *testing.T) {
	t.Parallel()

	s := time.Second
	tests := []struct {
		name   string
		last   *time.Duration
		starts []time.Duration
		i      int
		want   time.Duration
	}{
		{name: "next start", starts: []time.Duration{1 * s, 2 * s}, i: 0, want: 2 * s},
		{name: "last cue lasts 5 s by default", starts: []time.Duration{1 * s, 2 * s}, i: 1, want: 7 * s},
		{name: "last cue lasts the option", last: new(7 * s), starts: []time.Duration{3600 * s}, i: 0, want: 3607 * s},
		{name: "last cue duration 0 keeps the start", last: new(time.Duration(0)), starts: []time.Duration{32800 * time.Millisecond}, i: 0, want: 32800 * time.Millisecond},
		{name: "same start does not end the cue", starts: []time.Duration{1 * s, 1 * s, 4 * s}, i: 0, want: 4 * s},
		{name: "2 cues at the same start at the end", starts: []time.Duration{1 * s, 1 * s}, i: 0, want: 6 * s},
		{name: "second of 2 cues at the same start", starts: []time.Duration{1 * s, 1 * s}, i: 1, want: 6 * s},
		{name: "an earlier start is skipped", starts: []time.Duration{3 * s, 1 * s, 5 * s}, i: 0, want: 5 * s},
		{name: "rounds to whole milliseconds", last: new(1000500 * time.Microsecond), starts: []time.Duration{0}, i: 0, want: 1001 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := parsing.NewState(model.SubRip, model.CommonReadOptions{LastCueDuration: tt.last})
			assert.Equal(t, tt.want, st.EndAtNextStart(tt.starts, tt.i))
		})
	}
}

func TestEachStateStartsWithoutWarnings(t *testing.T) {
	t.Parallel()

	common := model.CommonReadOptions{Lenient: true}
	first := parsing.NewState(model.SubRip, common)
	_, _, err := first.Prepare([]byte("a\x01\n"), false)
	require.NoError(t, err)
	assert.Len(t, first.Warnings(), 1)

	assert.Empty(t, parsing.NewState(model.SubRip, common).Warnings())
	assert.True(t, first.Lenient())
	assert.False(t, parsing.NewState(model.SubRip, model.CommonReadOptions{}).Lenient())
}

func TestFail(t *testing.T) {
	t.Parallel()

	lines := []string{"2", "00:00:05,000 --> 00:00:0G,000", "Hello"}
	tests := []struct {
		name    string
		err     *model.ParseError
		line    int
		block   int
		lines   []string
		warning model.ParseWarning
	}{
		{
			name:    "the line argument",
			err:     &model.ParseError{Format: model.SubRip, Line: 6, Msg: `The time "00:00:0G,000" is not valid.`},
			line:    6,
			block:   1,
			lines:   lines,
			warning: model.ParseWarning{Message: `The time "00:00:0G,000" is not valid.`, Line: 6, Block: 1, Lines: lines, Action: model.Skipped},
		},
		{
			name:    "no line keeps the error line in the message",
			err:     &model.ParseError{Format: model.JSON, Line: 4, Msg: "The field cues[1].end must be a number."},
			line:    model.NoLine,
			block:   1,
			warning: model.ParseWarning{Message: "The field cues[1].end must be a number. (line 4)", Line: model.NoLine, Block: 1, Action: model.Skipped},
		},
		{
			name:    "no line and an error without a line",
			err:     &model.ParseError{Format: model.JSON, Msg: "The field cues[1].end must be a number."},
			line:    model.NoLine,
			block:   model.NoBlock,
			warning: model.ParseWarning{Message: "The field cues[1].end must be a number.", Line: model.NoLine, Block: model.NoBlock, Action: model.Skipped},
		},
		{
			name:    "a line argument other than the error line",
			err:     &model.ParseError{Format: model.SubRip, Line: 7, Msg: "Block #1 is broken."},
			line:    5,
			block:   1,
			warning: model.ParseWarning{Message: "Block #1 is broken.", Line: 5, Block: 1, Action: model.Skipped},
		},
		{
			name:    "the cause without a message",
			err:     &model.ParseError{Format: model.JSON, Err: errors.New("unexpected end of JSON input")},
			line:    model.NoLine,
			block:   model.NoBlock,
			warning: model.ParseWarning{Message: "unexpected end of JSON input", Line: model.NoLine, Block: model.NoBlock, Action: model.Skipped},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strict := parsing.NewState(model.SubRip, model.CommonReadOptions{})
			err := strict.Fail(tt.err, tt.line, tt.block, tt.lines)
			require.ErrorIs(t, err, tt.err)
			assert.Empty(t, strict.Warnings())

			lenient := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: true})
			require.NoError(t, lenient.Fail(tt.err, tt.line, tt.block, tt.lines))
			assert.Equal(t, []model.ParseWarning{tt.warning}, lenient.Warnings())
		})
	}
}

func TestFailAddsTheFormatInStrictMode(t *testing.T) {
	t.Parallel()

	_, err := parsing.BoundedTime(timecode.MaxTime, "100000:00:00.000", 3)
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Empty(t, perr.Format)

	err = parsing.NewState(model.WebVTT, model.CommonReadOptions{}).Fail(perr, 3, 1, nil)
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, `vtt: line 3: The time "100000:00:00.000" is not below 100000 hours.`, err.Error())
}

func TestWarnRecordsInBothModes(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		t.Run(fmt.Sprintf("lenient %t", lenient), func(t *testing.T) {
			t.Parallel()

			s := parsing.NewState(model.CSV, model.CommonReadOptions{Lenient: lenient})
			lines := []string{"a", "b"}
			s.Warn("First.", 2, 0, lines, model.Repaired)
			s.Warn("Second.", model.NoLine, model.NoBlock, nil, model.Skipped)
			lines[0] = "changed"
			assert.Equal(t, []model.ParseWarning{
				{Message: "First.", Line: 2, Block: 0, Lines: []string{"a", "b"}, Action: model.Repaired},
				{Message: "Second.", Line: model.NoLine, Block: model.NoBlock, Action: model.Skipped},
			}, s.Warnings())
		})
	}
}

func TestWarningsReturnsACopy(t *testing.T) {
	t.Parallel()

	s := parsing.NewState(model.SubRip, model.CommonReadOptions{})
	s.Warn("First.", 1, 0, nil, model.Repaired)
	s.Warnings()[0].Message = "changed"
	assert.Equal(t, "First.", s.Warnings()[0].Message)
}

// Ports the orderedTimes cases of tests/Parsers/LenientParsingTest.php and tests/Parsers/CsvParserTest.php
// of subtitle-toolbox-php.
func TestOrderedTimes(t *testing.T) {
	t.Parallel()

	ms := time.Millisecond
	lines := []string{"2", "00:00:05,000 --> 00:00:03,000"}
	tests := []struct {
		name      string
		start     time.Duration
		end       time.Duration
		lenient   bool
		noSwap    bool
		wantStart time.Duration
		wantEnd   time.Duration
		wantErr   string
		warning   string
	}{
		{name: "in order", start: 1000 * ms, end: 2000 * ms, wantStart: 1000 * ms, wantEnd: 2000 * ms},
		{name: "end equals start", start: 2000 * ms, end: 2000 * ms, wantStart: 2000 * ms, wantEnd: 2000 * ms},
		{name: "strict", start: 5000 * ms, end: 3000 * ms, wantErr: "The cue ends at 3 s, before it starts at 5 s."},
		{
			name: "lenient swaps", start: 5000 * ms, end: 3000 * ms, lenient: true, wantStart: 3000 * ms, wantEnd: 5000 * ms,
			warning: "The cue ends at 3 s, before it starts at 5 s. The parser swapped the times.",
		},
		{
			name: "lenient swaps at 30 s", start: 31500 * ms, end: 1500 * ms, lenient: true, wantStart: 1500 * ms, wantEnd: 31500 * ms,
			warning: "The cue ends at 1.5 s, before it starts at 31.5 s. The parser swapped the times.",
		},
		{name: "lenient fails at 30.001 s", start: 30001 * ms, end: 0, lenient: true, wantErr: "The cue ends at 0 s, before it starts at 30.001 s."},
		{name: "lenient without swap", start: 5000 * ms, end: 3000 * ms, lenient: true, noSwap: true, wantErr: "The cue ends at 3 s, before it starts at 5 s."},
		{name: "fractions", start: 2 * ms, end: ms, wantErr: "The cue ends at 0.001 s, before it starts at 0.002 s."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: tt.lenient})
			start, end, err := s.OrderedTimes(tt.start, tt.end, 4, 1, lines, !tt.noSwap)
			if tt.wantErr != "" {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, model.ParseError{Format: model.SubRip, Line: 4, Msg: tt.wantErr}, *perr)
				assert.Empty(t, s.Warnings())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStart, start)
			assert.Equal(t, tt.wantEnd, end)
			if tt.warning == "" {
				assert.Empty(t, s.Warnings())
				return
			}
			assert.Equal(t, []model.ParseWarning{{Message: tt.warning, Line: 4, Block: 1, Lines: lines, Action: model.Repaired}}, s.Warnings())
		})
	}
}

func TestBoundedTime(t *testing.T) {
	t.Parallel()

	below := timecode.MaxTime - time.Millisecond
	tests := []struct {
		name    string
		d       time.Duration
		text    string
		wantErr string
	}{
		{name: "0", d: 0, text: "00:00:00,000"},
		{name: "99999:59:59.999", d: below, text: "99999:59:59.999"},
		{name: "100000:00:00.000", d: timecode.MaxTime, text: "100000:00:00.000", wantErr: `The time "100000:00:00.000" is not below 100000 hours.`},
		{name: "20 hour digits", d: timecode.MaxTime, text: "99999999999999999999:00:04.000", wantErr: `The time "99999999999999999999:00:04.000" is not below 100000 hours.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsing.BoundedTime(tt.d, tt.text, 6)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.d, got)
				return
			}
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{Line: 6, Msg: tt.wantErr}, *perr)
		})
	}
}

func TestBoundedField(t *testing.T) {
	t.Parallel()

	got, err := parsing.BoundedField(timecode.MaxTime-time.Millisecond, "cues[0].start")
	require.NoError(t, err)
	assert.Equal(t, timecode.MaxTime-time.Millisecond, got)

	_, err = parsing.BoundedField(timecode.MaxTime, "cues[1].start")
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, model.ParseError{Msg: "The field cues[1].start is not below 100000 hours."}, *perr)
}

func TestCheckWordTimestamps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lines   []string
		wantErr string
	}{
		{name: "no lines"},
		{name: "no timestamp", lines: []string{"The pool opens at seven."}},
		{name: "below the limit", lines: []string{"The pool <00:00:02.000>opens <99999:59:59.999>at seven."}},
		{name: "a short timestamp is no word timestamp", lines: []string{"<00:02.000>The pool"}},
		{
			name:    "100000 hours",
			lines:   []string{"The pool", "opens <100000:00:00.000>at seven."},
			wantErr: `The time "100000:00:00.000" is not below 100000 hours.`,
		},
		{
			name:    "20 hour digits",
			lines:   []string{"The pool opens <00:00:02.000>at <99999999999999999999:00:05.000>seven."},
			wantErr: `The time "99999999999999999999:00:05.000" is not below 100000 hours.`,
		},
		{
			name:    "the first bad timestamp",
			lines:   []string{"<100001:00:00.000>The <100000:00:00.000>pool"},
			wantErr: `The time "100001:00:00.000" is not below 100000 hours.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := parsing.CheckWordTimestamps(tt.lines, 7)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{Line: 7, Msg: tt.wantErr}, *perr)
		})
	}
}

func TestQuote(t *testing.T) {
	t.Parallel()

	sixty := strings.Repeat("abcdefghij", 6)
	tests := []struct {
		name string
		text string
		want string
	}{
		{"empty", "", `""`},
		{"short", "Downloaded from www.example.org", `"Downloaded from www.example.org"`},
		{"60 runes stay", sixty, `"` + sixty + `"`},
		{"61 runes keep 57", sixty + "k", `"` + sixty[:57] + `..."`},
		{"runes, not bytes", strings.Repeat("é", 60), `"` + strings.Repeat("é", 60) + `"`},
		{"61 runes of 2 bytes", strings.Repeat("é", 61), `"` + strings.Repeat("é", 57) + `..."`},
		{"quotes inside stay", `say "hi"`, `"say "hi""`},
		{"an invalid byte counts as 1 rune", "\xff" + sixty, `"` + "\xff" + sixty[:56] + `..."`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parsing.Quote(tt.text))
		})
	}
}

func TestAddParsed(t *testing.T) {
	t.Parallel()

	s := time.Second
	cue := func(start time.Duration, text string) model.Cue {
		return model.Cue{Start: start, End: start + s, Lines: []string{text}}
	}
	tests := []struct {
		name         string
		cues         []model.Cue
		comments     []parsing.ParsedComment
		wantTexts    []string
		wantComments []model.Comment
	}{
		{
			name:         "in order",
			cues:         []model.Cue{cue(1*s, "a"), cue(2*s, "b")},
			comments:     []parsing.ParsedComment{{Text: "before a", CuesBefore: 0}, {Text: "before b", CuesBefore: 1}, {Text: "end", CuesBefore: 2}},
			wantTexts:    []string{"a", "b"},
			wantComments: []model.Comment{{Text: "before a", BeforeCue: 0}, {Text: "before b", BeforeCue: 1}, {Text: "end", BeforeCue: 2}},
		},
		{
			name:         "a comment follows its cue when the cues sort",
			cues:         []model.Cue{cue(5*s, "late"), cue(1*s, "early"), cue(9*s, "last")},
			comments:     []parsing.ParsedComment{{Text: "before early", CuesBefore: 1}, {Text: "before late", CuesBefore: 0}},
			wantTexts:    []string{"early", "late", "last"},
			wantComments: []model.Comment{{Text: "before early", BeforeCue: 0}, {Text: "before late", BeforeCue: 1}},
		},
		{
			name:         "same starts keep the file order",
			cues:         []model.Cue{cue(2*s, "a"), cue(1*s, "b"), cue(2*s, "c")},
			comments:     []parsing.ParsedComment{{Text: "before c", CuesBefore: 2}},
			wantTexts:    []string{"b", "a", "c"},
			wantComments: []model.Comment{{Text: "before c", BeforeCue: 2}},
		},
		{
			name:         "starts that round to the same millisecond keep the file order",
			cues:         []model.Cue{cue(1000400*time.Microsecond, "a"), cue(1000100*time.Microsecond, "b")},
			comments:     []parsing.ParsedComment{{Text: "before b", CuesBefore: 1}},
			wantTexts:    []string{"a", "b"},
			wantComments: []model.Comment{{Text: "before b", BeforeCue: 1}},
		},
		{
			name:         "comments only",
			comments:     []parsing.ParsedComment{{Text: "note", CuesBefore: 0}},
			wantComments: []model.Comment{{Text: "note", BeforeCue: 0}},
		},
		{name: "nothing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sub := &model.Subtitle{}
			parsing.AddParsed(sub, tt.cues, tt.comments)
			var texts []string
			for _, c := range sub.All() {
				texts = append(texts, c.Text())
			}
			assert.Equal(t, tt.wantTexts, texts)
			assert.Equal(t, tt.wantComments, sub.Comments())
		})
	}
}
