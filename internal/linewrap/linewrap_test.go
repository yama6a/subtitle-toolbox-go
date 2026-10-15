package linewrap_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// Adapted from testTagsCountNoCharactersAndEntitiesCountOne in tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestMeasuredWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []linewrap.Word
	}{
		{
			name: "tags count 0 and an entity counts 1",
			text: "<i>Tom &amp;</i> Jerry",
			want: []linewrap.Word{{Text: "<i>Tom", Length: 3}, {Text: "&amp;</i>", Length: 1}, {Text: "Jerry", Length: 5}},
		},
		{
			name: "several spaces collapse",
			text: "  one   two ",
			want: []linewrap.Word{{Text: "one", Length: 3}, {Text: "two", Length: 3}},
		},
		{
			name: "a tab and U+00A0 stay inside a word",
			text: "a\tb\u00a0c d",
			want: []linewrap.Word{{Text: "a\tb\u00a0c", Length: 5}, {Text: "d", Length: 1}},
		},
		{
			name: "a space inside a tag does not split",
			text: `<font color="#ffff00">Tom</font>`,
			want: []linewrap.Word{{Text: `<font color="#ffff00">Tom</font>`, Length: 3}},
		},
		{
			name: "numeric entities and a lone ampersand",
			text: "&#233;&#xE9; & &amp",
			want: []linewrap.Word{{Text: "&#233;&#xE9;", Length: 2}, {Text: "&", Length: 1}, {Text: "&amp", Length: 4}},
		},
		{
			name: "invalid UTF-8 counts bytes",
			text: "caf\xe9 au",
			want: []linewrap.Word{{Text: "caf\xe9", Length: 4}, {Text: "au", Length: 2}},
		},
		{name: "no words", text: "   ", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.MeasuredWords(tt.text))
		})
	}
}

func TestLength(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 11, linewrap.Length(linewrap.MeasuredWords("<i>Tom &amp;</i> Jerry")))
	assert.Equal(t, 0, linewrap.Length(nil))
}

// Adapted from tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestWrap(t *testing.T) {
	t.Parallel()

	dialogue := []string{"- Are you coming?", "- Yes, I am on my way now."}
	tests := []struct {
		name     string
		lines    []string
		cpl, max int
		keep     bool
		want     []string
	}{
		{
			// testWrapsIntoLinesOfEqualLength
			name: "lines of equal length", lines: []string{"The bakery opens at seven tomorrow."}, cpl: 20, max: 2,
			want: []string{"The bakery opens", "at seven tomorrow."},
		},
		{
			// testUsesTheFewestLinesThatFit
			name: "the fewest lines that fit", lines: []string{"one two three four"}, cpl: 10, max: 3,
			want: []string{"one two", "three four"},
		},
		{
			// testKeepsAWordLongerThanTheLineWhole
			name: "a word longer than the line stays whole", lines: []string{"Supercalifragilisticexpialidocious is long"}, cpl: 10, max: 2,
			want: []string{"Supercalifragilisticexpialidocious", "is long"},
		},
		{
			// testClosesAndReopensItalicTagsAtTheLineBreak
			name: "italic tags close and open at the break", lines: []string{"<i>Are you coming to the party?</i>"}, cpl: 16, max: 2,
			want: []string{"<i>Are you coming</i>", "<i>to the party?</i>"},
		},
		{
			name: "nested tags close and open in order", lines: []string{"<i>This is a very long <b>subtitle line</b> that does not fit</i> on the screen"},
			cpl: 42, max: 2,
			want: []string{"<i>This is a very long <b>subtitle line</b></i>", "<i>that does not fit</i> on the screen"},
		},
		{
			// testJoinsDialogueLinesOnlyWhenAskedToKeepThem
			name: "dialogue lines stay with keepDialogueLines", lines: dialogue, cpl: 30, max: 2, keep: true,
			want: dialogue,
		},
		{
			name: "dialogue lines join without keepDialogueLines", lines: dialogue, cpl: 30, max: 2,
			want: []string{"- Are you coming? -", "Yes, I am on my way now."},
		},
		{
			name: "a minus sign starts a segment", lines: []string{"- Cold.", "-20 degrees."}, cpl: 30, max: 2, keep: true,
			want: []string{"- Cold.", "-20 degrees."},
		},
		{
			name: "a dash after white space and tags starts a segment", lines: []string{"Hi", "\u00a0<i>\u2014 there"}, cpl: 30, max: 2, keep: true,
			want: []string{"Hi", "\u00a0<i>\u2014 there"},
		},
		{
			name: "segments join their words with 1 space", lines: []string{"- One  two", "three", "- Four"}, cpl: 5, max: 1, keep: true,
			want: []string{"- One two three", "- Four"},
		},
		{
			// testReturnsNoLinesForEmptyText
			name: "no lines for empty text", lines: []string{"", " "}, cpl: 20, max: 2,
			want: nil,
		},
		{
			name: "a line of only tags", lines: []string{"<i></i>"}, cpl: 20, max: 2,
			want: []string{"<i></i>"},
		},
		{
			name: "a voice tag closes at the break", lines: []string{"<v Fred>Hello there my friend, how are you doing today"}, cpl: 30, max: 2,
			want: []string{"<v Fred>Hello there my friend,</v>", "<v Fred>how are you doing today"},
		},
		{
			name: "invalid UTF-8 counts bytes", lines: []string{"caf\xe9 au lait"}, cpl: 8, max: 2,
			want: []string{"caf\xe9", "au lait"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.Wrap(tt.lines, tt.cpl, tt.max, tt.keep))
		})
	}
}

// Adapted from tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestWrapToFit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
		ok    bool
	}{
		// testWrapToFitReturnsNullWhenDialogueLinesBreakALimit
		{name: "dialogue lines that fit", lines: []string{"- Hi.", "- Yes?"}, want: []string{"- Hi.", "- Yes?"}, ok: true},
		{name: "a dialogue line too long", lines: []string{"- Are you coming?", "- Yes, I am on my way now."}},
		{name: "too many dialogue lines", lines: []string{"- Hi.", "- Yes?", "- No."}},
		// testJoinsContinuedLinesBeforeWrapping
		{name: "continued lines join", lines: []string{"- We left", "early and came back late."}, want: []string{"- We left early", "and came back late."}, ok: true},
		// testKeepsAWordLongerThanTheLineWhole
		{name: "a word longer than the line", lines: []string{"Supercalifragilisticexpialidocious is long"}},
		// testReturnsNoLinesForEmptyText
		{name: "empty text", lines: []string{""}, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := linewrap.WrapToFit(tt.lines, 20, 2)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Adapted from tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestFits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lines    []string
		cpl, max int
		want     bool
	}{
		{name: "tags count 0", lines: []string{`<font color="#ffff00">Tom &amp; Jerry</font>`}, cpl: 11, max: 1, want: true},
		{name: "one character too many", lines: []string{`<font color="#ffff00">Tom &amp; Jerry</font>`}, cpl: 10, max: 1},
		{name: "too many lines", lines: []string{"a", "b", "c"}, cpl: 10, max: 2},
		{name: "a word longer than the line", lines: []string{"Supercalifragilisticexpialidocious", "is long"}, cpl: 10, max: 2},
		{name: "spaces collapse", lines: []string{"a   b"}, cpl: 3, max: 1, want: true},
		{name: "no lines", cpl: 1, max: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.Fits(tt.lines, tt.cpl, tt.max))
		})
	}
}

// Adapted from testFewestLinesIsTheLineCountOfWrapWithoutALineLimit in tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestFewestLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		cpl   int
		want  int
	}{
		{name: "words", lines: []string{"one two three four"}, cpl: 10, want: 2},
		{name: "a word longer than the line", lines: []string{"Supercalifragilisticexpialidocious is long"}, cpl: 10, want: 3},
		{name: "tags and entities", lines: []string{"<i>Tags count</i> no &amp; characters"}, cpl: 12, want: 3},
		{name: "dialogue lines join", lines: []string{"- First line", "- Second line"}, cpl: 32, want: 1},
		{name: "empty", lines: []string{""}, cpl: 32, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.FewestLines(tt.lines, tt.cpl))
		})
	}
}

// TestFewestLinesIsTheLineCountOfWrap checks FewestLines against Wrap without a line limit, for the PHP cases and
// for random ASCII and CJK text.
func TestFewestLinesIsTheLineCountOfWrap(t *testing.T) {
	t.Parallel()

	check := func(lines []string, cpl int) {
		want := len(linewrap.Wrap(lines, cpl, math.MaxInt, false))
		assert.Equal(t, want, linewrap.FewestLines(lines, cpl), "%q at %d", lines, cpl)
	}
	check([]string{"The bakery opens at seven tomorrow."}, 20)
	check([]string{"one two three four"}, 10)
	check([]string{"Supercalifragilisticexpialidocious is long"}, 10)
	check([]string{"<i>Tags count</i> no &amp; characters"}, 12)
	check([]string{"- First line", "- Second line"}, 32)
	check([]string{""}, 32)
	for _, r := range readRows(t, "wrap.json") {
		check(r.Lines, max(r.CPL, 1))
	}
	random := newRandomText(37)
	for range 200 {
		check([]string{random.text(30)}, 1+random.intn(20))
	}
}

// Adapted from tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestVisibleCharacters(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 4, linewrap.VisibleCharacters([]string{"<i>ab</i>", "c&amp;"}))
	assert.Equal(t, 0, linewrap.VisibleCharacters(nil))
	assert.Equal(t, []int{5, 3}, linewrap.VisibleLineLengths([]string{"<i>Hello</i>", "<b></b>", "", "a b"}))
	assert.Empty(t, linewrap.VisibleLineLengths([]string{"<b></b>"}))
}

// Adapted from testCharactersPerSecondIsInfiniteWithoutDuration in tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestCharactersPerSecond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     float64
	}{
		{name: "2 s", duration: 2 * time.Second, want: 10},
		{name: "0 s", duration: 0, want: math.Inf(1)},
		{name: "negative", duration: -time.Second, want: math.Inf(1)},
		{name: "500 ms", duration: 500 * time.Millisecond, want: 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.InDelta(t, tt.want, linewrap.CharactersPerSecond(20, tt.duration), 0)
		})
	}
}

// Adapted from testFitsCharactersPerSecond in tests/LineWrapperTest.php of subtitle-toolbox-php.
func TestFitsCharactersPerSecond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lines    []string
		duration time.Duration
		maxCPS   float64
		want     bool
	}{
		{name: "no limit", lines: []string{"<i>Hello</i>"}, duration: time.Second, want: true},
		{name: "at the limit", lines: []string{"<i>Hello</i>"}, duration: time.Second, maxCPS: 5, want: true},
		{name: "above the limit", lines: []string{"<i>Hello</i>"}, duration: time.Second, maxCPS: 4.9},
		{name: "no visible characters", lines: []string{"<i></i>"}, maxCPS: 1, want: true},
		{name: "no duration", lines: []string{"Hi"}, maxCPS: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.FitsCharactersPerSecond(tt.lines, tt.duration, tt.maxCPS))
		})
	}
}

func TestWrapTurns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		cpl   int
		want  []string
	}{
		{
			name: "each turn wraps on its own", lines: []string{"- Are you coming with us to the coast tonight?", "- Yes."}, cpl: 42,
			want: []string{"- Are you coming with", "us to the coast tonight?", "- Yes."},
		},
		{
			name: "a turn after a first line without a dash", lines: []string{"Are you coming?", "- Yes, in a minute, I promise you."}, cpl: 20,
			want: []string{"Are you coming?", "- Yes, in a minute,", "I promise you."},
		},
		{
			name: "a dash after a sentence end starts a turn", lines: []string{"<i>- Are you coming? - Yes, in a minute, I promise you.</i>"}, cpl: 42,
			want: []string{"<i>- Are you coming?</i>", "<i>- Yes, in a minute, I promise you.</i>"},
		},
		{
			name: "a minus sign starts no turn", lines: []string{"- It is very cold outside tonight.", "-20 degrees, they said on the radio."}, cpl: 42,
			want: []string{"- It is very cold outside tonight.", "-20 degrees, they said on the radio."},
		},
		{
			name: "a dash in mid sentence starts no turn", lines: []string{"- We walked along the coast - all the way to the old harbour lighthouse."}, cpl: 42,
			want: []string{"- We walked along the coast - all the", "way to the old harbour lighthouse."},
		},
		{
			name: "a dash at the end of a line starts no turn", lines: []string{"- Wait. -", "- What?"}, cpl: 42,
			want: []string{"- Wait. -", "- What?"},
		},
		{
			name: "the ellipsis character ends a sentence", lines: []string{"- Well\u2026 - Go on."}, cpl: 42,
			want: []string{"- Well\u2026", "- Go on."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.WrapTurns(tt.lines, tt.cpl, 2))
		})
	}
}

// Adapted from tests/FixesTest.php of subtitle-toolbox-php.
func TestUnwrapTurns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{name: "lines join", lines: []string{"<i>Please take a seat,</i>", "the meeting starts."}, want: []string{"<i>Please take a seat,</i> the meeting starts."}},
		{
			name:  "each turn keeps its line",
			lines: []string{"- Are you coming?", "- Yes, in a minute,", "I promise you."},
			want:  []string{"- Are you coming?", "- Yes, in a minute, I promise you."},
		},
		{name: "a first line without a dash", lines: []string{"Who is there?", "- Only me."}, want: []string{"Who is there?", "- Only me."}},
		{name: "a minus sign", lines: []string{"- It is very cold tonight.", "-20 degrees, they said."}, want: []string{"- It is very cold tonight. -20 degrees, they said."}},
		{name: "a dash inside a line stays", lines: []string{"- Hi. - Yes?", "Come in."}, want: []string{"- Hi. - Yes? Come in."}},
		{name: "no lines", lines: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, linewrap.UnwrapTurns(tt.lines))
		})
	}
}

func TestRightToLeftEmbedding(t *testing.T) {
	t.Parallel()

	const (
		rle = "\u202b"
		pdf = "\u202c"
	)
	tests := []struct {
		name string
		fn   func([]string) []string
		in   []string
		want []string
	}{
		{
			name: "wrap embeds each line",
			fn:   func(lines []string) []string { return linewrap.Wrap(lines, 6, 2, false) },
			in:   []string{rle + "\u05d0\u05d1\u05d2 \u05d3\u05d4\u05d5?" + pdf},
			want: []string{rle + "\u05d0\u05d1\u05d2" + pdf, rle + "\u05d3\u05d4\u05d5?" + pdf},
		},
		{
			name: "unwrap embeds the joined line",
			fn:   linewrap.UnwrapTurns,
			in:   []string{rle + "\u05d0\u05d1\u05d2" + pdf, rle + "\u05d3\u05d4\u05d5?" + pdf},
			want: []string{rle + "\u05d0\u05d1\u05d2 \u05d3\u05d4\u05d5?" + pdf},
		},
		{
			name: "lines without the embedding get none",
			fn:   func(lines []string) []string { return linewrap.Wrap(lines, 6, 2, false) },
			in:   []string{"\u05d0\u05d1\u05d2 \u05d3\u05d4\u05d5?"},
			want: []string{"\u05d0\u05d1\u05d2", "\u05d3\u05d4\u05d5?"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.fn(tt.in))
		})
	}
}

type row struct {
	Name      string   `json:"name"`
	Call      string   `json:"call"`
	Lines     []string `json:"lines"`
	CPL       int      `json:"cpl"`
	MaxLines  int      `json:"maxLines"`
	Want      []string `json:"want"`
	WantCount int      `json:"wantCount"`
}

// readRows reads a row table of testdata/line-wrapping. The text \xff in a row stands for the byte 0xFF.
func readRows(t *testing.T, name string) []row {
	t.Helper()

	var rows []row
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "line-wrapping/"+name), &rows))
	require.NotEmpty(t, rows)
	for i := range rows {
		rows[i].Lines = withInvalidBytes(rows[i].Lines)
		rows[i].Want = withInvalidBytes(rows[i].Want)
	}
	return rows
}

func withInvalidBytes(lines []string) []string {
	for i, line := range lines {
		lines[i] = strings.ReplaceAll(line, `\xff`, "\xff")
	}
	return lines
}

// TestRows runs the Chinese, Japanese and Korean rows, which gosmopolitan keeps out of Go source.
func TestRows(t *testing.T) {
	t.Parallel()

	for _, r := range readRows(t, "wrap.json") {
		t.Run(r.Name, func(t *testing.T) {
			t.Parallel()

			switch r.Call {
			case "Wrap":
				assert.Equal(t, r.Want, linewrap.Wrap(r.Lines, r.CPL, r.MaxLines, false))
			case "UnwrapTurns":
				assert.Equal(t, r.Want, linewrap.UnwrapTurns(r.Lines))
			case "VisibleCharacters":
				assert.Equal(t, r.WantCount, linewrap.VisibleCharacters(r.Lines))
			case "FewestLines":
				assert.Equal(t, r.WantCount, linewrap.FewestLines(r.Lines, r.CPL))
			default:
				t.Fatalf("unknown call %q", r.Call)
			}
		})
	}
}
