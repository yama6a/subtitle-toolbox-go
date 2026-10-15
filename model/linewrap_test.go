package model_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func cueLines(s *model.Subtitle) [][]string {
	out := [][]string{}
	for _, c := range s.All() {
		out = append(out, c.Lines)
	}
	return out
}

func oneCue(lines ...string) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: lines})
	return s
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestWrapLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lines    []string
		cpl, max int
		want     []string
	}{
		{
			name: "2 balanced lines", lines: []string{"This is a very long subtitle line that does not fit on the screen"}, cpl: 42, max: 2,
			want: []string{"This is a very long subtitle line", "that does not fit on the screen"},
		},
		{name: "a cue that fits stays", lines: []string{"Short line,", "and another one"}, cpl: 42, max: 2, want: []string{"Short line,", "and another one"}},
		{
			name: "a cue with too many lines joins", lines: []string{"One", "two three", "four", "five six"}, cpl: 15, max: 2,
			want: []string{"One two three", "four five six"},
		},
		{
			name: "3 lines when 2 do not fit", lines: []string{"one two three four five six seven eight"}, cpl: 15, max: 3,
			want: []string{"one two three", "four five six", "seven eight"},
		},
		{
			name: "the least overflow when the text does not fit", lines: []string{"one two three four five six seven eight"}, cpl: 10, max: 2,
			want: []string{"one two three four", "five six seven eight"},
		},
		{name: "a word longer than the line stays", lines: []string{"Donaudampfschifffahrtsgesellschaft"}, cpl: 10, max: 2, want: []string{"Donaudampfschifffahrtsgesellschaft"}},
		{
			name: "tags close and open at the break", lines: []string{"<i>This is a very long <b>subtitle line</b> that does not fit</i> on the screen"}, cpl: 42, max: 2,
			want: []string{"<i>This is a very long <b>subtitle line</b></i>", "<i>that does not fit</i> on the screen"},
		},
		{
			name: "nested tags close and open", lines: []string{"<i><b>This is a very long subtitle line that does not fit on the screen</b></i>"}, cpl: 42, max: 2,
			want: []string{"<i><b>This is a very long subtitle line</b></i>", "<i><b>that does not fit on the screen</b></i>"},
		},
		{
			name: "no break inside a tag", lines: []string{`<font color="#ff0000">Red text that is long enough to need a break somewhere</font>`}, cpl: 30, max: 2,
			want: []string{`<font color="#ff0000">Red text that is long enough</font>`, `<font color="#ff0000">to need a break somewhere</font>`},
		},
		{
			name: "a voice tag", lines: []string{"<v Fred>Hello there my friend, how are you doing today"}, cpl: 30, max: 2,
			want: []string{"<v Fred>Hello there my friend,</v>", "<v Fred>how are you doing today"},
		},
		{
			name: "tags count 0 and entities 1", lines: []string{"<b>Tom</b> &amp; <i>Jerry</i> &lt;3 <00:00:01.500>coffee"}, cpl: 21, max: 2,
			want: []string{"<b>Tom</b> &amp; <i>Jerry</i> &lt;3 <00:00:01.500>coffee"},
		},
		{name: "entities count 1", lines: []string{"Tom &amp; Jerry &lt;3 coffee"}, cpl: 20, max: 2, want: []string{"Tom &amp; Jerry", "&lt;3 coffee"}},
		{name: "multibyte characters count once", lines: []string{"Größere Äpfel überqueren übermütig"}, cpl: 34, max: 2, want: []string{"Größere Äpfel überqueren übermütig"}},
		{
			name: "multibyte characters on 2 lines", lines: []string{"Größere Äpfel überqueren übermütig die Straße, während Bäume grünen."}, cpl: 42, max: 2,
			want: []string{"Größere Äpfel überqueren übermütig", "die Straße, während Bäume grünen."},
		},
		{name: "invalid UTF-8 counts bytes", lines: []string{"caf\xe9 au lait"}, cpl: 8, max: 2, want: []string{"caf\xe9", "au lait"}},
		{
			name: "each dialogue turn on lines of its own", lines: []string{"- Are you coming with us to the coast tonight?", "- Yes."}, cpl: 42, max: 2,
			want: []string{"- Are you coming with", "us to the coast tonight?", "- Yes."},
		},
		{name: "3 dialogue lines stay", lines: []string{"- Are you coming?", "- Yes.", "- Me too."}, cpl: 42, max: 2, want: []string{"- Are you coming?", "- Yes.", "- Me too."}},
		{
			name: "a first line without a dash", lines: []string{"Are you coming?", "- Yes, in a minute, I promise you."}, cpl: 20, max: 2,
			want: []string{"Are you coming?", "- Yes, in a minute,", "I promise you."},
		},
		{
			name: "dialogue turns inside a line", lines: []string{"<i>- Are you coming? - Yes, in a minute, I promise you.</i>"}, cpl: 42, max: 2,
			want: []string{"<i>- Are you coming?</i>", "<i>- Yes, in a minute, I promise you.</i>"},
		},
		{
			name: "a dash word at the end of a line starts a turn", lines: []string{"Are you coming? -Yes"}, cpl: 10, max: 2,
			want: []string{"Are you", "coming?", "-Yes"},
		},
		{
			name: "a minus sign starts no turn", lines: []string{"- It is very cold outside tonight.", "-20 degrees, they said on the radio."}, cpl: 42, max: 2,
			want: []string{"- It is very cold outside tonight.", "-20 degrees, they said on the radio."},
		},
		{
			name: "a dash in mid sentence starts no turn", lines: []string{"- We walked along the coast - all the way to the old harbour lighthouse."}, cpl: 42, max: 2,
			want: []string{"- We walked along the coast - all the", "way to the old harbour lighthouse."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := oneCue(tt.lines...)
			require.NoError(t, s.WrapLines(tt.cpl, tt.max))
			assert.Equal(t, [][]string{tt.want}, cueLines(s))
		})
	}
}

// Adapted from testWrapLinesWithZeroMaxCharsThrowsException in tests/FixesTest.php of subtitle-toolbox-php.
func TestWrapLinesLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cpl, max int
		want     string
	}{
		{name: "0 characters", cpl: 0, max: 2, want: "got 0 and 2."},
		{name: "0 lines", cpl: 42, max: 0, want: "got 42 and 0."},
		{name: "negative", cpl: -1, max: -5, want: "got -1 and -5."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := oneCue("This is a very long subtitle line that does not fit on the screen")
			err := s.WrapLines(tt.cpl, tt.max)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			require.EqualError(t, err, "stbox: invalid argument: The maximum characters per line and the maximum lines must be at least 1, "+tt.want)
			assert.Equal(t, [][]string{{"This is a very long subtitle line that does not fit on the screen"}}, cueLines(s))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestUnwrapLines(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 1 * time.Second, End: 2 * time.Second, Lines: []string{"<i>Please take a seat,</i>", "the meeting starts."}},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"One line"}},
		model.Cue{Start: 5 * time.Second, End: 6 * time.Second},
		model.Cue{Start: 7 * time.Second, End: 8 * time.Second, Lines: []string{"- Are you coming?", "- Yes, in a minute,", "I promise you."}},
		model.Cue{Start: 9 * time.Second, End: 10 * time.Second, Lines: []string{"Who is there?", "- Only me."}},
		model.Cue{Start: 11 * time.Second, End: 12 * time.Second, Lines: []string{"- It is very cold tonight.", "-20 degrees, they said."}},
	)

	s.UnwrapLines()

	assert.Equal(t, [][]string{
		{"<i>Please take a seat,</i> the meeting starts."},
		{"One line"},
		nil,
		{"- Are you coming?", "- Yes, in a minute, I promise you."},
		{"Who is there?", "- Only me."},
		{"- It is very cold tonight. -20 degrees, they said."},
	}, cueLines(s))
}

// Adapted from testWrapLinesSplitsDialogueTurnsWithinALine in tests/FixesTest.php of subtitle-toolbox-php.
func TestUnwrapThenWrapKeepsDialogueTurns(t *testing.T) {
	t.Parallel()

	s := oneCue("- Are you coming?", "- Yes, in a minute, I promise you.")
	s.UnwrapLines()
	require.NoError(t, s.WrapLines(42, 2))
	assert.Equal(t, [][]string{{"- Are you coming?", "- Yes, in a minute, I promise you."}}, cueLines(s))
}

// TestWrapLinesRows runs the Arabic, Hebrew and Japanese rows, which bidichk and gosmopolitan keep out of Go source.
func TestWrapLinesRows(t *testing.T) {
	t.Parallel()

	var rows []struct {
		Name     string   `json:"name"`
		Call     string   `json:"call"`
		Lines    []string `json:"lines"`
		CPL      int      `json:"cpl"`
		MaxLines int      `json:"maxLines"`
		Want     []string `json:"want"`
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "line-wrapping/wrap_lines.json"), &rows))
	require.NotEmpty(t, rows)
	for _, r := range rows {
		t.Run(r.Name, func(t *testing.T) {
			t.Parallel()

			s := oneCue(r.Lines...)
			switch r.Call {
			case "WrapLines":
				require.NoError(t, s.WrapLines(r.CPL, r.MaxLines))
			case "UnwrapLines":
				s.UnwrapLines()
			default:
				t.Fatalf("unknown call %q", r.Call)
			}
			assert.Equal(t, [][]string{r.Want}, cueLines(s))
		})
	}
}

func readSubRip(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	s, warnings, err := srt.Read(bytes.NewReader(testfiles.Read(tb, name)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(tb, err)
	require.Empty(tb, warnings)
	return s
}

// Adapted from tests/FixesTest.php and tests/TextTransformsTest.php of subtitle-toolbox-php.
func TestLineWrapFixtures(t *testing.T) {
	t.Parallel()

	wrap := func(cpl int) func(testing.TB, *model.Subtitle) {
		return func(tb testing.TB, s *model.Subtitle) { tb.Helper(); require.NoError(tb, s.WrapLines(cpl, 2)) }
	}
	tests := []struct {
		name     string
		in, want string
		edit     func(testing.TB, *model.Subtitle)
	}{
		{name: "dialogue turns keep their lines", in: "fixes/own_dialogue_dashes.srt", want: "fixes/own_dialogue_dashes_wrapped.srt", edit: wrap(42)},
		{
			name: "dialogue turns unwrap", in: "fixes/own_dialogue_turns_wrapped.srt", want: "fixes/own_dialogue_turns_unwrapped.srt",
			edit: func(_ testing.TB, s *model.Subtitle) { s.UnwrapLines() },
		},
		{name: "Simplified Chinese at the Netflix limit", in: "fixes/own_zh_long_lines.srt", want: "fixes/own_zh_long_lines_wrapped.srt", edit: wrap(16)},
		{name: "Japanese at the Netflix limit", in: "fixes/own_ja_long_lines.srt", want: "fixes/own_ja_long_lines_wrapped.srt", edit: wrap(13)},
		{name: "dialogue turns after CJK sentence ends", in: "fixes/own_cjk_dialogue.srt", want: "fixes/own_cjk_dialogue_wrapped.srt", edit: wrap(16)},
		{
			name: "right-to-left fix then wrap", in: "transforms/own_rtl_long.srt", want: "transforms/own_rtl_long_wrapped.srt",
			edit: func(tb testing.TB, s *model.Subtitle) {
				tb.Helper()
				s.FixRightToLeft()
				require.NoError(tb, s.WrapLines(20, 2))
			},
		},
		{
			name: "wrap then right-to-left fix", in: "transforms/own_rtl_long.srt", want: "transforms/own_rtl_long_wrapped.srt",
			edit: func(tb testing.TB, s *model.Subtitle) {
				tb.Helper()
				require.NoError(tb, s.WrapLines(20, 2))
				s.FixRightToLeft()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readSubRip(t, tt.in)
			tt.edit(t, s)
			assert.Equal(t, cueLines(readSubRip(t, tt.want)), cueLines(s))
		})
	}
}

// Adapted from testWrapLinesBreaksChineseAndJapaneseLinesOfARealFile in tests/FixesTest.php of subtitle-toolbox-php.
func TestUnwrapLinesUndoesTheCJKWrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cpl  int
	}{
		{name: "fixes/own_zh_long_lines.srt", cpl: 16},
		{name: "fixes/own_ja_long_lines.srt", cpl: 13},
	}
	for _, tt := range tests {
		t.Run(strings.TrimPrefix(tt.name, "fixes/"), func(t *testing.T) {
			t.Parallel()

			wrapped := readSubRip(t, tt.name)
			require.NoError(t, wrapped.WrapLines(tt.cpl, 2))
			wrapped.UnwrapLines()
			original := readSubRip(t, tt.name)
			original.UnwrapLines()
			assert.Equal(t, cueLines(original), cueLines(wrapped))
		})
	}
}
