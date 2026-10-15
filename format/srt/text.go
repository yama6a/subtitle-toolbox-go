package srt

import (
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/ssatag"
)

var (
	tagCandidateRegexp = regexp.MustCompile(`</?[a-zA-Z][a-zA-Z0-9]*(?:[\t\n\v\f\r ][^<>]*)?>`)
	// attributeTagRegexp matches a tag whose attributes are all name=value pairs, so "<a sentence>" is no tag.
	attributeTagRegexp = regexp.MustCompile(`^</?[a-zA-Z][a-zA-Z0-9]*` +
		`(?:[\t\n\v\f\r ]+[a-zA-Z_:][-a-zA-Z0-9_:.]*[\t\n\v\f\r ]*=[\t\n\v\f\r ]*(?:"[^"]*"|'[^']*'|[^\t\n\v\f\r "'=<>]+))*` +
		`[\t\n\v\f\r ]*/?>$`)
	overrideBlockRegexp = regexp.MustCompile(`\{\\[^{}]*\}`)
)

// cueText returns the text of the text lines of a cue in core markup, and the numpad value of its first
// alignment tag, or 0.
func cueText(lines []string) (string, int) {
	escaped := make([]string, len(lines))
	for i, line := range lines {
		escaped[i] = escapeLine(line)
	}
	return convertOverrideTags(strings.Join(markupcore.UnknownTagsAsText(escaped), "\n"))
}

// escapeLine escapes the text of line and each tag candidate that is no tag. Players show &amp; as typed,
// so every & is text.
func escapeLine(line string) string {
	locs := tagCandidateRegexp.FindAllStringIndex(line, -1)
	if locs == nil {
		return markupcore.EscapeText(line)
	}
	var b strings.Builder
	at := 0
	for _, loc := range locs {
		b.WriteString(markupcore.EscapeText(line[at:loc[0]]))
		tag := line[loc[0]:loc[1]]
		if isTag(tag) {
			b.WriteString(tag)
		} else {
			b.WriteString(markupcore.EscapeText(tag))
		}
		at = loc[1]
	}
	b.WriteString(markupcore.EscapeText(line[at:]))
	return b.String()
}

// isTag reports whether a candidate is a core tag or a tag with only name=value attributes.
func isTag(candidate string) bool {
	return markupcore.IsCoreTag(markupcore.TagName(strings.Trim(candidate, "</>"))) || attributeTagRegexp.MatchString(candidate)
}

// convertOverrideTags converts the ASS override blocks of text, such as {\an8} or {\i1}, and returns the
// text and the numpad value of the first alignment tag, or 0. A bold, italic, underline or strike toggle
// becomes a core tag. Other tags stay in one brace block after the converted tags.
func convertOverrideTags(text string) (string, int) {
	if !strings.Contains(text, `{\`) {
		return text, 0
	}
	numpad := 0
	var open []string
	text = overrideBlockRegexp.ReplaceAllStringFunc(text, func(block string) string {
		var tags, unknown strings.Builder
		for _, tag := range overrideTags(block[1 : len(block)-1]) {
			if n, ok := ssatag.Alignment(tag); ok {
				if numpad == 0 {
					numpad = n
				}
			} else if name, on, ok := toggle(tag); ok {
				tags.WriteString(switchTag(name, on, &open))
			} else {
				unknown.WriteString(tag)
			}
		}
		if unknown.Len() == 0 {
			return tags.String()
		}
		return tags.String() + "{" + unknown.String() + "}"
	})
	for i := len(open) - 1; i >= 0; i-- {
		text += "</" + open[i] + ">"
	}
	return text, numpad
}

// overrideTags splits the content of an override block before each backslash.
func overrideTags(content string) []string {
	var tags []string
	for {
		i := strings.IndexByte(content[1:], '\\')
		if i < 0 {
			return append(tags, content)
		}
		tags = append(tags, content[:i+1])
		content = content[i+1:]
	}
}

// toggle reads a tag such as \b1 or \i0. One final LF after it passes, as the PHP pattern end $ allows it.
func toggle(tag string) (name string, on, ok bool) {
	tag = strings.TrimSuffix(tag, "\n")
	if len(tag) != len(`\b1`) || !strings.ContainsRune("bius", rune(tag[1])) || tag[2] != '0' && tag[2] != '1' {
		return "", false, false
	}
	return tag[1:2], tag[2] == '1', true
}

// switchTag returns the start or end tag of name, or "" when the tag is already in that state.
// A closed tag leaves open in opening order.
func switchTag(name string, on bool, open *[]string) string {
	i := -1
	for j, n := range *open {
		if n == name {
			i = j
		}
	}
	switch {
	case on == (i >= 0):
		return ""
	case on:
		*open = append(*open, name)
		return "<" + name + ">"
	default:
		*open = append((*open)[:i], (*open)[i+1:]...)
		return "</" + name + ">"
	}
}
