package markupcore

import (
	"regexp"
	"slices"
	"strings"
)

const (
	// TagPattern matches a tag such as <b> or </font>. A "<" before white space starts no tag, and a quote does not hide the ">".
	TagPattern = `<(?:[^<> \t\n\r\f\v][^<>]*)?>`
	// VoiceTagStartPattern matches a <v> tag up to its name. Group 1 holds the classes, such as ".loud".
	VoiceTagStartPattern = `<v(\.[^ \t\n\r\f\v>]*)?`
	// VoiceTagPattern matches a <v> tag with a name. Group 1 holds the classes and group 2 the name.
	VoiceTagPattern = VoiceTagStartPattern + `[ \t\n\r\f\v]+([^>]*)>`
	// WordTimestampPattern matches a word timestamp such as <00:01:02.500>.
	WordTimestampPattern = `<\d{2,}:[0-5]\d:[0-5]\d\.\d{3}>`
	// EntityPattern matches an entity such as &amp;, &#233; or &#xE9;.
	EntityPattern = `&(?:[a-zA-Z][a-zA-Z0-9]*|#[0-9]+|#[xX][0-9a-fA-F]+);`
)

const (
	asciiSpace = " \t\n\r\f\v"
	phpTrimSet = " \t\n\r\x00\v"
)

var (
	tagRegexp        = regexp.MustCompile(TagPattern)
	unknownTagRegexp = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)>`)
)

// IsStyleTag reports whether the lower-case name is one of the style tags b, i, u, s and font.
func IsStyleTag(name string) bool {
	switch name {
	case "b", "i", "u", "s", "font":
		return true
	}
	return false
}

// IsCoreTag reports whether the lower-case name is a style tag or v.
func IsCoreTag(name string) bool {
	return name == "v" || IsStyleTag(name)
}

// SplitTags splits a line at its tags, with the text runs at the even indexes and the tags at the odd indexes.
func SplitTags(line string) []string {
	locs := tagRegexp.FindAllStringIndex(line, -1)
	parts := make([]string, 0, 2*len(locs)+1)
	at := 0
	for _, loc := range locs {
		parts = append(parts, line[at:loc[0]], line[loc[0]:loc[1]])
		at = loc[1]
	}
	return append(parts, line[at:])
}

// TagName returns the first word of a tag body such as `FONT color="yellow"`, in lower case.
func TagName(body string) string {
	body = strings.Trim(body, phpTrimSet)
	if i := strings.IndexAny(body, asciiSpace); i >= 0 {
		body = body[:i]
	}
	return asciiLower(body)
}

// StripAllTags removes every tag, word timestamps too. Entities stay.
func StripAllTags(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	return tagRegexp.ReplaceAllString(text, "")
}

// KeepTags removes every tag whose name is not in names. The name of <c.yellow> is c.yellow, compared without case.
func KeepTags(text string, names ...string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	keep := make([]string, len(names))
	for i, n := range names {
		keep[i] = asciiLower(n)
	}
	return tagRegexp.ReplaceAllStringFunc(text, func(tag string) string {
		body := strings.TrimPrefix(tag[1:], "/")
		name := body[:strings.IndexAny(body, asciiSpace+"/>")]
		if name != "" && slices.Contains(keep, asciiLower(name)) {
			return tag
		}
		return ""
	})
}

// HasVisibleText reports whether a line holds more than white space outside its tags. An entity counts as text.
func HasVisibleText(lines []string) bool {
	for _, line := range lines {
		if strings.Trim(StripAllTags(line), phpTrimSet) != "" {
			return true
		}
	}
	return false
}

// UnknownTagsAsText escapes a start tag such as <today> as text when it has no attributes, its name is no core,
// WebVTT or <br> tag, and no line of the cue holds its end tag. FFmpeg writes such text from ASS.
func UnknownTagsAsText(lines []string) []string {
	text := asciiLower(strings.Join(lines, "\n"))
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = unknownTagRegexp.ReplaceAllStringFunc(line, func(tag string) string {
			name := tag[1 : len(tag)-1]
			lower := asciiLower(name)
			if isKnownTag(lower) || hasEndTag(text, lower) {
				return tag
			}
			return "&lt;" + name + "&gt;"
		})
	}
	return out
}

func isKnownTag(name string) bool {
	switch name {
	case "c", "lang", "ruby", "rt", "br":
		return true
	}
	return IsCoreTag(name)
}

func hasEndTag(text, name string) bool {
	for {
		i := strings.Index(text, "</"+name)
		if i < 0 {
			return false
		}
		text = text[i+2+len(name):]
		if strings.HasPrefix(strings.TrimLeft(text, asciiSpace), ">") {
			return true
		}
	}
}

// RemoveEmptyTagPairs removes style tag pairs without text, such as <i></i> or <b><i></i></b>, until none is left.
// With withSpaces, a pair that holds only white space goes too and leaves the white space.
func RemoveEmptyTagPairs(text string, ignoreCase, withSpaces bool) string {
	for {
		out, removed := removeEmptyTagPairsOnce(text, ignoreCase, withSpaces)
		if !removed {
			return out
		}
		text = out
	}
}

func removeEmptyTagPairsOnce(text string, ignoreCase, withSpaces bool) (string, bool) {
	var b strings.Builder
	copied := 0
	for i := 0; ; i++ {
		j := strings.IndexByte(text[i:], '<')
		if j < 0 {
			break
		}
		i += j
		if end, spaces, ok := emptyTagPairAt(text, i, ignoreCase, withSpaces); ok {
			b.WriteString(text[copied:i])
			b.WriteString(spaces)
			copied = end
			i = end - 1
		}
	}
	if copied == 0 {
		return text, false
	}
	b.WriteString(text[copied:])
	return b.String(), true
}

// emptyTagPairAt matches <name ...>spaces</name> at text[i]. It returns the end of the pair and the spaces.
func emptyTagPairAt(text string, i int, ignoreCase, withSpaces bool) (end int, spaces string, ok bool) {
	name, inner, ok := styleTagAt(text[i+1:], ignoreCase)
	if !ok {
		return 0, "", false
	}
	if withSpaces {
		spaces = inner[:len(inner)-len(strings.TrimLeft(inner, asciiSpace))]
	}
	closing := inner[len(spaces):]
	n := len(name) + 3
	if len(closing) < n || closing[:2] != "</" || !sameName(closing[2:n-1], name, ignoreCase) || closing[n-1] != '>' {
		return 0, "", false
	}
	return len(text) - len(closing) + n, spaces, true
}

// styleTagAt matches the rest of an opening style tag such as `font color="red">` at the start of s. It returns the
// name as written and the text after the tag.
func styleTagAt(s string, ignoreCase bool) (name, after string, ok bool) {
	for _, n := range [...]string{"b", "i", "u", "s", "font"} {
		if len(s) > len(n) && sameName(s[:len(n)], n, ignoreCase) {
			name = s[:len(n)]
			break
		}
	}
	if name == "" || strings.IndexByte(asciiSpace+".>", s[len(name)]) < 0 {
		return "", "", false
	}
	k := strings.IndexAny(s[len(name):], "<>")
	if k < 0 || s[len(name)+k] != '>' {
		return "", "", false
	}
	return name, s[len(name)+k+1:], true
}

func sameName(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return asciiLower(a) == asciiLower(b)
	}
	return a == b
}

// TagSpan is the place of a tag in the lines of a cue: the line index, the byte offset and the byte length.
type TagSpan struct {
	Line, Offset, Length int
}

// UnbalancedTags returns the closing tags of the lower-case names that close no open tag, and the names that are
// still open after the last line. A closing tag closes the last open tag of its name. Names compare without case.
func UnbalancedTags(lines, names []string) (stray []TagSpan, open []string) {
	for li, line := range lines {
		for _, loc := range tagRegexp.FindAllStringIndex(line, -1) {
			body := line[loc[0]+1 : loc[1]-1]
			closing := strings.HasPrefix(body, "/")
			name, ok := namedTag(strings.TrimPrefix(body, "/"), names)
			switch {
			case !ok:
			case !closing:
				open = append(open, name)
			default:
				if i := lastIndex(open, name); i >= 0 {
					open = slices.Delete(open, i, i+1)
				} else {
					stray = append(stray, TagSpan{Line: li, Offset: loc[0], Length: loc[1] - loc[0]})
				}
			}
		}
	}
	return stray, open
}

// namedTag returns the name in names that body starts with, followed by white space, a dot or the end.
func namedTag(body string, names []string) (string, bool) {
	for _, n := range names {
		if len(body) >= len(n) && asciiLower(body[:len(n)]) == n &&
			(len(body) == len(n) || strings.IndexByte(asciiSpace+".", body[len(n)]) >= 0) {
			return n, true
		}
	}
	return "", false
}

func lastIndex(s []string, v string) int {
	for i, x := range slices.Backward(s) {
		if x == v {
			return i
		}
	}
	return -1
}

// OpenTag is a core tag that is still open: its lower-case name and the opening tag as written.
type OpenTag struct {
	Name, Tag string
}

// OpenCoreTags returns the core tags that are open after text, given the tags that are open before it.
func OpenCoreTags(text string, open []OpenTag) []OpenTag {
	open = slices.Clone(open)
	for _, tag := range tagRegexp.FindAllString(text, -1) {
		body, closing := strings.CutPrefix(tag[1:], "/")
		n := 0
		for n < len(body) && isASCIILetter(body[n]) {
			n++
		}
		name := asciiLower(body[:n])
		if !IsCoreTag(name) {
			continue
		}
		if !closing {
			open = append(open, OpenTag{Name: name, Tag: tag})
			continue
		}
		for i := len(open) - 1; i >= 0; i-- {
			if open[i].Name == name {
				open = slices.Delete(open, i, i+1)
				break
			}
		}
	}
	return open
}

// CloseCoreTags returns the closing tags of open, the last opened tag first.
func CloseCoreTags(open []OpenTag) string {
	var b strings.Builder
	for _, t := range slices.Backward(open) {
		b.WriteString("</" + t.Name + ">")
	}
	return b.String()
}

// CarryOpenTags starts each line with the tags that the lines before it leave open, for writers that style each line
// alone. It first turns WebVTT color classes into <font> tags, so that a color can span lines too.
func CarryOpenTags(lines []string) []string {
	lines = strings.Split(WebVTTColorsToFont(strings.Join(lines, "\n")), "\n")
	var open []OpenTag
	for i, line := range lines {
		var prefix strings.Builder
		for _, t := range open {
			prefix.WriteString(t.Tag)
		}
		lines[i] = prefix.String() + line
		open = OpenCoreTags(line, open)
	}
	return lines
}

func isASCIILetter(c byte) bool {
	return 'a' <= c|0x20 && c|0x20 <= 'z'
}

// asciiLower lowers A to Z only, as PHP strtolower does. It keeps every other byte, also of invalid UTF-8.
func asciiLower(s string) string {
	i := strings.IndexFunc(s, func(r rune) bool { return 'A' <= r && r <= 'Z' })
	if i < 0 {
		return s
	}
	b := []byte(s)
	for j := i; j < len(b); j++ {
		if 'A' <= b[j] && b[j] <= 'Z' {
			b[j] += 'a' - 'A'
		}
	}
	return string(b)
}
