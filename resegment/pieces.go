package resegment

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	closers     = `["'\)\]\x{2019}\x{201D}\x{3009}\x{300B}\x{300D}\x{300F}\x{3011}\x{FF09}\x{FF3D}\x{FF5D}]*`
	cjkBreaks   = `[\x{3001}\x{3002}\x{FF01}\x{FF0C}\x{FF1A}\x{FF1B}\x{FF1F}\x{FF61}\x{FF64}]`
	sentenceEnd = `[.?!\x{2026}\x{3002}\x{FF01}\x{FF1F}\x{FF61}]+`
	clauseEnd   = `(?:[,;:\x{2013}\x{2014}\x{3001}\x{FF0C}\x{FF1A}\x{FF1B}\x{FF64}]|--|^-)`
)

var (
	tagRegexp         = regexp.MustCompile(markupcore.TagPattern)
	cjkChunkRegexp    = regexp.MustCompile(`(?s).*?` + cjkBreaks + `+` + closers + `|.+`)
	cjkBreakEndRegexp = regexp.MustCompile(cjkBreaks + closers + `$`)
	sentenceRegexp    = regexp.MustCompile(sentenceEnd + closers + `$`)
	clauseRegexp      = regexp.MustCompile(clauseEnd + closers + `$`)
)

type tokenKind int

const (
	textToken tokenKind = iota
	tagToken
	markerToken
)

// token is a run of text, a tag, or the zero-width marker of the word with index word.
type token struct {
	kind tokenKind
	text string
	word int
}

// piece is the unit that a break can separate: a word of the text, or a part of a word.
type piece struct {
	text       string
	length     int
	separator  string
	startWord  int
	words      []int
	plain      string
	openBefore []markupcore.OpenTag
	openAfter  []markupcore.OpenTag
	time       time.Duration
}

func (p piece) hasTime() bool { return p.startWord >= 0 }

// cuePieces holds the pieces of one cue and what the breaks need to know about them.
type cuePieces struct {
	pieces []piece
	// positions[i] is the visible length before piece i, plus 1 for each separator up to piece i.
	positions []int
	// ranks[i] is the break rank before piece i, or -1 for no break.
	ranks []int
	ends  []bool
}

func newCuePieces(c model.Cue) cuePieces {
	words := make([]string, len(c.Words))
	for i, w := range c.Words {
		words[i] = w.Text
	}

	var ps cuePieces
	var open []markupcore.OpenTag
	for _, p := range rawPieces(c.Lines, markupcore.WordMarks(c.Lines, words)) {
		p.openBefore = open
		open = markupcore.OpenCoreTags(p.text, open)
		p.openAfter = open
		p.plain = markupcore.PlainText(p.text)
		if p.hasTime() {
			p.time = c.Words[p.startWord].Start.Round(time.Millisecond)
		}
		ps.pieces = append(ps.pieces, p)
	}
	ps.measure()
	return ps
}

// rawPieces splits the lines into pieces, with the markers of the words at their marks.
func rawPieces(lines []string, marks []markupcore.Mark) []piece {
	var raw []piece
	var prefix []token
	next := 0
	for lineIndex, line := range lines {
		var lineMarks []markupcore.Mark
		var lineWords []int
		for next < len(marks) && marks[next].Line == lineIndex {
			lineMarks = append(lineMarks, marks[next])
			lineWords = append(lineWords, next)
			next++
		}
		for wordIndex, word := range splitSpaces(tokenize(line, lineMarks, lineWords)) {
			separator := ""
			switch {
			case wordIndex > 0:
				separator = " "
			case len(raw) > 0 || len(prefix) > 0:
				separator = "\n"
			}
			raw, prefix = addWordPieces(raw, prefix, word, separator)
		}
	}
	return raw
}

func (ps *cuePieces) measure() {
	n := len(ps.pieces)
	ps.positions = make([]int, n+1)
	ps.ranks = make([]int, n)
	ps.ends = make([]bool, n)
	for i, p := range ps.pieces {
		sep := 0
		if i+1 < n && ps.pieces[i+1].separator != "" {
			sep = 1
		}
		ps.positions[i+1] = ps.positions[i] + p.length + sep
		next := ""
		if i+1 < n {
			next = ps.pieces[i+1].plain
		}
		r, _ := utf8.DecodeRuneInString(next)
		ps.ends[i] = sentenceRegexp.MatchString(p.plain) && !unicode.IsLower(r)
	}
	for i := range n {
		ps.ranks[i] = ps.rank(i)
	}
}

func (ps *cuePieces) rank(i int) int {
	switch {
	case i == 0:
		return -1
	case ps.ends[i-1]:
		return 0
	case clauseRegexp.MatchString(ps.pieces[i-1].plain):
		return 1
	case ps.pieces[i].separator != "" || ps.pieces[i].hasTime():
		return 2
	}
	return -1
}

// text joins pieces first to after-1. It opens the core tags that are open before them and closes the tags that
// are open after them, unless after is the end.
func (ps *cuePieces) text(first, after int) string {
	var b strings.Builder
	for _, t := range ps.pieces[first].openBefore {
		b.WriteString(t.Tag)
	}
	for i := first; i < after; i++ {
		if i > first {
			b.WriteString(ps.pieces[i].separator)
		}
		b.WriteString(ps.pieces[i].text)
	}
	if after < len(ps.pieces) {
		b.WriteString(markupcore.CloseCoreTags(ps.pieces[after-1].openAfter))
	}
	return b.String()
}

func (ps *cuePieces) lines(first, after int) []string {
	return strings.Split(ps.text(first, after), "\n")
}

// wordIndexes returns the indexes of the words whose markers lie in pieces first to after-1.
func (ps *cuePieces) wordIndexes(first, after int) []int {
	var out []int
	for _, p := range ps.pieces[first:after] {
		out = append(out, p.words...)
	}
	return out
}

// times returns the start of each piece, then the end. A word start sets the time of its piece. Other times split
// the span between 2 anchors in proportion to the visible characters.
func (ps *cuePieces) times(start, end time.Duration) []time.Duration {
	n := len(ps.pieces)
	anchors := []int{0}
	anchorTime := map[int]time.Duration{0: start}
	last := start
	for i := 1; i < n; i++ {
		if t := ps.pieces[i].time; ps.pieces[i].hasTime() && t > last && t < end {
			anchors = append(anchors, i)
			anchorTime[i] = t
			last = t
		}
	}
	anchors = append(anchors, n)
	anchorTime[n] = max(start, end)

	times := make([]time.Duration, n+1)
	for k := 1; k < len(anchors); k++ {
		a, b := anchors[k-1], anchors[k]
		span := ps.positions[b] - ps.positions[a]
		for i := a; i < b; i++ {
			share := 0.0
			if span > 0 {
				share = float64(ps.positions[i]-ps.positions[a]) / float64(span)
			}
			d := float64(anchorTime[b] - anchorTime[a])
			times[i] = (anchorTime[a] + time.Duration(d*share)).Round(time.Millisecond)
		}
	}
	times[n] = anchorTime[n]
	return times
}

// tokenize splits line into tags, text and word markers. marks and words hold the place and the index of each marker.
func tokenize(line string, marks []markupcore.Mark, words []int) []token {
	var tokens []token
	k := 0
	emit := func(upTo int) {
		for k < len(marks) && marks[k].Offset <= upTo {
			tokens = append(tokens, token{kind: markerToken, word: words[k]})
			k++
		}
	}
	addText := func(a, b int) {
		for a < b {
			emit(a)
			next := b
			if k < len(marks) && marks[k].Offset < b {
				next = marks[k].Offset
			}
			tokens = append(tokens, token{kind: textToken, text: line[a:next]})
			a = next
		}
	}
	at := 0
	for _, loc := range tagRegexp.FindAllStringIndex(line, -1) {
		addText(at, loc[0])
		emit(loc[1] - 1)
		tokens = append(tokens, token{kind: tagToken, text: line[loc[0]:loc[1]]})
		at = loc[1]
	}
	addText(at, len(line))
	emit(len(line))
	return tokens
}

// splitSpaces groups the tokens into words at the ASCII spaces outside tags.
func splitSpaces(tokens []token) [][]token {
	var words [][]token
	var cur []token
	for _, t := range tokens {
		if t.kind != textToken {
			cur = append(cur, t)
			continue
		}
		for i, chunk := range strings.Split(t.text, " ") {
			if i > 0 && len(cur) > 0 {
				words = append(words, cur)
				cur = nil
			}
			if chunk != "" {
				cur = append(cur, token{kind: textToken, text: chunk})
			}
		}
	}
	if len(cur) > 0 {
		words = append(words, cur)
	}
	return words
}

// splitWord splits a word after a run of CJK break punctuation and at a word marker that follows text.
func splitWord(tokens []token) [][]token {
	parts := [][]token{nil}
	hasText, afterBreak := false, false
	add := func(t token) { parts[len(parts)-1] = append(parts[len(parts)-1], t) }
	for _, t := range tokens {
		if t.kind == textToken {
			for _, chunk := range cjkChunkRegexp.FindAllString(t.text, -1) {
				if hasText && afterBreak {
					parts = append(parts, nil)
				}
				add(token{kind: textToken, text: chunk})
				hasText = true
				afterBreak = cjkBreakEndRegexp.MatchString(chunk)
			}
			continue
		}
		if hasText && startsPart(t, afterBreak) {
			parts = append(parts, nil)
			hasText, afterBreak = false, false
		}
		add(t)
	}
	return parts
}

// startsPart reports whether a tag or marker after text starts a new part.
func startsPart(t token, afterBreak bool) bool {
	return t.kind == markerToken || afterBreak && !strings.HasPrefix(t.text, "</")
}

func render(tokens []token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.kind != markerToken {
			b.WriteString(t.text)
		}
	}
	return b.String()
}

// addWordPieces adds the pieces of one word. A piece without visible text joins the piece before it, so that no
// cue holds only tags. At the start of a cue, it goes before the next piece.
func addWordPieces(raw []piece, prefix []token, word []token, separator string) ([]piece, []token) {
	for i, tokens := range splitWord(word) {
		sep := ""
		if i == 0 {
			sep = separator
		}
		text := render(tokens)
		length := linewrap.Length(linewrap.MeasuredWords(text))
		switch {
		case length == 0 && len(raw) > 0:
			last := &raw[len(raw)-1]
			last.text += sep + text
			last.words = append(last.words, markerWords(tokens)...)
		case length == 0:
			prefix = append(append(prefix, tokens...), token{kind: textToken, text: " "})
		default:
			tokens = append(prefix, tokens...)
			prefix = nil
			p := piece{text: render(tokens), length: length, separator: sep, startWord: startWord(tokens)}
			p.words = markerWords(tokens)
			raw = append(raw, p)
		}
	}
	return raw, prefix
}

func startWord(tokens []token) int {
	for _, t := range tokens {
		switch t.kind {
		case markerToken:
			return t.word
		case textToken:
			return -1
		case tagToken:
		}
	}
	return -1
}

func markerWords(tokens []token) []int {
	var out []int
	for _, t := range tokens {
		if t.kind == markerToken {
			out = append(out, t.word)
		}
	}
	return out
}
