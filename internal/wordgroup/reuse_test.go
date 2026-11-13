package wordgroup_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func src(text string, start int) wordgroup.SourceWord {
	s := ms(start)
	return wordgroup.SourceWord{Text: text, Start: &s, End: new(s + ms(100))}
}

func untimedSrc(text string) wordgroup.SourceWord { return wordgroup.SourceWord{Text: text} }

func TestReuse(t *testing.T) {
	t.Parallel()
	conf := 0.7
	beach := model.Cue{Lines: []string{"The beach was quiet."}, Words: []model.Word{
		{Text: "The", End: ms(200)}, {Text: "beach", Start: ms(240), End: ms(650)}, {Text: "quiet", Start: ms(950), End: ms(1200)},
	}}
	type want struct {
		source []int
		word   int // index in Cue.Words of Choice.Word, -1 for nil
		text   string
		start  int
		end    int
		conf   float64
	}
	tests := []struct {
		name      string
		cue       model.Cue
		source    []wordgroup.SourceWord
		sameTimes bool
		sameText  bool
		ok        bool
		want      []want
	}{
		{"words: reuse, D1 and the full stop rule", beach, []wordgroup.SourceWord{src("The", 0), src("beach", 240), src("quiet", 950)}, false, false, true, []want{
			{source: []int{0}, word: 0}, {text: "beach was", start: 240, end: 650, conf: 1, word: -1}, {source: []int{2}, word: 2},
		}},
		{"words: a run over a source word without start", model.Cue{Lines: []string{"Pay 15. now"}, Words: []model.Word{
			{Text: "Pay", End: ms(100)}, {Text: "now", Start: ms(900), End: ms(1000)},
		}}, []wordgroup.SourceWord{src("Pay", 0), untimedSrc("15."), src("now", 900)}, false, false, true, []want{
			{source: []int{0, 1}, word: -1}, {source: []int{2}, word: 1},
		}},
		{"words: the leading span matches a source word without start", model.Cue{Lines: []string{"Well, the end"}, Words: []model.Word{
			{Text: "the", Start: ms(500), End: ms(600)}, {Text: "end", Start: ms(700), End: ms(800)},
		}}, []wordgroup.SourceWord{untimedSrc("Well,"), src("the", 500), src("end", 700)}, false, false, true, []want{
			{source: []int{0}, word: -1}, {source: []int{1}, word: 0}, {source: []int{2}, word: 1},
		}},
		{"words: D2 a moved word keeps its confidence, a word on a source start gets 1.0", model.Cue{Lines: []string{"a b c"}, Words: []model.Word{
			{Text: "a", Start: ms(5000), End: ms(5100), Confidence: &conf}, {Text: "b", Start: ms(100), End: ms(200), Confidence: &conf}, {Text: "c", Start: ms(300), End: ms(400)},
		}}, []wordgroup.SourceWord{src("x", 100), src("c", 300)}, false, false, true, []want{
			{text: "a", start: 5000, end: 5100, conf: 0.7, word: -1}, {text: "b", start: 100, end: 200, conf: 1, word: -1}, {source: []int{1}, word: 2},
		}},
		{"words: two source words with one start, the second matches", model.Cue{Lines: []string{"x b"}, Words: []model.Word{
			{Text: "x", End: ms(10)}, {Text: "b", Start: ms(100), End: ms(200)},
		}}, []wordgroup.SourceWord{src("a", 100), src("b", 100)}, false, false, true, []want{
			{text: "x", start: 0, end: 10, conf: 1, word: -1}, {source: []int{1}, word: 1},
		}},
		{
			"words: the leading span without a match starts at the cue start",
			model.Cue{Start: ms(-50), Lines: []string{"hi a"}, Words: []model.Word{{Text: "a", Start: ms(100), End: ms(200)}}},
			nil, false, false, true,
			[]want{{text: "hi", start: 0, end: 100, conf: 1, word: -1}, {text: "a", start: 100, end: 200, conf: 1, word: -1}},
		},
		{"no words: different times", model.Cue{Lines: []string{"a"}}, []wordgroup.SourceWord{src("a", 0)}, false, true, false, nil},
		{"no words: no source", model.Cue{Lines: []string{"a"}}, nil, true, true, false, nil},
		{"no words: same text", model.Cue{Lines: []string{"a b"}}, []wordgroup.SourceWord{src("a", 0), src("b", 1)}, true, true, true, []want{{source: []int{0}, word: -1}, {source: []int{1}, word: -1}}},
		{"no words: other word count", model.Cue{Lines: []string{"a b c"}}, []wordgroup.SourceWord{src("a", 0), src("b", 1)}, true, false, false, nil},
		{
			"no words: an edited word keeps the times of its source word",
			model.Cue{Lines: []string{"The bakery opens at six."}},
			[]wordgroup.SourceWord{src("The", 0), src("bakery", 100), src("opens", 200), src("at", 300), {Text: "seven.", Start: new(ms(1320)), End: new(ms(2200))}},
			true, false, true,
			[]want{
				{source: []int{0}, word: -1},
				{source: []int{1}, word: -1},
				{source: []int{2}, word: -1},
				{source: []int{3}, word: -1},
				{text: "six.", start: 1320, end: 2200, conf: 1, word: -1},
			},
		},
		{
			"no words: a source word without times",
			model.Cue{Start: ms(-5), Lines: []string{"a z"}},
			[]wordgroup.SourceWord{src("a", 0), untimedSrc("b")},
			true, false, true,
			[]want{
				{source: []int{0}, word: -1}, {text: "z", start: 0, end: 0, conf: 1, word: -1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := wordgroup.Reuse(&tt.cue, tt.source, tt.sameTimes, tt.sameText)
			require.Equal(t, tt.ok, ok)
			require.Len(t, got, len(tt.want))
			for i, w := range tt.want {
				ch := got[i]
				if w.source != nil {
					assert.Equal(t, w.source, ch.Source, i)
					assert.Nil(t, ch.New, i)
				} else {
					require.NotNil(t, ch.New, i)
					assert.Equal(t, wordgroup.NewWord{Text: w.text, Start: ms(w.start), End: ms(w.end), Confidence: w.conf}, *ch.New, i)
				}
				if w.word < 0 {
					assert.Nil(t, ch.Word, i)
				} else {
					assert.Same(t, &tt.cue.Words[w.word], ch.Word, i)
				}
			}
		})
	}
}

func TestSameWords(t *testing.T) {
	t.Parallel()
	words := []model.Word{{Text: "a", End: ms(100)}, {Text: "b", Start: ms(200), End: ms(300)}}
	source := []wordgroup.SourceWord{src("a", 0), src("b", 200)}
	edited := model.Cue{Lines: []string{"a c"}, Words: words}
	tests := []struct {
		name string
		cue  model.Cue
		want bool
	}{
		{"no words", model.Cue{Lines: []string{"anything"}}, true},
		{"unchanged", model.Cue{Lines: []string{"a b"}, Words: words}, true},
		{"edited word", edited, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, wordgroup.SameWords(&tt.cue, source))
		})
	}
}

func bigCue(n int) (model.Cue, []wordgroup.SourceWord) {
	var b strings.Builder
	words := make([]model.Word, n)
	source := make([]wordgroup.SourceWord, n)
	for i := range n {
		text := fmt.Sprintf("w%d", i)
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(text)
		words[i] = model.Word{Text: text, Start: ms(i), End: ms(i + 1)}
		source[i] = src(text, i)
	}
	return model.Cue{Lines: []string{b.String()}, End: ms(n), Words: words}, source
}

// sharedStartCue has n source words at 0 s whose spans match none of them.
func sharedStartCue(n int) (model.Cue, []wordgroup.SourceWord) {
	words := make([]model.Word, n)
	source := make([]wordgroup.SourceWord, n)
	for i := range n {
		words[i] = model.Word{Text: "a", End: ms(1)}
		source[i] = src("a", 0)
	}
	return model.Cue{Lines: []string{strings.Repeat("a Z ", n)}, End: ms(n), Words: words}, source
}

// untimedLeadCue has n source words without a start and a leading span that none of their runs matches.
func untimedLeadCue(n int) (model.Cue, []wordgroup.SourceWord) {
	source := make([]wordgroup.SourceWord, n)
	for i := range n {
		source[i] = untimedSrc("a")
	}
	words := []model.Word{{Text: "Y", Start: ms(5), End: ms(6)}}
	return model.Cue{Lines: []string{strings.Repeat("a ", n) + "X Y"}, End: ms(n), Words: words}, source
}

var reuseShapes = []struct {
	name  string
	build func(n int) (model.Cue, []wordgroup.SourceWord)
	words int // the word count that makes the text about 1 MB
}{
	{"distinct starts", bigCue, 160000},
	{"one shared start", sharedStartCue, 250000},
	{"untimed leading span", untimedLeadCue, 500000},
}

func BenchmarkReuse(b *testing.B) {
	for _, shape := range reuseShapes {
		for _, n := range []int{10000, shape.words} {
			cue, source := shape.build(n)
			b.Run(fmt.Sprintf("%s/words=%d", shape.name, n), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(cue.Lines[0])))
				for b.Loop() {
					if got, _ := wordgroup.Reuse(&cue, source, true, true); len(got) == 0 {
						b.Fatal("no words")
					}
				}
			})
		}
	}
}

func TestWordgroupGrowth(t *testing.T) {
	t.Parallel()
	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("set STBOX_GROWTH=1 to run the growth tests")
	}
	best := func(f func()) time.Duration {
		d := time.Duration(1<<63 - 1)
		for range 3 {
			start := time.Now()
			f()
			d = min(d, time.Since(start))
		}
		return d
	}
	run := func(n int) map[string]time.Duration {
		cue, source := bigCue(n)
		words := make([]wordgroup.Word, n)
		for i := range n {
			words[i] = word(cue.Words[i].Text, i, i+1)
		}
		text := cue.Lines[0]
		shared, sharedSource := sharedStartCue(n)
		lead, leadSource := untimedLeadCue(5 * n) // this shape is fast, so a larger base keeps the ratio steady
		return map[string]time.Duration{
			"Reuse":                best(func() { wordgroup.Reuse(&cue, source, true, true) }),
			"Reuse shared start":   best(func() { wordgroup.Reuse(&shared, sharedSource, true, true) }),
			"Reuse untimed leader": best(func() { wordgroup.Reuse(&lead, leadSource, true, true) }),
			"Spans":                best(func() { wordgroup.Spans(&cue) }),
			"CueWords":             best(func() { wordgroup.CueWords(text, words, ms(n)) }),
			"Group":                best(func() { wordgroup.Group(words) }),
		}
	}
	small, large := run(10000), run(100000)
	for name, s := range small {
		ratio := float64(large[name]) / float64(max(s, time.Microsecond))
		t.Logf("%s: %v at 10000 words, %v at 100000, ratio %.1f", name, s, large[name], ratio)
		assert.LessOrEqual(t, ratio, 30.0, name)
	}
}
