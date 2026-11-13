package wordgroup

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// SourceWord is a stored source word as a writer reads it.
type SourceWord struct {
	Text       string         // trimmed. An empty text matches no span
	Start      *time.Duration // rounded to whole milliseconds, nil when the stored word has no valid start
	End        *time.Duration
	Confidence *float64
}

// Choice is one entry of the words to write: a run of reused source words, or a new word.
type Choice struct {
	Source []int       // indexes of reused source words, in order
	Word   *model.Word // set for one reused source word that a span of Cue.Words matched. The writer writes its End and Confidence
	New    *NewWord    // set when Source is empty
}

// NewWord is a word that the writer builds in its own shape.
type NewWord struct {
	Text       string
	Start, End time.Duration
	Confidence float64
}

// Reuse returns the words to write for a cue, and false when the cue gets no words.
// sameTimes tells that the stored start and end equal the cue times. sameText tells that the stored text equals the
// cue text. A new word on the start of a source word gets confidence 1.0, any other new word the confidence of its
// Cue.Words entry, else 1.0.
func Reuse(c *model.Cue, stored []SourceWord, sameTimes, sameText bool) ([]Choice, bool) {
	source := foldSource(stored)
	if len(c.Words) > 0 {
		return reuseSpans(c, source), true
	}
	if !sameTimes || len(source) == 0 {
		return nil, false
	}
	if sameText {
		return keepAll(source), true
	}
	texts := strings.Split(CueText(c), " ")
	if len(texts) != len(source) {
		return nil, false
	}
	start := max(0, c.Start)
	choices := make([]Choice, len(texts))
	for i, t := range texts {
		if sameWordText(t, source[i].Text) {
			choices[i] = Choice{Source: []int{i}}
			continue
		}
		s := start
		if source[i].Start != nil {
			s = *source[i].Start
		}
		e := s
		if source[i].End != nil {
			e = *source[i].End
		}
		choices[i] = Choice{New: &NewWord{Text: t, Start: s, End: e, Confidence: 1}}
	}
	return choices, true
}

// foldSource returns a copy of source with the space folding of PlainText in each text, so a stored word with a
// no-break space matches the cue text. The indexes stay the same.
func foldSource(source []SourceWord) []SourceWord {
	out := make([]SourceWord, len(source))
	for i, s := range source {
		s.Text = foldSpaces(s.Text)
		out[i] = s
	}
	return out
}

func keepAll(source []SourceWord) []Choice {
	choices := make([]Choice, len(source))
	for i := range source {
		choices[i] = Choice{Source: []int{i}}
	}
	return choices
}

// reuseSpans matches each span to the first source word that can match it. A source word matches only a span with
// its own start, so an index by start finds the candidates of a span without a scan of all source words.
func reuseSpans(c *model.Cue, source []SourceWord) []Choice {
	starts := map[time.Duration]bool{}
	for _, s := range source {
		if s.Start != nil {
			starts[s.Start.Round(time.Millisecond)] = true
		}
	}
	idx := newRunIndex(source)
	var untimed *untimedIndex
	spans := Spans(c)
	choices := make([]Choice, 0, len(spans))
	next := 0
	for _, sp := range spans {
		var first, end int
		var ok bool
		if sp.Word != nil {
			first, end, ok = idx.find(sp.Text, sp.Start.Round(time.Millisecond), source, next)
		} else {
			if untimed == nil {
				untimed = newUntimedIndex(source)
			}
			first, end, ok = untimed.find(sp.Text, source, next)
		}
		if ok {
			ch := Choice{Source: makeRange(first, end)}
			if end == first+1 {
				ch.Word = sp.Word
			}
			choices = append(choices, ch)
			next = end
			continue
		}
		choices = append(choices, Choice{New: newWord(sp, max(0, c.Start), starts)})
	}
	return choices
}

// newWord builds the word of a span that no source word matched. A word on the start of a source word is an edited
// word and gets confidence 1.0. A moved word keeps its confidence.
func newWord(sp Span, cueStart time.Duration, starts map[time.Duration]bool) *NewWord {
	start, conf := sp.Start, 1.0
	if sp.Word == nil {
		start = cueStart
	} else if sp.Word.Confidence != nil && !starts[start.Round(time.Millisecond)] {
		conf = *sp.Word.Confidence
	}
	return &NewWord{Text: sp.Text, Start: start, End: sp.End, Confidence: conf}
}

func makeRange(from, to int) []int {
	out := make([]int, to-from)
	for i := range out {
		out[i] = from + i
	}
	return out
}

// runMatch tries the source word i, then the following source words without a start, as one run. The texts of the
// run join with one space. It returns the index after the first run that matches text. It compares in place, so a
// try costs at most the length of text.
func runMatch(text string, source []SourceWord, i int) (int, bool) {
	pos := 0
	for j := i; j < len(source); j++ {
		if j > i {
			if source[j].Start != nil || !strings.HasPrefix(text[pos:], " ") {
				return 0, false
			}
			pos++
		}
		if !strings.HasPrefix(text[pos:], source[j].Text) {
			return 0, false
		}
		pos += len(source[j].Text)
		if pos == len(text) || (pos > 0 && onlyPunct(text[pos:])) {
			return j + 1, true
		}
	}
	return 0, false
}

func onlyPunct(s string) bool {
	for s != "" {
		r, n := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError || !unicode.IsPunct(r) {
			return false
		}
		s = s[n:]
	}
	return true
}

// sameWordText reports whether the cue text t is the source text s. The OpenAI words lack the full stops of the
// text, so "quiet." is the source word "quiet".
func sameWordText(t, s string) bool {
	if t == s {
		return true
	}
	rest, ok := strings.CutPrefix(t, s)
	return ok && s != "" && onlyPunct(rest)
}

// SameWords reports whether the cue has no words, or has exactly the source words. It compares texts and starts only.
// A typed End or Confidence that differs from the stored one does not count.
func SameWords(c *model.Cue, source []SourceWord) bool {
	if len(c.Words) == 0 {
		return true
	}
	choices, ok := Reuse(c, source, true, true)
	if !ok || len(choices) != len(source) {
		return false
	}
	for i, ch := range choices {
		if ch.New != nil || !slices.Equal(ch.Source, []int{i}) {
			return false
		}
	}
	return true
}
