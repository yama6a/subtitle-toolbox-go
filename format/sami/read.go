package sami

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var (
	titleRegexp     = regexp.MustCompile(headPattern("title"))
	samiParamRegexp = regexp.MustCompile(headPattern("samiparam"))
	styleRegexp     = regexp.MustCompile(headPattern("style"))
	classRuleRegexp = regexp.MustCompile(`\.([A-Za-z_][\w-]*)\s*\{([^}]*)\}`)
	bodyStartRegexp = regexp.MustCompile(`<` + fold("body") + `\b[^>]*>`)
	bodyEndRegexp   = regexp.MustCompile(`</` + fold("body") + `\s*>`)
	syncEndRegexp   = regexp.MustCompile(`</` + fold("sync") + `\s*>`)
	syncRegexp      = regexp.MustCompile(`<` + fold("sync") + `\b`)
	anySyncRegexp   = regexp.MustCompile(`<` + fold("sync"))
	startRegexp     = regexp.MustCompile(`\b` + fold("start") + `\s*=\s*["']?\s*(-?\d+)`)
	headStartRegexp = regexp.MustCompile(`\A<` + fold("head") + `\b`)
	headEndRegexp   = regexp.MustCompile(`</` + fold("head") + `\s*>`)
)

func headPattern(tag string) string {
	return `(?s)<` + fold(tag) + `\b[^>]*>(.*?)</` + fold(tag) + `\s*>`
}

// names holds class names by their lower-case key, in the order of their first use. The first spelling wins.
type names struct {
	keys    []string
	written map[string]string
}

func (n *names) add(name string) bool {
	key := markupcore.ASCIILower(name)
	if _, ok := n.written[key]; ok {
		return false
	}
	if n.written == nil {
		n.written = map[string]string{}
	}
	n.keys = append(n.keys, key)
	n.written[key] = name
	return true
}

// styleClasses are the class rules of the STYLE block.
type styleClasses struct {
	names
	lang map[string]string
}

type syncBlock struct {
	start      time.Duration
	paragraphs []paragraph
}

type reader struct {
	st      *parsing.State
	opts    ReadOptions
	content string
}

// Read reads SAMI text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.SAMI, err)
	}
	st := parsing.NewState(model.SAMI, common)
	content, ok, err := st.Prepare(content, true)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	rd := &reader{st: st, opts: opts, content: markupcore.NormalizeLineEnds(string(content))}
	if err := rd.read(s); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func (rd *reader) read(s *model.Subtitle) error {
	data := model.NewData()
	classes := rd.readHead(s, data)
	blocks, err := rd.syncs()
	if err != nil {
		return err
	}
	class, hasClass, err := rd.chooseClass(classes, blocks)
	if err != nil {
		return err
	}
	if hasClass {
		data.Set("class", class)
		if lang := classes.lang[markupcore.ASCIILower(class)]; lang != "" {
			s.SetMetadata(model.MetadataLanguage, lang)
		}
	}
	if err := s.SetFormatData(formatDataKey, data); err != nil {
		return fmt.Errorf("%s file data: %w", model.SAMI, err)
	}

	cues, err := rd.cues(blocks, class, hasClass)
	if err != nil {
		return err
	}
	s.AddCues(cues...)
	return rd.st.NoCues(len(cues), rd.textLine)
}

// readHead reads the title into s, and the SAMIParam and STYLE blocks into data. It returns the classes of STYLE.
func (rd *reader) readHead(s *model.Subtitle, data *model.Data) styleClasses {
	if m := titleRegexp.FindStringSubmatch(rd.content); m != nil {
		if title := strings.Trim(m[1], markupcore.PHPTrimSet); title != "" {
			s.SetMetadata(model.MetadataTitle, markupcore.DecodeEntities(title))
		}
	}
	if m := samiParamRegexp.FindStringSubmatch(rd.content); m != nil {
		data.Set("samiParam", m[1])
	}
	m := styleRegexp.FindStringSubmatch(rd.content)
	if m == nil {
		return styleClasses{}
	}
	data.Set("style", m[1])
	return readClasses(m[1])
}

// readClasses reads the .CLASS rules of a STYLE block. The lang of a rule is its last lang declaration.
func readClasses(style string) styleClasses {
	classes := styleClasses{lang: map[string]string{}}
	for _, m := range classRuleRegexp.FindAllStringSubmatch(stripCSSComments(style), -1) {
		lang := ""
		for declaration := range strings.SplitSeq(m[2], ";") {
			name, value, ok := strings.Cut(declaration, ":")
			if ok && markupcore.ASCIILower(strings.Trim(name, markupcore.PHPTrimSet)) == "lang" {
				lang = strings.Trim(value, " \t\n\"'")
			}
		}
		if classes.add(m[1]) {
			classes.lang[markupcore.ASCIILower(m[1])] = lang
		}
	}
	return classes
}

// stripCSSComments removes the matches of /\/\*.*?\*\/|<!--|-->/s. A regular expression takes quadratic time on many
// /* without */.
func stripCSSComments(style string) string {
	var b strings.Builder
	closed := true
	for i := 0; i < len(style); {
		rest := style[i:]
		switch {
		case closed && strings.HasPrefix(rest, "/*"):
			if end := strings.Index(rest[2:], "*/"); end >= 0 {
				i += 2 + end + 2
				continue
			}
			closed = false
		case strings.HasPrefix(rest, "<!--"):
			i += len("<!--")
			continue
		case strings.HasPrefix(rest, "-->"):
			i += len("-->")
			continue
		}
		b.WriteByte(style[i])
		i++
	}
	return b.String()
}

func (rd *reader) bodyStart() int {
	if loc := bodyStartRegexp.FindStringIndex(rd.content); loc != nil {
		return loc[1]
	}
	return 0
}

// syncs returns the SYNC blocks of the body, sorted by start.
func (rd *reader) syncs() ([]syncBlock, error) {
	bodyStart := rd.bodyStart()
	body := rd.content[bodyStart:]
	if loc := bodyEndRegexp.FindStringIndex(body); loc != nil {
		body = body[:loc[0]]
	}
	body = syncEndRegexp.ReplaceAllLiteralString(body, "")

	line := 1 + strings.Count(rd.content[:bodyStart], "\n")
	counted := 0
	locs := syncRegexp.FindAllStringIndex(body, -1)
	var blocks []syncBlock
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		line += strings.Count(body[counted:loc[1]], "\n")
		counted = loc[1]
		b, ok, err := rd.syncBlock(body[loc[1]:end], i, line)
		if err != nil {
			return nil, err
		}
		if ok {
			blocks = append(blocks, b)
		}
	}
	slices.SortStableFunc(blocks, func(a, b syncBlock) int { return cmp.Compare(a.start, b.start) })
	return blocks, nil
}

// syncBlock reads the block after the <SYNC of block index on line. False means that lenient mode skipped it.
func (rd *reader) syncBlock(chunk string, index, line int) (syncBlock, bool, error) {
	lines := func() []string {
		var out []string
		for l := range strings.SplitSeq("<SYNC"+chunk, "\n") {
			if l = strings.Trim(l, markupcore.PHPTrimSet); l != "" {
				out = append(out, l)
			}
		}
		return out
	}
	tag, content, found := strings.Cut(chunk, ">")
	var m []string
	if found {
		m = startRegexp.FindStringSubmatch(tag)
	}
	if m == nil {
		err := &model.ParseError{Line: line, Msg: "SYNC tag " + strconv.Itoa(index+1) + " has no valid Start attribute."}
		return syncBlock{}, false, rd.st.Fail(err, line, index, lines())
	}
	start, negative, err := startTime(m[1], line)
	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return syncBlock{}, false, err
		}
		return syncBlock{}, false, rd.st.Fail(perr, line, index, lines())
	}
	if negative && rd.st.Lenient() {
		rd.st.Warn("SYNC tag "+strconv.Itoa(index+1)+" has a negative Start. The parser read it as 0.",
			line, index, lines(), model.Repaired)
	}
	return syncBlock{start: start, paragraphs: readParagraphs(content)}, true, nil
}

// startTime reads the milliseconds of a Start value. True means a negative value, for any number of digits.
func startTime(value string, line int) (time.Duration, bool, error) {
	digits, negative := strings.CutPrefix(value, "-")
	digits = strings.TrimLeft(digits, "0")
	if negative && digits != "" {
		return 0, true, nil
	}
	if len(digits) > len(strconv.FormatInt(timecode.MaxTime.Milliseconds(), 10)) {
		_, err := parsing.BoundedTime(timecode.MaxTime, value, line)
		return 0, false, err
	}
	ms, _ := strconv.ParseInt("0"+digits, 10, 64) // at most 12 digits
	d, err := parsing.BoundedTime(time.Duration(ms)*time.Millisecond, value, line)
	return d, false, err
}

// chooseClass returns the class to read. False means no class, so every paragraph counts.
func (rd *reader) chooseClass(classes styleClasses, blocks []syncBlock) (string, bool, error) {
	var used names
	for _, b := range blocks {
		for _, p := range b.paragraphs {
			if p.hasClass {
				used.add(p.class)
			}
		}
	}

	if want := rd.opts.LanguageClass; want != "" {
		return optionClass(want, classes, used)
	}
	class, hasClass, matched := chooseDefaultClass(classes, used)
	if !matched && rd.st.Lenient() {
		line, lines := rd.classLine(class)
		rd.st.Warn("No <P> class matches a class of the STYLE block. The parser read the class "+class+".",
			line, model.NoBlock, lines, model.Repaired)
	}
	return class, hasClass, nil
}

func optionClass(want string, classes styleClasses, used names) (string, bool, error) {
	key := markupcore.ASCIILower(want)
	if name, ok := classes.written[key]; ok {
		return name, true, nil
	}
	if name, ok := used.written[key]; ok {
		return name, true, nil
	}
	return "", false, &model.ParseError{Format: model.SAMI, Msg: "The SAMI file has no class " + want + "."}
}

// chooseDefaultClass picks the class without the option. False for matched means that no <P> class is in STYLE.
func chooseDefaultClass(classes styleClasses, used names) (class string, hasClass, matched bool) {
	switch {
	case len(classes.keys) == 0 && len(used.keys) == 0:
		return "", false, true
	case len(classes.keys) == 0:
		return used.written[used.keys[0]], true, true
	case len(used.keys) == 0:
		return classes.written[classes.keys[0]], true, true
	}
	for _, key := range classes.keys {
		if _, ok := used.written[key]; ok {
			return classes.written[key], true, true
		}
	}
	return used.written[used.keys[0]], true, false
}

// classLine returns the first line from the body start on that holds Class=class, and that line trimmed.
func (rd *reader) classLine(class string) (int, []string) {
	start := rd.bodyStart()
	if start > 0 && rd.content[start-1] != '\n' {
		next := strings.IndexByte(rd.content[start:], '\n')
		if next < 0 {
			return model.NoLine, nil
		}
		start += next + 1
	}
	re := regexp.MustCompile(`(?m)^.*\b` + fold("class") + `\s*=\s*["']?` + fold(regexp.QuoteMeta(class)) + `\b.*$`)
	loc := re.FindStringIndex(rd.content[start:])
	if loc == nil {
		return model.NoLine, nil
	}
	match := rd.content[start+loc[0] : start+loc[1]]
	return 1 + strings.Count(rd.content[:start+loc[0]], "\n"), []string{strings.Trim(match, markupcore.PHPTrimSet)}
}

// cues returns a cue for each block whose paragraphs of the class hold lines. A cue ends at the next block that
// touches the class.
func (rd *reader) cues(blocks []syncBlock, class string, hasClass bool) ([]model.Cue, error) {
	var cues []model.Cue
	open := -1
	for _, b := range blocks {
		touching := touchingParagraphs(b, class, hasClass)
		if touching == nil {
			continue
		}
		if open >= 0 {
			cues[open].End = b.start
			open = -1
		}
		var lines []string
		for _, p := range touching {
			lines = append(lines, p.lines...)
		}
		if lines == nil {
			continue
		}
		c := model.Cue{Start: b.start, End: b.start, Lines: lines}
		if err := c.SetFormatData(formatDataKey, cueData(touching, lines)); err != nil {
			return nil, fmt.Errorf("%s cue data: %w", model.SAMI, err)
		}
		cues = append(cues, c)
		open = len(cues) - 1
	}
	if open >= 0 {
		cues[open].End = rd.st.LastCueEnd(cues[open].Start)
	}
	return cues, nil
}

// touchingParagraphs returns the paragraphs of b that have the class or no class. Without a class, every paragraph
// counts.
func touchingParagraphs(b syncBlock, class string, hasClass bool) []paragraph {
	var touching []paragraph
	for _, p := range b.paragraphs {
		if !p.hasClass || !hasClass || markupcore.ASCIILower(p.class) == markupcore.ASCIILower(class) {
			touching = append(touching, p)
		}
	}
	return touching
}

func cueData(paragraphs []paragraph, lines []string) *model.Data {
	list := make([]any, len(paragraphs))
	for i, p := range paragraphs {
		var attributes any = []any{}
		if len(p.attrs) > 0 {
			d := model.NewData()
			for _, a := range p.attrs {
				d.Set(a.name, a.value)
			}
			attributes = d
		}
		list[i] = model.NewData(model.Field{Key: "attributes", Value: attributes}, model.Field{Key: "html", Value: p.html})
	}
	stored := make([]any, len(lines))
	for i, l := range lines {
		stored[i] = l
	}
	return model.NewData(model.Field{Key: "paragraphs", Value: list}, model.Field{Key: "lines", Value: stored})
}

const nbspEntity = "&nbsp;"

// textLine returns the line of the first text outside tags, comments, the head and &nbsp;, for content without any
// <SYNC. A file with SYNC tags can give no cues on purpose, for example when the chosen class has no text.
func (rd *reader) textLine() (int, bool) {
	if anySyncRegexp.MatchString(rd.content) {
		return 0, false
	}
	c := rd.content
	m := markupFinder{
		heads:    finder{find: headEndRegexp.FindStringIndex},
		comments: finder{find: func(s string) []int { return index(s, "-->") }},
		tags:     finder{find: func(s string) []int { return index(s, ">") }},
	}
	for i := 0; i < len(c); {
		if end := m.end(c, i); end > i {
			i = end
			continue
		}
		if strings.IndexByte(markupcore.PHPTrimSet, c[i]) < 0 {
			return 1 + strings.Count(c[:i], "\n"), true
		}
		i++
	}
	return 0, false
}

// markupFinder finds the matches of /<HEAD\b.*?<\/HEAD\s*>|<!--.*?-->|<[^>]*>|&nbsp;/is of PHP in linear time.
type markupFinder struct {
	heads, comments, tags finder
}

// end returns the end of the match at i, or i when none starts there.
func (m *markupFinder) end(c string, i int) int {
	switch {
	case len(c)-i >= len(nbspEntity) && markupcore.ASCIILower(c[i:i+len(nbspEntity)]) == nbspEntity:
		return i + len(nbspEntity)
	case c[i] != '<':
		return i
	case headStartRegexp.MatchString(c[i:]) && m.heads.next(c, i+len("<head")) >= 0:
		return m.heads.end
	case strings.HasPrefix(c[i:], "<!--") && m.comments.next(c, i+len("<!--")) >= 0:
		return m.comments.end
	case m.tags.next(c, i+1) >= 0:
		return m.tags.end
	}
	return i
}

// finder finds the first match of find at or after an offset. It keeps its last match, so a run of starts without
// an end, such as many <HEAD without </HEAD>, costs one search in place of one search each.
type finder struct {
	find       func(string) []int
	searched   bool
	start, end int
}

// next returns the end of the first match at or after from, or -1. Calls must not go back before an earlier from.
func (f *finder) next(c string, from int) int {
	if !f.searched || f.start >= 0 && f.start < from {
		f.searched, f.start, f.end = true, -1, -1
		if loc := f.find(c[from:]); loc != nil {
			f.start, f.end = from+loc[0], from+loc[1]
		}
	}
	return f.end
}

func index(s, sub string) []int {
	if i := strings.Index(s, sub); i >= 0 {
		return []int{i, i + len(sub)}
	}
	return nil
}
