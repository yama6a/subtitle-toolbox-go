package markupcore

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

var exactVoiceTagRegexp = regexp.MustCompile(`^` + VoiceTagPattern + `$`)

// TimedText is the text and the start of a word.
type TimedText struct {
	Text  string
	Start time.Duration
}

// Voice is a whole-cue voice tag. An empty Name is no voice.
type Voice struct {
	Name  string
	Class string // voice classes such as ".loud", "" for none
	Close bool   // a </v> ends the last line with visible text
}

// Match is the byte range of a found word in a text. Start is -1 for a word that the search did not find.
type Match struct{ Start, End int }

// Mark is the place of the marker of a word in the lines: the byte offset in line Line.
type Mark struct{ Line, Offset int }

// CleanLines cleans each line, as Cue.SetText does, and drops the lines that stay empty. A clean line has LF
// and CR turned into one form, runs of spaces and tabs collapsed, and no PHP trim character at either end.
func CleanLines(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l = CleanLine(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// CleanLine cleans one line as CleanLines does.
func CleanLine(s string) string {
	return strings.Trim(collapseRuns(NormalizeLineEnds(s)), PHPTrimSet)
}

func collapseRuns(s string) string {
	if !strings.ContainsAny(s, "\t\n") && !strings.Contains(s, "  ") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		ch := s[i]
		if ch == '\t' {
			ch = ' '
		}
		if (ch == ' ' || ch == '\n') && len(out) > 0 && out[len(out)-1] == ch {
			continue
		}
		out = append(out, ch)
	}
	return string(out)
}

// plainIndex is the plain text of lines and, for each byte of it, the place it came from.
type plainIndex struct {
	text    []byte
	marks   []Mark
	tags    []tagAt
	pending bool
	keep    bool
}

// tagAt is a tag of the lines and the length of the plain text before it.
type tagAt struct{ line, start, end, plain int }

func buildPlain(lines []string, keepMarks bool) plainIndex {
	p := plainIndex{keep: keepMarks}
	for i, line := range lines {
		p.pending = true
		at := 0
		for _, loc := range tagRegexp.FindAllStringIndex(line, -1) {
			p.run(line[at:loc[0]], i, at)
			p.tags = append(p.tags, tagAt{i, loc[0], loc[1], len(p.text)})
			at = loc[1]
		}
		p.run(line[at:], i, at)
	}
	return p
}

func (p *plainIndex) run(s string, line, base int) {
	for i := 0; i < len(s); {
		if s[i] == '&' {
			if v, n, ok := decodeEntity(s[i:]); ok {
				p.add(v, line, base+i)
				i += n
				continue
			}
		}
		r, w := utf8.DecodeRuneInString(s[i:])
		switch {
		case r != utf8.RuneError && IsSpace(r):
			p.pending = true
		default:
			p.emit(s[i:i+w], line, base+i)
		}
		i += w
	}
}

func (p *plainIndex) add(decoded string, line, offset int) {
	for j := 0; j < len(decoded); {
		r, w := utf8.DecodeRuneInString(decoded[j:])
		if r != utf8.RuneError && IsSpace(r) {
			p.pending = true
		} else {
			p.emit(decoded[j:j+w], line, offset)
		}
		j += w
	}
}

func (p *plainIndex) emit(s string, line, offset int) {
	if p.pending && len(p.text) > 0 {
		p.put(" ", line, offset)
	}
	p.pending = false
	p.put(s, line, offset)
}

func (p *plainIndex) put(s string, line, offset int) {
	p.text = append(p.text, s...)
	if p.keep {
		for range len(s) {
			p.marks = append(p.marks, Mark{line, offset})
		}
	}
}

// CuePlainText returns the lines joined with one space, tags removed, entities decoded, each run of white space
// as IsSpace defines it replaced by one space, and trimmed. Cue.PlainText calls it.
func CuePlainText(lines []string) string {
	return string(buildPlain(lines, false).text)
}

// FindWords finds each word in text in order, with a byte search from the end of the match before. It stops at
// the first word that it does not find and returns the matches before it.
func FindWords(text string, words []string) []Match {
	out := make([]Match, 0, len(words))
	at := 0
	for _, w := range words {
		i := strings.Index(text[at:], w)
		if w == "" || i < 0 {
			break
		}
		out = append(out, Match{at + i, at + i + len(w)})
		at += i + len(w)
	}
	return out
}

// SearchWords finds each word in text in order, each search from the end of the match before. With fold, runes
// compare with simple case folding. A word that it does not find gets Start -1, and the next word searches from the
// same place. A text that was not found once is not searched again.
func SearchWords(text string, words []string, fold bool) []Match {
	hay, src := text, []int(nil)
	if fold {
		hay, src = foldText(text)
	}
	out := make([]Match, len(words))
	missing := map[string]bool{}
	at := 0
	for k, w := range words {
		out[k] = Match{-1, -1}
		if fold {
			w = foldString(w)
		}
		if w == "" || missing[w] {
			continue
		}
		start, ok := searchFrom(hay, w, at, src)
		if !ok {
			missing[w] = true
			continue
		}
		at = start + len(w)
		if fold {
			out[k] = Match{src[start], src[at]}
		} else {
			out[k] = Match{start, at}
		}
	}
	return out
}

// searchFrom returns the first start at or after at where w occurs in hay. With src, the start and the end must
// both be at a rune boundary of the source text.
func searchFrom(hay, w string, at int, src []int) (int, bool) {
	for {
		i := strings.Index(hay[at:], w)
		if i < 0 {
			return 0, false
		}
		start := at + i
		if src == nil || (src[start] >= 0 && src[start+len(w)] >= 0) {
			return start, true
		}
		at = start + 1
	}
}

func foldRune(r rune) rune {
	if r < utf8.RuneSelf {
		if r >= 'a' && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}
	lo := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		lo = min(lo, f)
	}
	return lo
}

func foldString(s string) string {
	folded, _ := foldWithMap(s, false)
	return folded
}

func foldText(s string) (string, []int) {
	return foldWithMap(s, true)
}

// foldWithMap folds each rune of s to the smallest rune of its case orbit. A byte that is not UTF-8 stays as it is.
// The map holds, for each byte of the folded text, the source offset of its rune, or -1 inside a rune.
func foldWithMap(s string, withMap bool) (string, []int) {
	var b strings.Builder
	b.Grow(len(s))
	var src []int
	if withMap {
		src = make([]int, 0, len(s)+1)
	}
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		before := b.Len()
		if r == utf8.RuneError && w == 1 {
			b.WriteByte(s[i])
		} else {
			b.WriteRune(foldRune(r))
		}
		if withMap {
			src = append(src, i)
			for range b.Len() - before - 1 {
				src = append(src, -1)
			}
		}
		i += w
	}
	if withMap {
		src = append(src, len(s))
	}
	return b.String(), src
}

// WordMarks returns the place of the marker of each word, by the placement rule of docs/markup.md. It finds the
// words in CuePlainText of lines with FindWords, so it returns fewer marks than words when one is missing.
func WordMarks(lines []string, words []string) []Mark {
	p := buildPlain(lines, true)
	matches := FindWords(string(p.text), words)
	marks := make([]Mark, len(matches))
	for k, m := range matches {
		at := p.marks[m.Start]
		marks[k] = Mark{at.Line, markerOffset(lines[at.Line], at.Offset)}
	}
	return marks
}

// markerOffset moves p back over white space and whole opening tags, then forward over white space.
func markerOffset(line string, p int) int {
	for p > 0 {
		before := p
		if line[p-1] == '>' {
			p = openingTagStart(line, p)
		} else if r, w := utf8.DecodeLastRuneInString(line[:p]); r != utf8.RuneError && IsSpace(r) {
			p -= w
		}
		if p == before {
			break
		}
	}
	for p < len(line) {
		r, w := utf8.DecodeRuneInString(line[p:])
		if r == utf8.RuneError || !IsSpace(r) {
			break
		}
		p += w
	}
	return p
}

// openingTagStart returns the start of the opening tag that ends at p, or p when none does.
func openingTagStart(line string, p int) int {
	j := strings.LastIndexAny(line[:p-1], "<>")
	if j < 0 || line[j] != '<' || strings.HasPrefix(line[j:], "</") || !isTag(line[j:p]) {
		return p
	}
	return j
}

func isTag(s string) bool {
	loc := tagRegexp.FindStringIndex(s)
	return loc != nil && loc[0] == 0 && loc[1] == len(s)
}

// TagWords returns lines with the word timestamp tag of each word at its mark, and the voice tag. A word without a
// mark gets no tag.
func TagWords(lines []string, words []TimedText, voice Voice) []string {
	marks := WordMarks(lines, wordTexts(words))
	out := make([]string, len(lines))
	k := 0
	for i, line := range lines {
		var b strings.Builder
		copied := 0
		for k < len(marks) && marks[k].Line == i {
			b.WriteString(line[copied:marks[k].Offset])
			b.WriteString("<" + CoreTimestamp(words[k].Start) + ">")
			copied = marks[k].Offset
			k++
		}
		b.WriteString(line[copied:])
		out[i] = b.String()
	}
	if voice.Name == "" {
		return out
	}
	first, last := -1, -1
	for i, line := range lines {
		if HasVisibleText([]string{line}) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first >= 0 {
		out[first] = "<v" + voice.Class + " " + EscapeText(voice.Name) + ">" + out[first]
		if voice.Close {
			out[last] += "</v>"
		}
	}
	return out
}

// WordTags is what ExtractWordTags found in the lines of a cue.
type WordTags struct {
	Lines []string        // the cleaned lines without the extracted tags
	Words []TimedText     // the timed words in order, without the empty ones and the ones that go back in time
	Ends  []time.Duration // the end of each word of Words, from the next tag, else from cueEnd
	Found bool            // the lines held a word timestamp tag
	Voice Voice           // the whole-cue voice, Name "" for none
}

type timestamp struct {
	tagAt
	time time.Duration
}

// ExtractWordTags reads the word timestamp tags and the whole-cue voice of lines. cueEnd is the end of the last
// word. classAllowed tells whether a voice tag with classes may become the voice. docs/markup.md has the rules.
func ExtractWordTags(lines []string, cueEnd time.Duration, classAllowed bool) WordTags {
	lines = CleanLines(lines)
	p := buildPlain(lines, false)
	var stamps []timestamp
	for _, t := range p.tags {
		if d, ok := WordTimestamp(lines[t.line][t.start:t.end]); ok && d < timecode.MaxTime {
			stamps = append(stamps, timestamp{t, d})
		}
	}
	res := WordTags{Lines: lines}
	if len(stamps) == 0 {
		res.Voice, res.Lines = extractVoice(lines, classAllowed)
		return res
	}
	res.Found = true
	res.Words, res.Ends = stampWords(string(p.text), stamps, cueEnd)
	res.Voice, res.Lines = extractVoice(removeWordTags(lines), classAllowed)
	// Removing tags can join text into a new tag, such as "<a<00:00:01.000>>". Words that the new text lacks go.
	found := len(FindWords(CuePlainText(res.Lines), wordTexts(res.Words)))
	res.Words, res.Ends = res.Words[:found], res.Ends[:found]
	return res
}

func wordTexts(words []TimedText) []string {
	texts := make([]string, len(words))
	for i, w := range words {
		texts[i] = w.Text
	}
	return texts
}

func stampWords(plain string, stamps []timestamp, cueEnd time.Duration) ([]TimedText, []time.Duration) {
	var kept []int
	for i, s := range stamps {
		if len(kept) == 0 || s.time >= stamps[kept[len(kept)-1]].time {
			kept = append(kept, i)
		}
	}
	var words []TimedText
	var ends []time.Duration
	for n, i := range kept {
		to := len(plain)
		if n+1 < len(kept) {
			to = stamps[kept[n+1]].plain
		}
		text := strings.TrimFunc(plain[stamps[i].plain:to], IsSpace)
		if text == "" {
			continue
		}
		end := cueEnd
		if i+1 < len(stamps) {
			end = stamps[i+1].time
		}
		if end >= timecode.MaxTime {
			end = stamps[i].time
		}
		words = append(words, TimedText{text, stamps[i].time})
		ends = append(ends, max(end, stamps[i].time))
	}
	return words, ends
}

func removeWordTags(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		parts := SplitTags(line)
		var b strings.Builder
		for j, part := range parts {
			if j%2 == 1 {
				if d, ok := WordTimestamp(part); ok && d < timecode.MaxTime {
					continue
				}
			}
			b.WriteString(part)
		}
		out[i] = b.String()
	}
	return CleanLines(out)
}

// extractVoice applies steps 7 to 9 of the rules in docs/markup.md to lines without word timestamp tags.
func extractVoice(lines []string, classAllowed bool) (Voice, []string) {
	lines = CleanLines(lines)
	voice, ok := wholeCueVoice(lines)
	if !ok || (voice.Class != "" && !classAllowed) {
		return Voice{}, lines
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		parts := SplitTags(line)
		var b strings.Builder
		for j, part := range parts {
			if j%2 == 1 && (part == "</v>" || exactVoiceTagRegexp.MatchString(part)) {
				continue
			}
			b.WriteString(part)
		}
		out[i] = b.String()
	}
	if out = CleanLines(out); !HasVisibleText(out) {
		return Voice{}, lines
	}
	return voice, out
}

// voiceScan walks the parts of the lines to find the whole-cue voice.
type voiceScan struct {
	voice             Voice
	closed            bool
	visibleAfterClose bool
}

func (v *voiceScan) text(part string) {
	if strings.Trim(part, PHPTrimSet) != "" {
		v.visibleAfterClose = v.visibleAfterClose || v.closed
		v.closed = false
	}
}

// tag takes a tag part. canStart is true for the tag that may start the voice. It returns false for a second voice.
func (v *voiceScan) tag(part string, canStart bool) bool {
	if part == "</v>" {
		v.closed = true
		return true
	}
	m := exactVoiceTagRegexp.FindStringSubmatch(part)
	if m == nil {
		return true
	}
	t := Voice{Name: strings.Trim(DecodeEntities(m[2]), PHPTrimSet), Class: m[1]}
	switch {
	case v.voice.Name == "" && canStart:
		v.voice = t
	case t != v.voice:
		return false
	}
	v.closed = false
	return true
}

// wholeCueVoice returns the voice that covers the lines, or false when they have none.
func wholeCueVoice(lines []string) (Voice, bool) {
	var v voiceScan
	firstVisible := slices.IndexFunc(lines, func(l string) bool { return HasVisibleText([]string{l}) })
	for i, line := range lines {
		parts := SplitTags(line)
		for j, part := range parts {
			if j%2 == 0 {
				v.text(part)
			} else if !v.tag(part, i == firstVisible && j == 1 && parts[0] == "") {
				return Voice{}, false
			}
		}
	}
	v.voice.Close = v.closed
	return v.voice, v.voice.Name != "" && !v.visibleAfterClose
}
