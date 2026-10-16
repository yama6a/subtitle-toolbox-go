package stats_test

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stats"
)

const delta = 1e-9

func at(seconds float64) time.Duration {
	return time.Duration(math.Round(seconds*1000)) * time.Millisecond
}

func cue(start, end float64, lines ...string) model.Cue {
	return model.Cue{Start: at(start), End: at(end), Lines: lines}
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	var s model.Subtitle
	s.AddCues(cues...)
	return &s
}

func ptr[T any](v T) *T { return &v }

func load(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{})
	require.NoError(t, err)
	return s
}

// textCases reads one case of testdata/statistics/texts.json.
func textCases(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	var all map[string][]struct {
		Start float64  `json:"start"`
		End   float64  `json:"end"`
		Lines []string `json:"lines"`
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "statistics/texts.json"), &all))
	var s model.Subtitle
	for _, c := range all[name] {
		s.AddCue(cue(c.Start, c.End, c.Lines...))
	}
	return &s
}

func assertRange(t *testing.T, want stats.Range, got *stats.Range) {
	t.Helper()

	require.NotNil(t, got)
	assert.InDelta(t, want.Min, got.Min, delta, "min")
	assert.InDelta(t, want.Average, got.Average, delta, "average")
	assert.InDelta(t, want.Max, got.Max, delta, "max")
}

func assertDurationRange(t *testing.T, want, got *stats.DurationRange) {
	t.Helper()

	require.NotNil(t, got)
	assert.Equal(t, want.Min, got.Min, "min")
	assert.Equal(t, want.Max, got.Max, "max")
	assert.InDelta(t, float64(want.Average), float64(got.Average), 1, "average")
}

func words(pairs ...any) []stats.WordCount {
	var out []stats.WordCount
	for pair := range slices.Chunk(pairs, 2) {
		out = append(out, stats.WordCount{Word: pair[0].(string), Count: pair[1].(int)})
	}
	return out
}

func TestOfFourCueFile(t *testing.T) {
	t.Parallel()

	got := stats.Of(load(t, "statistics/own_bakery.srt"))

	assert.Equal(t, 4, got.CueCount)
	assert.Equal(t, 26, got.WordCount)
	assert.Equal(t, 123, got.CharacterCount)
	assert.Equal(t, 9*time.Second, got.TotalDisplayTime)
	assert.Equal(t, ptr(11*time.Second), got.Span)
	assertRange(t, stats.Range{Min: 38 / 3.5, Average: (12 + 18.4 + 15 + 38/3.5) / 4, Max: 18.4}, got.CharactersPerSecond)
	assertRange(t, stats.Range{Min: 8 / 3.5 * 60, Average: (150 + 240 + 180 + 8/3.5*60) / 4, Max: 240}, got.WordsPerMinute)
	assertRange(t, stats.Range{Min: 15, Average: 20.5, Max: 26}, got.CharactersPerLine)
	assertDurationRange(t, &stats.DurationRange{Min: -500 * time.Millisecond, Average: time.Second * 2 / 3, Max: 2 * time.Second}, got.Gaps)
	assert.Equal(t, words("the", 4, "bread", 3, "bakery", 2, "at", 2), got.MostUsedWords[:4])
}

func TestOfCountsCharactersAsReadersSeeThem(t *testing.T) {
	t.Parallel()

	got := stats.Of(load(t, "statistics/own_combining_marks.srt"))

	assert.Equal(t, 32+23+18+26+43, got.CharacterCount)
	assertRange(t, stats.Range{Min: 18, Average: 28.4, Max: 43}, got.CharactersPerLine)
}

func TestOfWithoutCues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, stats.Statistics{}, stats.Of(&model.Subtitle{}))
}

func TestOfOneCueWithoutText(t *testing.T) {
	t.Parallel()

	got := stats.Of(subtitle(cue(1, 3, "<i> </i>")))

	assert.Equal(t, 1, got.CueCount)
	assert.Equal(t, 2*time.Second, got.TotalDisplayTime)
	assert.Equal(t, ptr(2*time.Second), got.Span)
	assert.Nil(t, got.Gaps)
	assert.Nil(t, got.CharactersPerSecond)
	assert.Nil(t, got.WordsPerMinute)
	assert.Nil(t, got.CharactersPerLine)
	assert.Empty(t, got.MostUsedWords)
}

func TestOfCuesWithoutTextCountOnlyAsCuesAndInTimes(t *testing.T) {
	t.Parallel()

	got := stats.Of(subtitle(cue(1, 3, "One two"), cue(9, 10, "<i> </i>"), cue(4, 8)))

	assert.Equal(t, 3, got.CueCount)
	assert.Equal(t, 2, got.WordCount)
	assert.Equal(t, 7, got.CharacterCount)
	assert.Equal(t, 7*time.Second, got.TotalDisplayTime)
	assert.Equal(t, ptr(9*time.Second), got.Span)
	assertRange(t, stats.Range{Min: 3.5, Average: 3.5, Max: 3.5}, got.CharactersPerSecond)
	assertRange(t, stats.Range{Min: 60, Average: 60, Max: 60}, got.WordsPerMinute)
	assertRange(t, stats.Range{Min: 7, Average: 7, Max: 7}, got.CharactersPerLine)
	assertDurationRange(t, &stats.DurationRange{Min: time.Second, Average: time.Second, Max: time.Second}, got.Gaps)
}

func TestOfCueWithoutDurationHasNoReadingSpeed(t *testing.T) {
	t.Parallel()

	got := stats.Of(subtitle(cue(1, 1, "Rain"), cue(2, 4, "Sun today")))

	assert.Equal(t, 13, got.CharacterCount)
	assertRange(t, stats.Range{Min: 4.5, Average: 4.5, Max: 4.5}, got.CharactersPerSecond)
	assertRange(t, stats.Range{Min: 60, Average: 60, Max: 60}, got.WordsPerMinute)
}

func TestOfCueThatEndsBeforeItStarts(t *testing.T) {
	t.Parallel()

	got := stats.Of(subtitle(cue(1, 3, "A"), cue(5, 4, "B"), cue(6, 7, "C")))

	assert.Equal(t, 2*time.Second, got.TotalDisplayTime)
	assert.Equal(t, ptr(6*time.Second), got.Span)
	assertRange(t, stats.Range{Min: 0.5, Average: 0.75, Max: 1}, got.CharactersPerSecond)
	assertDurationRange(t, &stats.DurationRange{Min: 2 * time.Second, Average: 2 * time.Second, Max: 2 * time.Second}, got.Gaps)
}

func TestOfGapCountsFromTheLatestEndOfTheEarlierCues(t *testing.T) {
	t.Parallel()

	got := stats.Of(subtitle(cue(0, 10, "A"), cue(2, 3, "B"), cue(12, 13, "C")))

	assertDurationRange(t, &stats.DurationRange{Min: -8 * time.Second, Average: -3 * time.Second, Max: 2 * time.Second}, got.Gaps)
}

func TestOfLinesAndWordsOfPortedPHPTests(t *testing.T) {
	t.Parallel()

	t.Run("visible characters", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(textCases(t, "visibleCharacters"))

		assert.Equal(t, 13, got.CharacterCount)
		assert.Equal(t, 4, got.WordCount)
		assertRange(t, stats.Range{Min: 2, Average: 6.5, Max: 11}, got.CharactersPerLine)
	})
	t.Run("most used words", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(textCases(t, "mostUsedWords"))

		assert.Equal(t, 10, got.WordCount)
		assert.Equal(t, words("\u00e4pfel", 2, "apples", 2, "don't", 2, "stop", 1, "12:30", 1), got.MostUsedWords)
	})
}

func TestOfMostUsedWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lines     []string
		wordCount int
		want      []stats.WordCount
	}{
		{"digits stay a string", []string{"2024 2024"}, 2, words("2024", 2)},
		{"ties keep the order of first use", []string{"b a c a b d"}, 6, words("b", 2, "a", 2, "c", 1, "d", 1)},
		{"a word of punctuation counts only as a word", []string{"-- Hello ... !!"}, 4, words("hello", 1)},
		{"symbols are stripped", []string{"$5 (a+b)= \u00a9Tee\u2122"}, 3, words("5", 1, "a+b", 1, "tee", 1)},
		{"the inside of a word stays", []string{"R&B don't e-mail"}, 3, words("r&b", 1, "don't", 1, "e-mail", 1)},
		{"tags are no words", []string{"<i>Hi</i> <b>hi</b>"}, 2, words("hi", 2)},
		{"an entity is decoded", []string{"AT&amp;T at&amp;t"}, 2, words("at&t", 2)},
		{"lines join with a space", []string{"Hello", "hello"}, 2, words("hello", 2)},
		{"no break space and ideographic space split", []string{"one\u00a0two\u3000three\u2028four"}, 4, words("one", 1, "two", 1, "three", 1, "four", 1)},
		{"mongolian vowel separator does not split", []string{"one\u180etwo"}, 1, words("one\u180etwo", 1)},
		{"zero width space does not split", []string{"one\u200btwo"}, 1, words("one\u200btwo", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := stats.Of(subtitle(cue(0, 2, tt.lines...)))

			assert.Equal(t, tt.wordCount, got.WordCount)
			assert.Equal(t, tt.want, got.MostUsedWords)
		})
	}
}

func TestOfLowerCaseIsTheGoMapping(t *testing.T) {
	t.Parallel()

	got := stats.Of(textCases(t, "lowerCase"))

	assert.Equal(t, words("istanbul", 1, "\u03bf\u03b4\u03bf\u03c3", 1), got.MostUsedWords)
}

func TestOfInvalidUTF8(t *testing.T) {
	t.Parallel()

	t.Run("bytes", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(subtitle(cue(1, 3, "Caf\xe9 au lait")))

		assert.Equal(t, 1, got.CueCount)
		assert.Equal(t, 3, got.WordCount)
		assert.Equal(t, 12, got.CharacterCount)
		assert.Equal(t, words("caf\xe9", 1, "au", 1, "lait", 1), got.MostUsedWords)
	})
	t.Run("white space is ASCII only", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(subtitle(cue(1, 3, "a\xe9\u00a0b c\td")))

		assert.Equal(t, 3, got.WordCount)
	})
	t.Run("a valid word of an invalid text gets the full mapping", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(subtitle(cue(1, 3, "\"Caf\u00e9!\" \xe9 \u00dcn\u00ef")))

		assert.Equal(t, words("caf\u00e9", 1, "\xe9", 1, "\u00fcn\u00ef", 1), got.MostUsedWords)
	})
	t.Run("ASCII punctuation and ASCII letters only", func(t *testing.T) {
		t.Parallel()

		got := stats.Of(subtitle(cue(1, 3, "\"CAF\xc9!\" \u00a1\xc9\u00c9")))

		assert.Equal(t, words("caf\xc9", 1, "\u00a1\xc9\u00c9", 1), got.MostUsedWords)
	})
}

func TestOfDoesNotChangeTheSubtitle(t *testing.T) {
	t.Parallel()

	s := load(t, "statistics/own_bakery.srt")
	before := s.Clone()

	stats.Of(s)

	assert.Equal(t, before.Cues(), s.Cues())
}

func TestOfRepeatedFifteenThousandTimesUnderFiveSeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("speed test")
	}
	t.Parallel()

	s := repeated(t, 15000)

	start := time.Now()
	got := stats.Of(s)

	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Equal(t, []int{60000, 26 * 15000, 123 * 15000}, []int{got.CueCount, got.WordCount, got.CharacterCount})
	assertRange(t, stats.Range{Min: 15, Average: 20.5, Max: 26}, got.CharactersPerLine)
}

func repeated(tb testing.TB, copies int) *model.Subtitle {
	tb.Helper()

	s, _, err := stbox.Load(testfiles.Path(tb, "statistics/own_bakery.srt"), stbox.ReadOptions{})
	require.NoError(tb, err)
	var out model.Subtitle
	for i := range copies {
		for _, c := range s.All() {
			c.Start += time.Duration(i) * 12 * time.Second
			c.End += time.Duration(i) * 12 * time.Second
			out.AddCue(c)
		}
	}
	return &out
}

func BenchmarkOf(b *testing.B) {
	for _, copies := range []int{1, 1000, 15000} {
		s := repeated(b, copies)
		b.Run("cues="+strconv.Itoa(s.Len()), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				stats.Of(s)
			}
		})
	}
}
