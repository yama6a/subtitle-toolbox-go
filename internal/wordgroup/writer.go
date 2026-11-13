package wordgroup

import (
	"strings"
	"time"
	"unicode"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// CueText returns the text of the cue as a transcript holds it: the lines joined with one space, ruby as
// "base (reading)", tags removed, entities decoded, each run of white space as one space, and no space at either end.
func CueText(c *model.Cue) string {
	return cleanSpace(markupcore.PlainText(markupcore.RubyAsText(strings.Join(c.Lines, " "))))
}

func cleanSpace(s string) string {
	var b strings.Builder
	pending := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			pending = true
			continue
		}
		if pending {
			b.WriteByte(' ')
			pending = false
		}
		b.WriteRune(r)
	}
	if pending {
		b.WriteByte(' ')
	}
	return strings.Trim(b.String(), " \x00")
}

// SameText reports whether stored is a string that gives the text of the cue.
func SameText(c *model.Cue, stored orderedjson.Value) bool {
	s, ok := stored.AsString()
	return ok && cleanSpace(s) == CueText(c)
}

// SameTime reports whether stored is a time that, rounded to whole milliseconds, equals t clamped to 0 or more.
// unit is the length of the stored value 1.
func SameTime(stored orderedjson.Value, unit, t time.Duration) bool {
	d, ok := stored.Duration(unit)
	return ok && d == max(0, t)
}

// CueSpeaker returns Cue.Speaker, else the name of the first <v> tag in the lines, else "".
func CueSpeaker(c *model.Cue) string {
	if c.Speaker != "" {
		return c.Speaker
	}
	name, _ := markupcore.Speaker(strings.Join(c.Lines, " "))
	return name
}

// TaggedLines returns the lines of the cue with a word timestamp tag at the place of each word of Cue.Words. The
// k-th tag belongs to Words[k]. A word that the lines do not hold gets no tag.
func TaggedLines(c *model.Cue) []string {
	words := make([]markupcore.TimedText, len(c.Words))
	for i, w := range c.Words {
		words[i] = markupcore.TimedText{Text: w.Text, Start: w.Start}
	}
	return markupcore.TagWords(c.Lines, words, markupcore.Voice{})
}

// Span is a timed part of the cue text: a word of Cue.Words and the untimed text after it.
type Span struct {
	Text  string
	Start time.Duration // the start of Word. The leading span has none
	End   time.Duration
	Word  *model.Word // nil for the leading span, the text before the first word
}

// Spans splits the text of the cue at the words of Cue.Words. A word that the text does not hold gives no span,
// and the span before it runs on to the next word that the text holds. A cue without words has no spans.
func Spans(c *model.Cue) []Span {
	if len(c.Words) == 0 {
		return nil
	}
	text := CueText(c)
	texts := make([]string, len(c.Words))
	for i, w := range c.Words {
		texts[i] = w.Text
	}
	matches := markupcore.SearchWords(text, texts, false)
	var found []int
	for k, m := range matches {
		if m.Start >= 0 {
			found = append(found, k)
		}
	}
	var spans []Span
	first, lead := len(text), c.End
	if len(found) > 0 {
		first, lead = matches[found[0]].Start, max(c.Words[found[0]].Start, c.Start)
	}
	if t := cleanSpace(text[:first]); t != "" {
		spans = append(spans, Span{Text: t, End: lead})
	}
	for n, k := range found {
		to := len(text)
		if n+1 < len(found) {
			to = matches[found[n+1]].Start
		}
		if t := cleanSpace(text[matches[k].Start:to]); t != "" {
			spans = append(spans, Span{Text: t, Start: c.Words[k].Start, End: c.Words[k].End, Word: &c.Words[k]})
		}
	}
	return spans
}
