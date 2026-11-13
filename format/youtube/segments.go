package youtube

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// stored is a segment of a cue with the stored object of its source segment. A new segment has an empty object.
type stored struct {
	segment
	fields orderedjson.Value
}

var styleTagRegexp = regexp.MustCompile(`(?i)^<(/?)(b|i|u|font)\b`)

// segmentsOf returns the segments that a writer writes for a cue. An unchanged cue keeps the stored segments. Else
// the segments come from the tagged lines of the cue, and a segment with the text and the offset of a stored
// segment keeps its other fields.
func segmentsOf(c model.Cue, source []stored) (segments []stored, unchanged bool) {
	start := max(0, c.Start)
	if isUnchanged(c, source, start) {
		return source, true
	}
	segments = fromMarkup(c, start)
	if kept, ok := retext(segments, source); ok {
		return kept, false
	}
	return keepFields(segments, source), false
}

func isUnchanged(c model.Cue, source []stored, start time.Duration) bool {
	if len(source) == 0 {
		return false
	}
	segs := make([]segment, len(source))
	for i, s := range source {
		segs[i] = s.segment
	}
	plain := markupcore.CleanLines(captionLines(segs, start, false))
	probe := model.Cue{Start: start, End: c.End, Lines: markupcore.CleanLines(captionLines(segs, start, true))}
	probe.ExtractWordTags()
	if !slices.Equal(plain, c.Lines) && !slices.Equal(probe.Lines, c.Lines) {
		return false
	}
	return len(c.Words) == 0 || slices.EqualFunc(c.Words, probe.Words, func(a, b model.Word) bool {
		return a.Text == b.Text && a.Start == b.Start
	})
}

// markupWalker builds segments from the tokens of tagged lines.
type markupWalker struct {
	out []stored
	// texts holds the text of each segment of out, joined once at the end.
	texts     []*strings.Builder
	open      []openTag
	offset    time.Duration
	hasOffset bool
	// timed is true right after a word timestamp, so the next text starts a segment.
	timed bool
}

// text adds the text between 2 tags. It joins the segment before it when no word timestamp came between them and
// the style is equal, or the style is plain and the text is white space only.
func (w *markupWalker) text(token string) {
	st := styleOf(w.open)
	decoded := markupcore.DecodeEntities(token)
	last := len(w.out) - 1
	switch {
	case last >= 0 && !w.timed && w.out[last].style == st,
		last >= 0 && !w.timed && st == (style{}) && trimSpace(token) == "":
		w.texts[last].WriteString(decoded)
	default:
		w.out = append(w.out, stored{
			segment: segment{offset: w.offset, hasOffset: w.hasOffset, style: st},
			fields:  orderedjson.ObjectOf(),
		})
		w.texts = append(w.texts, &strings.Builder{})
		w.texts[last+1].WriteString(decoded)
	}
	w.timed = false
}

// fromMarkup walks the tagged lines of the cue. A word timestamp tag sets the offset of the next text, and the
// style tags b, i, u and font set the style.
func fromMarkup(c model.Cue, start time.Duration) []stored {
	w := &markupWalker{}
	text := markupcore.RubyAsText(strings.Join(wordgroup.TaggedLines(&c), "\n"))
	for i, token := range markupcore.SplitTags(text) {
		switch at, isTime := markupcore.WordTimestamp(token); {
		case i%2 == 0:
			if token != "" {
				w.text(token)
			}
		case isTime:
			w.offset, w.hasOffset, w.timed = max(0, at-start), true, true
		default:
			w.open = applyTag(w.open, token)
		}
	}
	for i, b := range w.texts {
		w.out[i].text = b.String()
	}
	return moveSpacesAfterTimes(w.out)
}

// openTag is an open style tag. A <font> tag has the color of its attribute, "" for none.
type openTag struct{ name, color string }

func styleOf(open []openTag) style {
	var st style
	for _, t := range open {
		switch t.name {
		case "font":
			if t.color != "" {
				st.color = t.color
			}
		case "b":
			st.bold = true
		case "i":
			st.italic = true
		case "u":
			st.underline = true
		}
	}
	return st
}

func applyTag(open []openTag, tag string) []openTag {
	m := styleTagRegexp.FindStringSubmatch(tag)
	if m == nil {
		return open
	}
	name := strings.ToLower(m[2])
	if m[1] == "" {
		t := openTag{name: name}
		if name == "font" {
			if raw, ok := markupcore.FontColor(tag); ok {
				t.color, _ = markupcore.ColorHex(raw)
			}
		}
		return append(open, t)
	}
	for i := len(open) - 1; i >= 0; i-- {
		if open[i].name == name {
			return slices.Delete(open, i, i+1)
		}
	}
	return open
}

// moveSpacesAfterTimes moves the white space at the end of a segment to the start of the next segment when that one
// has another offset. The reader writes the leading white space of a timed segment before its word timestamp.
func moveSpacesAfterTimes(segs []stored) []stored {
	for i := 1; i < len(segs); i++ {
		cur, prev := &segs[i], &segs[i-1]
		if !cur.hasOffset || cur.hasOffset == prev.hasOffset && cur.offset == prev.offset {
			continue
		}
		body := strings.TrimRight(prev.text, asciiSpace)
		if body != "" {
			cur.text = prev.text[len(body):] + cur.text
			prev.text = body
		}
	}
	return segs
}

type segmentKey struct {
	text   string
	offset time.Duration
}

// keepFields gives each segment the fields of the next unused stored segment with the same text and offset. A
// missing offset counts as 0. Each text and offset has an ascending list of stored positions, and a cursor
// moves along it, so the cost is linear.
func keepFields(segs, source []stored) []stored {
	positions := map[segmentKey][]int{}
	for i, s := range source {
		k := segmentKey{s.text, s.offset}
		positions[k] = append(positions[k], i)
	}
	cursor := map[segmentKey]int{}
	next := 0
	for i, s := range segs {
		k := segmentKey{s.text, s.offset}
		list := positions[k]
		c := cursor[k]
		for c < len(list) && list[c] < next {
			c++
		}
		cursor[k] = c
		if c < len(list) {
			segs[i].fields = source[list[c]].fields
			next = list[c] + 1
		}
	}
	return segs
}

// retext handles a cue without word times and styles whose text has as many words as the stored segments. Each
// stored segment must be plain and hold one word. A changed word keeps the offset of its segment and loses its
// other fields.
func retext(segs, source []stored) ([]stored, bool) {
	if len(segs) != 1 || segs[0].hasOffset || segs[0].style != (style{}) ||
		!slices.ContainsFunc(source, func(s stored) bool { return s.hasOffset }) {
		return nil, false
	}
	words := strings.Fields(segs[0].text)
	if len(words) != len(source) {
		return nil, false
	}
	out := slices.Clone(source)
	for i, s := range source {
		lead, word, trail, ok := oneWord(s.text)
		if !ok || s.style != (style{}) {
			return nil, false
		}
		if word != words[i] {
			out[i].text = lead + words[i] + trail
			out[i].fields = orderedjson.ObjectOf()
		}
	}
	return out, true
}

// oneWord splits text into white space, one word and white space.
func oneWord(text string) (lead, word, trail string, ok bool) {
	rest := strings.TrimLeftFunc(text, unicode.IsSpace)
	word = strings.TrimRightFunc(rest, unicode.IsSpace)
	if word == "" || strings.IndexFunc(word, unicode.IsSpace) >= 0 {
		return "", "", "", false
	}
	return text[:len(text)-len(rest)], word, rest[len(word):], true
}
