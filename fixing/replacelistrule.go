package fixing

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

// listRunner holds the lookup tables of a ReplaceList for one Apply call. Entries with an empty From are left out.
type listRunner struct {
	wholeWords, wholeLines map[string]string
	list                   ReplaceList
	regexes                []*regexp2.Regexp
}

func newListRunner(l *ReplaceList) *listRunner {
	nonEmpty := func(in []Replacement) []Replacement {
		var out []Replacement
		for _, r := range in {
			if r.From != "" {
				out = append(out, r)
			}
		}
		return out
	}
	table := func(in []Replacement) map[string]string {
		m := make(map[string]string, len(in))
		for _, r := range in {
			if _, ok := m[r.From]; !ok {
				m[r.From] = r.To
			}
		}
		return m
	}
	run := &listRunner{list: ReplaceList{
		PartialWordsAlways: nonEmpty(l.PartialWordsAlways),
		BeginLines:         nonEmpty(l.BeginLines),
		EndLines:           nonEmpty(l.EndLines),
		PartialLines:       nonEmpty(l.PartialLines),
		PartialLinesAlways: nonEmpty(l.PartialLinesAlways),
		RegularExpressions: l.RegularExpressions,
	}}
	run.wholeWords = table(nonEmpty(l.WholeWords))
	run.wholeLines = table(nonEmpty(l.WholeLines))
	for _, r := range l.RegularExpressions {
		// Options.Validate has checked the pattern.
		re := regexp2.MustCompile(r.From, regexp2.None)
		re.MatchTimeout = matchTimeout
		run.regexes = append(run.regexes, re)
	}
	return run
}

func (f *fixer) replaceList(lines []string) []string {
	if f.err != nil {
		return lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = markupcore.MapTextRuns([]string{line}, func(text string, first, last bool) string {
			if f.err != nil {
				return text
			}
			return f.runner.run(f, text, first, last, i == len(lines)-1)
		})[0]
	}
	return out
}

// run is the rule for one text run. first and last mark the first and the last run of its line, lastLine marks the
// last line of the cue.
//
// Adapted from SubtitleEdit/subtitleedit, src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs (https://github.com/SubtitleEdit/subtitleedit/blob/e1b854665b40bf6e04271c2ec64084947060e632/src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs), MIT.
func (r *listRunner) run(f *fixer, text string, first, last, lastLine bool) string {
	if first && last {
		if to, ok := r.wholeLines[text]; ok {
			return to
		}
	}
	words := strings.Split(text, " ")
	for i, w := range words {
		words[i] = r.replaceWord(w)
	}
	text = strings.Join(words, " ")
	if first {
		text = r.replaceBeginLines(text)
	}
	if last && lastLine {
		text = r.replaceEndLines(text, f.continues)
	}
	for _, p := range r.list.PartialLines {
		text = replaceBetweenBoundaries(text, p.From, p.To)
	}
	for _, p := range r.list.PartialLinesAlways {
		text = strings.ReplaceAll(text, p.From, p.To)
	}
	return r.replaceRegexes(f, text)
}

func (r *listRunner) replaceRegexes(f *fixer, text string) string {
	for i, re := range r.regexes {
		if !utf8.ValidString(text) {
			return text
		}
		out, err := re.Replace(text, r.list.RegularExpressions[i].To, -1, -1)
		if err != nil {
			f.err = invalid("ReplaceList", `The regular expression "`+r.list.RegularExpressions[i].From+
				`" took longer than 1 s on cue `+strconv.Itoa(f.cue+1)+".")
			return text
		}
		text = out
	}
	return text
}

func (r *listRunner) replaceWord(word string) string {
	if hit, ok := r.lookUpWord(word); ok {
		return hit
	}
	for _, p := range r.list.PartialWordsAlways {
		word = strings.ReplaceAll(word, p.From, p.To)
	}
	if hit, ok := r.lookUpWord(word); ok {
		return hit
	}
	return word
}

// lookUpWord looks the word up with and without the punctuation around it.
func (r *listRunner) lookUpWord(word string) (string, bool) {
	pre, core, post := stripAffixes(word)
	if core == "" {
		return "", false
	}
	if to, ok := r.wholeWords[pre+core+post]; ok {
		return to, true
	}
	if to, ok := r.wholeWords[pre+core]; ok {
		return to + post, true
	}
	if to, ok := r.wholeWords[core+post]; ok {
		return pre + to, true
	}
	if to, ok := r.wholeWords[core]; ok {
		return pre + to + post, true
	}
	return "", false
}

// stripAffixes splits a word into a prefix of "-", ".", '"' and "(", a core, and a suffix of closing
// punctuation. The prefix leaves at least one byte, and the suffix never starts at byte 0.
func stripAffixes(word string) (pre, core, post string) {
	i := 0
	for _, set := range []struct {
		c   byte
		max int
	}{{'-', len(word)}, {'.', len(word)}, {'"', len(word)}, {'(', 1}} {
		for n := 0; n < set.max && i < len(word)-1 && word[i] == set.c; n++ {
			i++
		}
	}
	j := len(word)
	for j > i && strings.IndexByte(`".,!?)]:;`, word[j-1]) >= 0 {
		j--
	}
	if j == 0 && len(word) > 0 {
		j = 1
	}
	return word[:i], word[i:j], word[j:]
}

func (r *listRunner) replaceBeginLines(text string) string {
	prefixLen := 0
	if utf8.ValidString(text) {
		prefixLen = len(text) - len(strings.TrimLeftFunc(text, isBeginPrefix))
	}
	prefix, rest := text[:prefixLen], text[prefixLen:]
	for _, p := range r.list.BeginLines {
		if strings.HasPrefix(rest, p.From) {
			rest = p.To + rest[len(p.From):]
		}
		for _, mark := range []string{". ", "! ", "? "} {
			rest = strings.ReplaceAll(rest, mark+p.From, mark+p.To)
		}
	}
	return prefix + rest
}

func isBeginPrefix(r rune) bool {
	switch r {
	case ' ', '"', '\'', '[', '(', '¶', '-', '‐', '–', '—':
		return true
	}
	return false
}

// replaceEndLines replaces the end of the text. A cue that continues in the next cue gets no added full stop.
func (r *listRunner) replaceEndLines(text string, continues bool) string {
	for _, p := range r.list.EndLines {
		addsStop := strings.HasSuffix(p.To, ".") && !strings.HasSuffix(p.From, ".")
		if strings.HasSuffix(text, p.From) && (!continues || !addsStop) {
			text = text[:len(text)-len(p.From)] + p.To
		}
	}
	return text
}

// replaceBetweenBoundaries replaces from where it starts and ends at a boundary character or a text edge.
//
// Adapted from SubtitleEdit/subtitleedit, src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs (https://github.com/SubtitleEdit/subtitleedit/blob/e1b854665b40bf6e04271c2ec64084947060e632/src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs), MIT.
func replaceBetweenBoundaries(text, from, to string) string {
	var b strings.Builder
	offset := 0
	for {
		i := strings.Index(text[offset:], from)
		if i < 0 {
			break
		}
		pos := offset + i
		end := pos + len(from)
		startOK := pos == 0 || strings.HasPrefix(from, " ") || isBoundary(lastChar(text[:pos]))
		endOK := end == len(text) || strings.HasSuffix(to, " ") || isBoundary(firstChar(text[end:]))
		if startOK && endOK {
			b.WriteString(text[offset:pos])
			b.WriteString(to)
			offset = end
		} else {
			b.WriteString(text[offset : pos+1])
			offset = pos + 1
		}
	}
	if offset == 0 {
		return text
	}
	b.WriteString(text[offset:])
	return b.String()
}

// lastChar and firstChar return the rune at the edge of s, or RuneError for a byte that is not valid UTF-8.
func lastChar(s string) rune {
	r, n := utf8.DecodeLastRuneInString(s)
	if r == utf8.RuneError && n <= 1 {
		return utf8.RuneError
	}
	return r
}

func firstChar(s string) rune {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && n <= 1 {
		return utf8.RuneError
	}
	return r
}

func isBoundary(r rune) bool {
	switch r {
	case utf8.RuneError:
		return false
	case '<', '>', '`', '´', '♪':
		return true
	}
	return unicode.IsPunct(r) || unicode.IsSpace(r)
}
