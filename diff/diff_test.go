package diff_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/diff"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	added   = diff.Added
	removed = diff.Removed
	text    = diff.TextChanged
	timing  = diff.TimingChanged
	both    = diff.TextAndTimingChanged
)

func TestRealFilesParse(t *testing.T) {
	t.Parallel()

	original, edited := load(t, "own_original.srt"), load(t, "own_edited.srt")
	require.Equal(t, 13, original.Len())
	require.Equal(t, 14, edited.Len())
	for _, tt := range []struct {
		name  string
		s     *model.Subtitle
		index int
		start time.Duration
		end   time.Duration
		text  string
	}{
		{"original first", original, 0, 1 * time.Second, 3500 * time.Millisecond, "Good morning, and welcome\nto the coastal train."},
		{"original last", original, 12, 40 * time.Second, 42 * time.Second, "Have a pleasant trip."},
		{"edited first", edited, 0, 1 * time.Second, 3500 * time.Millisecond, "Good morning, and welcome\nto the coastal train."},
		{"edited last", edited, 13, 40 * time.Second, 42 * time.Second, "Have a pleasant trip."},
	} {
		c, ok := tt.s.Cue(tt.index)
		require.True(t, ok, tt.name)
		assert.Equal(t, tt.start, c.Start, tt.name)
		assert.Equal(t, tt.end, c.End, tt.name)
		assert.Equal(t, tt.text, c.Text(), tt.name)
	}
}

func TestCompareRealFiles(t *testing.T) {
	t.Parallel()

	original, edited := load(t, "own_original.srt"), load(t, "own_edited.srt")
	differences, err := diff.Compare(original, edited, diff.Options{})
	require.NoError(t, err)

	assert.Equal(t, []want{
		{timing, 1, 1},
		{text, 3, 3},
		{both, 5, 5},
		{added, -1, 6},
		{removed, 7, -1},
		{both, 8, 8},
		{text, 9, 9},
		{text, 10, 10},
		{added, -1, 12},
	}, summarize(differences))

	oldCue, _ := original.Cue(3)
	newCue, _ := edited.Cue(3)
	assert.Equal(t, &oldCue, differences[1].Old)
	assert.Equal(t, &newCue, differences[1].New)
	assert.Nil(t, differences[3].Old)
	assert.Equal(t, "Cards and coins are fine.", differences[3].New.Text())
	assert.Equal(t, "Thank you.", differences[4].Old.Text())
	assert.Nil(t, differences[4].New)
}

func TestCompareReturnsCopies(t *testing.T) {
	t.Parallel()

	original, edited := load(t, "own_original.srt"), load(t, "own_edited.srt")
	differences, err := diff.Compare(original, edited, diff.Options{})
	require.NoError(t, err)

	differences[1].Old.Lines[0] = "changed"
	differences[1].New.Lines[0] = "changed"
	differences[1].Old.Start = time.Hour

	oldCue, _ := original.Cue(3)
	newCue, _ := edited.Cue(3)
	assert.Equal(t, "The next stop is Milbrook.", oldCue.Text())
	assert.Equal(t, "The next stop is Millbrook.", newCue.Text())
	assert.Equal(t, 10500*time.Millisecond, oldCue.Start)
}

func TestCompareLeavesTheInputsUnchanged(t *testing.T) {
	t.Parallel()

	original, edited := load(t, "own_original.srt"), load(t, "own_edited.srt")
	beforeOld, beforeNew := original.Cues(), edited.Cues()

	_, err := diff.Compare(original, edited, diff.Options{IgnoreFormatting: true, IgnoreWhitespace: true})
	require.NoError(t, err)

	assert.Equal(t, beforeOld, original.Cues())
	assert.Equal(t, beforeNew, edited.Cues())
}

func TestTextReportMatchesExpectedFile(t *testing.T) {
	t.Parallel()

	differences, err := diff.Compare(load(t, "own_original.srt"), load(t, "own_edited.srt"), diff.Options{})
	require.NoError(t, err)

	assert.Equal(t, string(testfiles.Read(t, "diff/own_report.txt")), diff.Text(differences))
	assert.Empty(t, diff.Text(nil))
}

func TestOptionsOnRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts diff.Options
		want []want
	}{
		{"ignore formatting", diff.Options{IgnoreFormatting: true}, []want{
			{timing, 1, 1}, {text, 3, 3}, {both, 5, 5}, {added, -1, 6}, {removed, 7, -1}, {both, 8, 8}, {text, 10, 10}, {added, -1, 12},
		}},
		{"ignore whitespace", diff.Options{IgnoreWhitespace: true}, []want{
			{timing, 1, 1}, {text, 3, 3}, {both, 5, 5}, {added, -1, 6}, {removed, 7, -1}, {both, 8, 8}, {text, 9, 9}, {added, -1, 12},
		}},
		{"text only", diff.Options{TextOnly: true}, []want{
			{text, 3, 3}, {text, 5, 5}, {added, -1, 6}, {removed, 7, -1}, {text, 8, 8}, {text, 9, 9}, {text, 10, 10}, {added, -1, 12},
		}},
		{"tolerance 250 ms", diff.Options{TimeTolerance: tolerance(250 * time.Millisecond)}, []want{
			{text, 3, 3}, {both, 5, 5}, {added, -1, 6}, {removed, 7, -1}, {both, 8, 8}, {text, 9, 9}, {text, 10, 10}, {added, -1, 12},
		}},
		{"tolerance 0", diff.Options{TimeTolerance: tolerance(0)}, []want{
			{timing, 1, 1},
			{text, 3, 3},
			{both, 5, 5},
			{added, -1, 6},
			{removed, 7, -1},
			{both, 8, 8},
			{both, 9, 9},
			{text, 10, 10},
			{added, -1, 12},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, compare(t, load(t, "own_original.srt"), load(t, "own_edited.srt"), tt.opts))
		})
	}
}

func TestCompareExamples(t *testing.T) {
	t.Parallel()

	longOld, longNew := make([]row, 0, 300), make([]row, 0, 299)
	for n := range 300 {
		longOld = append(longOld, row{float64(n * 3), float64(n*3 + 2), fmt.Sprintf("line number %d", n)})
		if n != 150 {
			longNew = append(longNew, row{float64(n * 3), float64(n*3 + 2), fmt.Sprintf("LINE NUMBER %d", n)})
		}
	}
	longWant := make([]want, 0, 300)
	for n := range 149 {
		longWant = append(longWant, want{text, n, n})
	}
	longWant = append(longWant, want{text, 149, 149}, want{removed, 150, -1})
	for n := 151; n < 300; n++ {
		longWant = append(longWant, want{text, n, n - 1})
	}

	tests := []struct {
		name         string
		old, updated []row
		opts         diff.Options
		want         []want
	}{
		{
			name:    "an added cue does not shift the later pairs",
			old:     []row{{1, 2, "The train is late."}, {3, 4, "The bakery is open."}},
			updated: []row{{0, 0.5, "Welcome."}, {1, 2, "The train is late."}, {3, 4, "The bakery is open."}},
			want:    []want{{added, -1, 0}},
		},
		{
			name:    "the same text without a time overlap pairs",
			old:     []row{{10, 12, "The bakery opens at six."}},
			updated: []row{{100, 102, "The bakery opens at six."}},
			want:    []want{{timing, 0, 0}},
		},
		{
			name:    "unrelated cues list the removed one before the added one",
			old:     []row{{1, 2, "Rain at noon."}, {5, 6, "The harbour is closed."}, {9, 10, "Goodbye."}},
			updated: []row{{1, 2, "Rain at noon."}, {7, 8, "Coffee is free today."}, {9, 10, "Goodbye."}},
			want:    []want{{removed, 1, -1}, {added, -1, 1}},
		},
		{
			name:    "an overlap under half of the shorter cue does not pair",
			old:     []row{{1, 3, "The train leaves now."}},
			updated: []row{{2.9, 5, "A different line."}},
			want:    []want{{removed, 0, -1}, {added, -1, 0}},
		},
		{
			name:    "a text with a likeness of 0.7 pairs",
			old:     []row{{1, 3, "abcdefghij"}},
			updated: []row{{10, 12, "abcdefgXYZ"}},
			want:    []want{{both, 0, 0}},
		},
		{
			name:    "a text with a likeness of 0.6 does not pair",
			old:     []row{{1, 3, "abcdefghij"}},
			updated: []row{{10, 12, "abcdefXYZW"}},
			want:    []want{{removed, 0, -1}, {added, -1, 0}},
		},
		{
			name:    "a text of 70 percent of the length pairs",
			old:     []row{{1, 3, "abcdefghij"}},
			updated: []row{{10, 12, "abcdefg"}},
			want:    []want{{both, 0, 0}},
		},
		{
			name:    "a text under 70 percent of the length does not pair",
			old:     []row{{1, 3, "abcdefghij"}},
			updated: []row{{10, 12, "abcdef"}},
			want:    []want{{removed, 0, -1}, {added, -1, 0}},
		},
		{
			name:    "cues at the same time with unrelated text pair",
			old:     []row{{1, 3, "The train leaves now."}},
			updated: []row{{1, 3, "Coffee is free today."}},
			want:    []want{{text, 0, 0}},
		},
		{
			name: "300 cues with a changed case and one removed cue", old: longOld, updated: longNew, want: longWant,
		},
		{name: "two empty subtitles", want: []want{}},
		{
			name: "only added cues", updated: []row{{1, 2, "A"}, {3, 4, "B"}},
			want: []want{{added, -1, 0}, {added, -1, 1}},
		},
		{
			name: "only removed cues", old: []row{{1, 2, "A"}, {3, 4, "B"}},
			want: []want{{removed, 0, -1}, {removed, 1, -1}},
		},
		{
			name:    "the same cues give no difference",
			old:     []row{{1, 2, "A"}, {3, 4, "B"}},
			updated: []row{{1, 2, "A"}, {3, 4, "B"}},
			want:    []want{},
		},
		{
			name:    "a difference of exactly the tolerance is the same time",
			old:     []row{{1, 2, "A"}},
			updated: []row{{1.001, 2.001, "A"}},
			want:    []want{},
		},
		{
			name:    "a difference above the tolerance is a timing change",
			old:     []row{{1, 2, "A"}},
			updated: []row{{1.002, 2, "A"}},
			want:    []want{{timing, 0, 0}},
		},
		{
			name:    "text only hides the timing change",
			old:     []row{{1, 2, "A"}},
			updated: []row{{5, 6, "A"}},
			opts:    diff.Options{TextOnly: true},
			want:    []want{},
		},
		{
			name:    "text only keeps the text change",
			old:     []row{{1, 2, "Hello there."}},
			updated: []row{{5, 6, "Hello there!"}},
			opts:    diff.Options{TextOnly: true},
			want:    []want{{text, 0, 0}},
		},
		{
			name:    "two cues with the same start pair by text",
			old:     []row{{1, 2, "First line."}, {1, 2, "Second line."}},
			updated: []row{{1, 2, "First line."}, {1, 2, "Second line!"}},
			want:    []want{{text, 1, 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, compare(t, subtitle(tt.old...), subtitle(tt.updated...), tt.opts))
		})
	}
}

func TestCompareDoesNotPairNearlyEqualCuesWithoutTimeOverlap(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		shift    float64
		longCue  bool
		wantKind []diff.Kind
	}{
		{"4 s earlier, with a long cue", -4, true, []diff.Kind{removed, added}},
		{"4 s later, with a long cue", 4, true, []diff.Kind{removed, added}},
		{"4 s earlier", -4, false, []diff.Kind{removed, added}},
		{"4 s later", 4, false, []diff.Kind{removed, added}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var oldRows, newRows []row
			if tt.longCue {
				newRows = append(newRows, row{0, 6, "Opening title"})
			}
			for n := 1; n <= 250; n++ {
				oldRows = append(oldRows, row{float64(n * 10), float64(n*10 + 2), fmt.Sprintf("line number %d", n)})
				newRows = append(newRows, row{float64(n * 10), float64(n*10 + 2), fmt.Sprintf("LINE NUMBER %d", n)})
			}
			oldRows = append(oldRows, row{1005, 1007, "The ferry leaves at noon."})
			newRows = append(newRows, row{1005 + tt.shift, 1007 + tt.shift, "The ferry leaves at noon!"})

			differences, err := diff.Compare(subtitle(oldRows...), subtitle(newRows...), diff.Options{})
			require.NoError(t, err)

			var kinds []diff.Kind
			for _, d := range differences {
				if c := firstCue(d); len(c.Text()) >= 9 && c.Text()[:9] == "The ferry" {
					kinds = append(kinds, d.Kind)
				}
			}
			assert.Equal(t, tt.wantKind, kinds)
		})
	}
}

func firstCue(d diff.Difference) *model.Cue {
	if d.Old != nil {
		return d.Old
	}
	return d.New
}

func TestForcedFlagIsATextChange(t *testing.T) {
	t.Parallel()

	oldSub := subtitle(row{1, 2, "ENTER"}, row{3, 4, "EXIT"})
	newSub := subtitle(row{1, 2, "ENTER"}, row{3, 4, "EXIT"})
	c, _ := newSub.Cue(1)
	c.Forced = true
	require.NoError(t, newSub.SetCue(1, c))

	differences, err := diff.Compare(oldSub, newSub, diff.Options{})
	require.NoError(t, err)

	assert.Equal(t, []want{{text, 1, 1}}, summarize(differences))
	assert.Equal(t, "text changed: old cue 2, new cue 2\n"+
		"- 00:00:03.000 --> 00:00:04.000\n  EXIT\n"+
		"+ 00:00:03.000 --> 00:00:04.000 forced\n  EXIT\n", diff.Text(differences))
	equal, err := diff.Equal(newSub, newSub, diff.Options{})
	require.NoError(t, err)
	assert.True(t, equal)
}

func TestIgnoreFormattingDecodesEntities(t *testing.T) {
	t.Parallel()

	oldSub := subtitle(row{1, 2, "<i>Tom &amp; Jerry</i>"})
	newSub := subtitle(row{1, 2, "Tom & Jerry"})

	assert.Equal(t, []want{{text, 0, 0}}, compare(t, oldSub, newSub, diff.Options{}))
	assert.Empty(t, compare(t, oldSub, newSub, diff.Options{IgnoreFormatting: true}))
}

func TestIgnoreWhitespaceRemovesEveryWhitespaceRun(t *testing.T) {
	t.Parallel()

	oldSub := subtitle(row{1, 2, "a b\tc\fd\ve"})
	newSub := subtitle(row{1, 2, "abcde"})

	assert.Equal(t, []want{{text, 0, 0}}, compare(t, oldSub, newSub, diff.Options{}))
	assert.Empty(t, compare(t, oldSub, newSub, diff.Options{IgnoreWhitespace: true}))
}

func TestCompareLineBreakCountsAsText(t *testing.T) {
	t.Parallel()

	var oldSub, newSub model.Subtitle
	oldSub.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"one two", "three"}})
	newSub.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"one", "two three"}})

	assert.Equal(t, []want{{text, 0, 0}}, compare(t, &oldSub, &newSub, diff.Options{}))
	assert.Empty(t, compare(t, &oldSub, &newSub, diff.Options{IgnoreWhitespace: true}))
}

func TestCompareRepeatedTexts(t *testing.T) {
	t.Parallel()

	original, edited := load(t, "own_original.srt"), load(t, "own_edited.srt")
	copyOfBoth := compare(t, original, edited, diff.Options{})

	expected := make([]want, 0, len(copyOfBoth)*repeatedCopies)
	for n := range repeatedCopies {
		for _, w := range copyOfBoth {
			e := want{kind: w.kind, old: -1, new: -1}
			if w.old >= 0 {
				e.old = 1 + 13*n + w.old
			}
			if w.new >= 0 {
				e.new = 1 + 14*n + w.new
			}
			expected = append(expected, e)
		}
	}

	got := compare(t, repeatWithOverallCue(original), repeatWithOverallCue(edited), diff.Options{})
	assert.Equal(t, expected, got)
}

func TestCompareThreeThousandCuesWithTheSameText(t *testing.T) {
	t.Parallel()

	oldSub, newSub := musicCues(3000)
	got := compare(t, oldSub, newSub, diff.Options{})

	expected := make([]want, 3000)
	for n := range expected {
		expected[n] = want{timing, n, n}
	}
	assert.Equal(t, expected, got)
}

// repeatedCopies is how often repeatWithOverallCue repeats a subtitle.
const repeatedCopies = 400

// repeatWithOverallCue repeats the cues of s every 45 s and adds one cue over the whole file before them.
func repeatWithOverallCue(s *model.Subtitle) *model.Subtitle {
	var out model.Subtitle
	out.AddCue(model.Cue{End: time.Duration(45*repeatedCopies) * time.Second, Lines: []string{"Coastal train tour"}})
	for n := range repeatedCopies {
		for _, c := range s.Cues() {
			offset := time.Duration(45*n) * time.Second
			c.Start += offset
			c.End += offset
			out.AddCue(c)
		}
	}
	return &out
}

func musicCues(n int) (oldSub, newSub *model.Subtitle) {
	oldSub, newSub = &model.Subtitle{}, &model.Subtitle{}
	for i := range n {
		start := time.Duration(3*i) * time.Second
		oldSub.AddCue(model.Cue{Start: start, End: start + 2*time.Second, Lines: []string{"[MUSIC]"}})
		newSub.AddCue(model.Cue{Start: start + 300*time.Millisecond, End: start + 2300*time.Millisecond, Lines: []string{"[MUSIC]"}})
	}
	return oldSub, newSub
}

func TestEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b *model.Subtitle
		opts diff.Options
		want bool
	}{
		{"the same file twice", load(t, "own_original.srt"), load(t, "own_original.srt"), diff.Options{}, true},
		{"two files that differ", load(t, "own_original.srt"), load(t, "own_edited.srt"), diff.Options{}, false},
		{"two empty subtitles", &model.Subtitle{}, &model.Subtitle{}, diff.Options{}, true},
		{"empty and not empty", &model.Subtitle{}, load(t, "own_original.srt"), diff.Options{}, false},
		{"not empty and empty", load(t, "own_original.srt"), &model.Subtitle{}, diff.Options{}, false},
		{"a timing change that text only ignores", subtitle(row{1, 2, "A"}), subtitle(row{3, 4, "A"}), diff.Options{TextOnly: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := diff.Equal(tt.a, tt.b, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    diff.Options
		wantMsg string
	}{
		{"zero value", diff.Options{}, ""},
		{"zero tolerance", diff.Options{TimeTolerance: tolerance(0)}, ""},
		{"large tolerance", diff.Options{TimeTolerance: tolerance(time.Hour)}, ""},
		{"negative tolerance", diff.Options{TimeTolerance: tolerance(-100 * time.Millisecond)}, "The time tolerance must not be negative, got -0.1."},
		{"negative tolerance of one millisecond", diff.Options{TimeTolerance: tolerance(-time.Millisecond)}, "The time tolerance must not be negative, got -0.001."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.wantMsg == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var iae *model.InvalidArgumentError
			require.ErrorAs(t, err, &iae)
			assert.Equal(t, "diff.Options.TimeTolerance", iae.Option)
			assert.Equal(t, tt.wantMsg, iae.Msg)

			_, err = diff.Compare(&model.Subtitle{}, &model.Subtitle{}, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			_, err = diff.Equal(&model.Subtitle{}, &model.Subtitle{}, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
}

func TestText(t *testing.T) {
	t.Parallel()

	oldCue := model.Cue{Start: -1500 * time.Millisecond, End: 3661001 * time.Millisecond, Lines: []string{"a", "b"}, Forced: true}
	newCue := model.Cue{Start: 0, End: time.Second}
	tests := []struct {
		name string
		in   []diff.Difference
		want string
	}{
		{"no difference", nil, ""},
		{
			"removed",
			[]diff.Difference{{Kind: diff.Removed, OldIndex: 4, NewIndex: -1, Old: &oldCue}},
			"removed: old cue 5\n- -00:00:01.500 --> 01:01:01.001 forced\n  a\n  b\n",
		},
		{
			"added without text",
			[]diff.Difference{{Kind: diff.Added, OldIndex: -1, NewIndex: 0, New: &newCue}},
			"added: new cue 1\n+ 00:00:00.000 --> 00:00:01.000\n",
		},
		{"two blocks", []diff.Difference{
			{Kind: diff.Added, OldIndex: -1, NewIndex: 0, New: &newCue},
			{Kind: diff.TextChanged, OldIndex: 1, NewIndex: 2, Old: &newCue, New: &newCue},
		}, "added: new cue 1\n+ 00:00:00.000 --> 00:00:01.000\n\n" +
			"text changed: old cue 2, new cue 3\n- 00:00:00.000 --> 00:00:01.000\n+ 00:00:00.000 --> 00:00:01.000\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, diff.Text(tt.in))
		})
	}
}

func TestKindValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"added", "removed", "text changed", "timing changed", "text and timing changed"},
		[]string{string(diff.Added), string(diff.Removed), string(diff.TextChanged), string(diff.TimingChanged), string(diff.TextAndTimingChanged)})
}
