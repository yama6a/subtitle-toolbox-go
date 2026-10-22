package ass

import (
	"regexp"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/ssatag"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
)

const (
	// nbsp is U+00A0, the hard space of \h.
	nbsp = "\xc2\xa0"
	// maxKaraokeDigits is the digit count of whole centiseconds from which a karaoke value reaches 100000 hours.
	maxKaraokeDigits = 12
	centisecond      = 10 * time.Millisecond
)

var (
	overrideBlockRegexp = regexp.MustCompile(`\{[^{}]*\}`)
	toggleRegexp        = regexp.MustCompile(`^\\([bius])([01]?)$`)
	colorRegexp         = regexp.MustCompile(`^\\1?c(?:&H([0-9A-Fa-f]{1,8})&?)?$`)
	karaokeRegexp       = regexp.MustCompile(`^\\(?:k|K|kf|ko)(\d+)(?:\.(\d+))?$`)
	resetRegexp         = regexp.MustCompile(`^\\r(.*)$`)
	drawingRegexp       = regexp.MustCompile(`^\\p(\d+)$`)
)

//nolint:gochecknoglobals // replacers hold no state
var (
	softBreaks = strings.NewReplacer(`\N`, "\n", `\n`, " ", `\h`, nbsp)
	// hardBreaks applies with WrapStyle 2, which makes \n a hard break.
	hardBreaks = strings.NewReplacer(`\N`, "\n", `\n`, "\n", `\h`, nbsp)
)

// openTag is a core tag that the text markup holds open, by name, with its start tag.
type openTag struct {
	name, start string
}

// textMarkup converts the Text field of one event to core markup.
type textMarkup struct {
	styles []record
	// style is the style of the event, line its 1-based line number.
	style   record
	line    int
	open    []openTag
	b       strings.Builder
	drawing bool
	karaoke time.Duration
}

// convert returns the core markup lines of text and the numpad value of its first \an or \a tag, or 0.
// The style flags open their tags at the start and after \r. A karaoke time of 100000 hours or more fails.
// The karaoke time starts at start.
func (m *textMarkup) convert(text string, start time.Duration, hardBreak bool) ([]string, int, error) {
	breaks := softBreaks
	if hardBreak {
		breaks = hardBreaks
	}
	m.karaoke = start
	m.openStyle(m.style)
	numpad := 0
	at := 0
	for _, loc := range append(overrideBlockRegexp.FindAllStringIndex(text, -1), []int{len(text), len(text)}) {
		if !m.drawing && loc[0] > at {
			m.b.WriteString(breaks.Replace(markupcore.EscapeText(text[at:loc[0]])))
		}
		at = loc[1]
		if loc[0] == loc[1] {
			continue
		}
		var err error
		if numpad, err = m.block(text[loc[0]+1:loc[1]-1], numpad); err != nil {
			return nil, 0, err
		}
	}
	m.closeAll()
	return strings.Split(markupcore.RemoveEmptyTagPairs(m.b.String(), false, false), "\n"), numpad, nil
}

// block applies the tags of the override block content and returns numpad, or the alignment of its first
// alignment tag when numpad is 0.
func (m *textMarkup) block(content string, numpad int) (int, error) {
	for _, tag := range overrideTags(content) {
		n, ok := ssatag.Alignment(tag)
		switch {
		case ok && numpad == 0:
			numpad = n
		case ok:
		default:
			if err := m.tag(tag); err != nil {
				return 0, err
			}
		}
	}
	return numpad, nil
}

// tag applies one override tag other than an alignment tag.
func (m *textMarkup) tag(tag string) error {
	if g := toggleRegexp.FindStringSubmatch(tag); g != nil {
		start := ""
		if g[2] == "1" {
			start = "<" + g[1] + ">"
		}
		m.set(g[1], start)
		return nil
	}
	if g := colorRegexp.FindStringSubmatch(tag); g != nil {
		m.set("font", fontTag(g[1]))
		return nil
	}
	if g := karaokeRegexp.FindStringSubmatch(tag); g != nil {
		m.b.WriteString("<" + markupcore.CoreTimestamp(m.karaoke) + ">")
		d, err := parsing.BoundedTime(m.karaoke+karaokeDuration(g[1], g[2]), tag, m.line)
		m.karaoke = d
		return err
	}
	if g := resetRegexp.FindStringSubmatch(tag); g != nil {
		m.reset(g[1])
		return nil
	}
	if g := drawingRegexp.FindStringSubmatch(tag); g != nil {
		m.drawing = strings.Trim(g[1], "0") != ""
	}
	return nil
}

// reset closes all tags, then opens the flags of the style name, or of the event style when name is empty or
// unknown.
func (m *textMarkup) reset(name string) {
	m.closeAll()
	next := m.style
	if phpTrim(name) != "" {
		if s, ok := findStyle(m.styles, name); ok {
			next = s
		}
	}
	m.openStyle(next)
}

// karaokeDuration returns the time of a karaoke value in centiseconds from its whole and fraction digits.
// A value too large for a time.Duration returns timecode.MaxTime, which fails the bound check on its own.
func karaokeDuration(whole, fraction string) time.Duration {
	whole = strings.TrimLeft(whole, "0")
	if len(whole) >= maxKaraokeDigits {
		return timecode.MaxTime
	}
	var centiseconds int64
	for i := range len(whole) {
		centiseconds = 10*centiseconds + int64(whole[i]-'0')
	}
	d := time.Duration(centiseconds * int64(centisecond))
	unit := centisecond
	for i := 0; i < len(fraction) && unit > 1; i++ {
		unit /= 10
		d += time.Duration(fraction[i]-'0') * unit
	}
	return d
}

// overrideTags splits the content of an override block into trimmed tags. A tag starts at a backslash and ends
// before the next one, except inside balanced parentheses, so \t(\1c&HFF&) stays one tag. Text before the first
// backslash is no tag.
func overrideTags(content string) []string {
	var tags []string
	for i := strings.IndexByte(content, '\\'); i >= 0 && i < len(content); {
		end := tagEnd(content, i)
		tags = append(tags, phpTrim(content[i:end]))
		i = end
	}
	return tags
}

// tagEnd returns the end of the tag at start. It follows the PHP pattern /\\[^\\(]*(\((?:[^()]++|(?1))*\))?[^\\]*/,
// whose recursion RE2 lacks.
func tagEnd(content string, start int) int {
	i := start + 1
	for i < len(content) && content[i] != '\\' && content[i] != '(' {
		i++
	}
	if i < len(content) && content[i] == '(' {
		if end, ok := balancedEnd(content, i); ok {
			i = end
		}
	}
	for i < len(content) && content[i] != '\\' {
		i++
	}
	return i
}

// balancedEnd returns the index after the parenthesis that closes the one at open.
func balancedEnd(s string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

// set opens, replaces or closes the tag name. The tags opened after it close first and open again, so the tags
// always nest. An empty start closes the tag.
func (m *textMarkup) set(name, start string) {
	i := -1
	for j, t := range m.open {
		if t.name == name {
			i = j
			break
		}
	}
	if i >= 0 {
		if m.open[i].start == start {
			return
		}
		inner := append([]openTag(nil), m.open[i+1:]...)
		for j := len(m.open) - 1; j >= i; j-- {
			m.b.WriteString("</" + m.open[j].name + ">")
		}
		m.open = m.open[:i]
		for _, t := range inner {
			m.b.WriteString(t.start)
			m.open = append(m.open, t)
		}
	}
	if start != "" {
		m.b.WriteString(start)
		m.open = append(m.open, openTag{name, start})
	}
}

func (m *textMarkup) openStyle(style record) {
	for _, name := range styleTags(style) {
		m.set(name, "<"+name+">")
	}
}

func (m *textMarkup) closeAll() {
	for j := len(m.open) - 1; j >= 0; j-- {
		m.b.WriteString("</" + m.open[j].name + ">")
	}
	m.open = nil
}

// fontTag returns the <font> tag of an ASS color of 1 to 8 hex digits in BGR order, or "" for no digits.
// Short values pad on the left with zeros, and the last 6 digits count, so 8 digits drop the alpha byte.
func fontTag(hex string) string {
	if hex == "" {
		return ""
	}
	hex = strings.Repeat("0", max(0, 6-len(hex))) + hex
	return `<font color="#` + strings.ToLower(markupcore.BGRToRGB(hex[len(hex)-6:])) + `">`
}
