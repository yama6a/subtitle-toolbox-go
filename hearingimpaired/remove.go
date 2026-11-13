package hearingimpaired

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/hilabels"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

// removeFromLines removes the annotations that opts selects from the lines of one cue, and the lines that they
// emptied. Lines whose visible text is not valid UTF-8 stay as they are, as the PHP patterns with the u flag fail.
func removeFromLines(lines []string, opts Options) []string {
	if !utf8.ValidString(hilabels.VisibleOf(lines).Text) {
		return lines
	}
	out := lines
	pairs := opts.brackets()
	for {
		v := hilabels.VisibleOf(out)
		var spans []hilabels.Span
		for _, p := range pairs {
			for _, s := range bracketMatches(v.Text, p) {
				spans = append(spans, widen(v.Text, s))
			}
		}
		if len(spans) == 0 {
			break
		}
		out = v.Cut(out, spans)
	}
	if !opts.KeepSpeakerLabels {
		out = hilabels.Remove(out, opts.AnyCaseLabels)
	}
	out = removeMusic(out, opts)
	return hilabels.RemoveEmptied(lines, out)
}

func (o Options) brackets() []BracketPair {
	pairs := slices.Clone(o.CustomBrackets)
	if !o.KeepSquareBrackets {
		pairs = append(pairs, BracketPair{Open: "[", Close: "]"})
	}
	if !o.KeepParentheses {
		pairs = append(pairs, BracketPair{Open: "(", Close: ")"})
	}
	return pairs
}

// widen adds the spaces and tabs after s when a line break, a space or a tab comes before it. Then it adds
// the spaces and tabs before s when the text ends after s or a line break or punctuation follows.
func widen(text string, s hilabels.Span) hilabels.Span {
	if s.Start == 0 || strings.IndexByte("\n \t", text[s.Start-1]) >= 0 {
		s.End = len(text) - len(strings.TrimLeft(text[s.End:], " \t"))
	}
	if s.End == len(text) || strings.IndexByte("\n.,!?;:", text[s.End]) >= 0 {
		s.Start = len(strings.TrimRight(text[:s.Start], " \t"))
	}
	return s
}

// bracketMatches returns each p.Open that no backslash follows, up to the next p.Close and an optional colon,
// with no p.Open between them.
func bracketMatches(text string, p BracketPair) []hilabels.Span {
	var out []hilabels.Span
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], p.Open) && !strings.HasPrefix(text[i+len(p.Open):], `\`) {
			if end, ok := closeOf(text, i+len(p.Open), p); ok {
				out = append(out, hilabels.Span{Start: i, End: end})
				i = end
				continue
			}
		}
		_, n := utf8.DecodeRuneInString(text[i:])
		i += n
	}
	return out
}

func closeOf(text string, j int, p BracketPair) (int, bool) {
	for j < len(text) {
		if strings.HasPrefix(text[j:], p.Close) {
			end := j + len(p.Close)
			if end < len(text) && text[end] == ':' {
				end++
			}
			return end, true
		}
		if strings.HasPrefix(text[j:], p.Open) {
			return 0, false
		}
		_, n := utf8.DecodeRuneInString(text[j:])
		j += n
	}
	return 0, false
}

// removeMusic removes lyrics with opts.Lyrics, and lines of only music symbols unless opts.KeepMusicLines.
func removeMusic(lines []string, opts Options) []string {
	if !opts.Lyrics && opts.KeepMusicLines {
		return lines
	}
	v := hilabels.VisibleOf(lines)
	var spans []hilabels.Span
	if opts.Lyrics {
		spans = lyricsSpans(v.Text)
	}
	return v.Cut(lines, append(spans, musicLineSpans(v.Text, opts)...))
}

// lyricsSpans pairs the music symbols in order: the first with the second, the third with the fourth.
func lyricsSpans(text string) []hilabels.Span {
	var spans []hilabels.Span
	open := -1
	for i := 0; i < len(text); {
		n := symbolAt(text, i)
		switch {
		case n == 0:
			_, n = utf8.DecodeRuneInString(text[i:])
		case open < 0:
			open = i
		default:
			spans = append(spans, widen(text, hilabels.Span{Start: open, End: i + n}))
			open = -1
		}
		i += n
	}
	return spans
}

func musicLineSpans(text string, opts Options) []hilabels.Span {
	var spans []hilabels.Span
	for start := 0; start <= len(text); {
		end := len(text)
		if k := strings.IndexByte(text[start:], '\n'); k >= 0 {
			end = start + k
		}
		line := text[:end]
		if (opts.Lyrics && (startsWithSymbol(line, start) || endsWithSymbol(line, start))) ||
			(!opts.KeepMusicLines && onlySymbols(line, start)) {
			spans = append(spans, widen(text, hilabels.Span{Start: start, End: end}))
		}
		start = end + 1
	}
	return spans
}

// symbolAt returns the length of the music symbol at text[i], or 0. A music symbol is U+2669 to U+266C, or a #
// with white space or the text edge on both sides, as PHP \s with the u flag reads white space.
func symbolAt(text string, i int) int {
	r, n := utf8.DecodeRuneInString(text[i:])
	if r >= 0x2669 && r <= 0x266C {
		return n
	}
	if r != '#' {
		return 0
	}
	before, m := utf8.DecodeLastRuneInString(text[:i])
	after, k := utf8.DecodeRuneInString(text[i+1:])
	if (m == 0 || markupcore.IsSpace(before)) && (k == 0 || markupcore.IsSpace(after)) {
		return 1
	}
	return 0
}

// The line helpers get the visible text up to the end of the line, so that symbolAt sees the text before the
// line, and start, the offset of the line.

func startsWithSymbol(line string, start int) bool {
	p := hilabels.AfterLead(line, start)
	return p < len(line) && symbolAt(line, p) > 0
}

func endsWithSymbol(line string, start int) bool {
	p := len(line)
	for p > start {
		r, n := utf8.DecodeLastRuneInString(line[start:p])
		if !hilabels.IsHSpace(r) {
			break
		}
		p -= n
	}
	_, n := utf8.DecodeLastRuneInString(line[start:p])
	return n > 0 && symbolAt(line, p-n) == n
}

func onlySymbols(line string, start int) bool {
	p := hilabels.AfterLead(line, start)
	if p == len(line) || symbolAt(line, p) == 0 {
		return false
	}
	for p < len(line) {
		n := symbolAt(line, p)
		if n == 0 {
			var r rune
			if r, n = utf8.DecodeRuneInString(line[p:]); !hilabels.IsHSpace(r) {
				return false
			}
		}
		p += n
	}
	return true
}
