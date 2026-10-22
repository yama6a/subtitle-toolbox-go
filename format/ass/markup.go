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
	open   []openTag
	b      strings.Builder
}

// convert returns the core markup lines of text and the numpad value of its first \an or \a tag, or 0.
// The style flags open their tags at the start and after \r. A karaoke time of 100000 hours or more fails on line.
func (m *textMarkup) convert(text string, start time.Duration, hardBreak bool, line int, style record) ([]string, int, error) {
	m.open = nil
	m.b.Reset()
	breaks := softBreaks
	if hardBreak {
		breaks = hardBreaks
	}
	numpad := 0
	drawing := false
	karaoke := start
	m.openStyle(style)
	at := 0
	for _, loc := range append(overrideBlockRegexp.FindAllStringIndex(text, -1), []int{len(text), len(text)}) {
		if !drawing && loc[0] > at {
			m.b.WriteString(breaks.Replace(markupcore.EscapeText(text[at:loc[0]])))
		}
		at = loc[1]
		if loc[0] == loc[1] {
			continue
		}
		for _, tag := range overrideTags(text[loc[0]+1 : loc[1]-1]) {
			if n, ok := ssatag.Alignment(tag); ok {
				if numpad == 0 {
					numpad = n
				}
				continue
			}
			var err error
			if drawing, karaoke, err = m.tag(tag, drawing, karaoke, line, style); err != nil {
				return nil, 0, err
			}
		}
	}
	m.closeAll()
	return strings.Split(markupcore.RemoveEmptyTagPairs(m.b.String(), false, false), "\n"), numpad, nil
}

// tag applies one override tag other than an alignment tag and returns the new drawing state and karaoke time.
func (m *textMarkup) tag(tag string, drawing bool, karaoke time.Duration, line int, style record) (bool, time.Duration, error) {
	if g := toggleRegexp.FindStringSubmatch(tag); g != nil {
		start := ""
		if g[2] == "1" {
			start = "<" + g[1] + ">"
		}
		m.set(g[1], start)
	} else if g := colorRegexp.FindStringSubmatch(tag); g != nil {
		start := ""
		if g[1] != "" {
			start = fontTag(g[1])
		}
		m.set("font", start)
	} else if g := karaokeRegexp.FindStringSubmatch(tag); g != nil {
		m.b.WriteString("<" + markupcore.CoreTimestamp(karaoke) + ">")
		d, err := parsing.BoundedTime(karaoke+karaokeDuration(g[1], g[2]), tag, line)
		if err != nil {
			return drawing, karaoke, err
		}
		karaoke = d
	} else if g := resetRegexp.FindStringSubmatch(tag); g != nil {
		m.closeAll()
		next := style
		if phpTrim(g[1]) != "" {
			if s, ok := findStyle(m.styles, g[1]); ok {
				next = s
			}
		}
		m.openStyle(next)
	} else if g := drawingRegexp.FindStringSubmatch(tag); g != nil {
		drawing = strings.Trim(g[1], "0") != ""
	}
	return drawing, karaoke, nil
}

// karaokeDuration returns the time of a karaoke value in centiseconds from its whole and fraction digits.
// A value too large for a time.Duration returns timecode.MaxTime, which fails the bound check on its own.
func karaokeDuration(whole, fraction string) time.Duration {
	whole = strings.TrimLeft(whole, "0")
	if len(whole) >= maxKaraokeDigits {
		return timecode.MaxTime
	}
	var d time.Duration
	for i := range len(whole) {
		d = 10*d + time.Duration(whole[i]-'0')
	}
	d *= centisecond
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
//
// The scanner follows the PHP pattern /\\[^\\(]*(\((?:[^()]++|(?1))*\))?[^\\]*/, whose recursion RE2 lacks.
func overrideTags(content string) []string {
	var tags []string
	i := strings.IndexByte(content, '\\')
	for i >= 0 && i < len(content) {
		start := i
		i++
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
		tags = append(tags, phpTrim(content[start:i]))
	}
	return tags
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

// fontTag returns the <font> tag of an ASS color of 1 to 8 hex digits in BGR order. Short values pad on the
// left with zeros, and the last 6 digits count, so 8 digits drop the alpha byte.
func fontTag(hex string) string {
	hex = strings.Repeat("0", max(0, 6-len(hex))) + hex
	return `<font color="#` + strings.ToLower(markupcore.BGRToRGB(hex[len(hex)-6:])) + `">`
}
