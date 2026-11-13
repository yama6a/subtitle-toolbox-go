package speakers

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/hilabels"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// fromLabels turns the label at the start of a line, or after its dialogue dash, into a voice tag. A label alone
// on a line gives its tag to the next line.
func fromLabels(lines []string, anyCase bool) []string {
	out := make([]string, 0, len(lines))
	var pendingTag, pendingTags string
	for i, line := range lines {
		name, rest, ok := splitLabel(line, anyCase)
		if !ok {
			out = append(out, pendingTag+pendingTags+line)
			pendingTag, pendingTags = "", ""
			continue
		}
		tag := markupcore.VoiceTag(name, "")
		switch {
		case markupcore.VisibleText(rest) != "":
			out = append(out, tag+pendingTags+rest)
			pendingTag, pendingTags = "", ""
		case i == len(lines)-1:
			out = append(out, line)
			pendingTag, pendingTags = "", ""
		default:
			out = append(out, "")
			pendingTag, pendingTags = tag, pendingTags+styleTags(line)
		}
	}
	if slices.Equal(out, lines) {
		return lines
	}
	return markupcore.CleanLines(out)
}

// styleTags returns the tags of line that are not voice tags, in order.
func styleTags(line string) string {
	var b strings.Builder
	for i, tok := range markupcore.SplitTags(line) {
		if _, voice := voiceName(tok); i%2 == 1 && !voice {
			b.WriteString(tok)
		}
	}
	return b.String()
}

// splitLabel returns the speaker name of the label of line and the line without the dash and the label.
func splitLabel(line string, anyCase bool) (name, rest string, ok bool) {
	rest = hilabels.RemoveLabel(line, anyCase)
	if rest == markupcore.CleanLine(line) {
		return "", line, false
	}
	visible := markupcore.VisibleText(line)
	label, _, _ := strings.Cut(visible[hilabels.AfterLead(visible, 0):], ":")
	name = strings.Trim(label, markupcore.PHPTrimSet)
	if p := hilabels.SkipHSpace(visible, 0); p < len(visible) && visible[p] == '-' {
		rest = removeLeadingDash(rest)
	}
	if !strings.ContainsFunc(name, func(r rune) bool { return unicode.Is(unicode.Ll, r) }) {
		name = titleCase(name)
	}
	return name, rest, true
}

// removeLeadingDash removes a dialogue dash and the horizontal space around it from the start of the text, after
// the tags that open the line.
func removeLeadingDash(line string) string {
	parts := markupcore.SplitTags(line)
	k := 0
	for k+1 < len(parts) && parts[k] == "" {
		k += 2
	}
	text := parts[k]
	p := hilabels.SkipHSpace(text, 0)
	if p >= len(text) || text[p] != '-' {
		return line
	}
	parts[k] = text[hilabels.SkipHSpace(text, p+1):]
	return strings.Join(parts, "")
}

// titleCase turns an upper case name such as "DR. O'NEIL" into "Dr. O'Neil".
func titleCase(name string) string {
	if !utf8.ValidString(name) {
		return name
	}
	var b strings.Builder
	atStart := true
	for _, r := range cases.Lower(language.Und).String(name) {
		if atStart && unicode.Is(unicode.Ll, r) {
			b.WriteString(cases.Upper(language.Und).String(string(r)))
		} else {
			b.WriteRune(r)
		}
		atStart = markupcore.IsSpace(r) || r == '.' || r == '\'' || r == '-'
	}
	return b.String()
}

// rename writes the new name of each voice tag whose name is a key of names. The class stays.
func rename(lines []string, names map[string]string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = voiceTagAny.ReplaceAllStringFunc(line, func(tag string) string {
			m := voiceTagAny.FindStringSubmatch(tag)
			name := markupcore.DecodeEntities(strings.Trim(m[2], markupcore.PHPTrimSet))
			if to, ok := names[name]; ok {
				return markupcore.VoiceTag(to, m[1])
			}
			return tag
		})
	}
	return out
}

// colorsBySpeaker gives each speaker, in the order of List, the next color in lower case.
func colorsBySpeaker(works []work, opts Options) map[string]string {
	if opts.Style != StyleColors {
		return nil
	}
	colors := opts.Colors
	if len(colors) == 0 {
		colors = BBCColors()
	}
	var t tally
	for i, w := range works {
		t.add(i, w.lines)
	}
	out := make(map[string]string, len(t.counts))
	for i, sc := range t.counts {
		out[sc.Name] = strings.ToLower(colors[i%len(colors)])
	}
	return out
}

// setStyle writes the speakers of a cue with a voice tag in the style of opts. The new lines hold no voice tag.
func (w *work) setStyle(opts Options, colors map[string]string) {
	if w.cue.Speaker == "" && !voiceInLines.MatchString(strings.Join(w.lines, "\n")) {
		return
	}
	parts := speakerParts(w.lines)
	lines := make([]string, len(parts))
	for i, p := range parts {
		lines[i] = p.text
	}
	switch opts.Style {
	case StylePrefix:
		prefixLines(lines, parts, opts)
	case StyleDialogueDashes:
		dashLines(lines, parts, opts.DialogueDashStyle)
	case StyleColors:
		for i, p := range parts {
			if p.speaker != "" {
				lines[i] = `<font color="` + colors[p.speaker] + `">` + p.text + "</font>"
			}
		}
	case StyleNone:
	}
	w.lines = lines
	w.changed = true
}

// prefixLines writes the name and the separator before each part that starts a speaker.
func prefixLines(lines []string, parts []part, opts Options) {
	sep := ": "
	if opts.Separator != nil {
		sep = *opts.Separator
	}
	for i, p := range parts {
		if !p.starts || p.speaker == "" {
			continue
		}
		name := p.speaker
		if !opts.KeepNameCase {
			name = upperCase(name)
		}
		lines[i] = markupcore.EscapeText(name) + markupcore.EscapeText(sep) + p.text
	}
}

// dashLines writes a dash before each part that starts a speaker, when 2 or more parts do.
func dashLines(lines []string, parts []part, style model.DialogueDashStyle) {
	starts := 0
	for _, p := range parts {
		if p.starts {
			starts++
		}
	}
	if starts < 2 {
		return
	}
	dash := markupcore.EscapeText(style.Text())
	for i, p := range parts {
		if p.starts && !startsWithDash(p.text) {
			lines[i] = dash + p.text
		}
	}
}

func startsWithDash(text string) bool {
	visible := markupcore.VisibleText(text)
	p := hilabels.SkipHSpace(visible, 0)
	return p < len(visible) && visible[p] == '-'
}

func upperCase(s string) string {
	if !utf8.ValidString(s) {
		b := []byte(s)
		for i, c := range b {
			if 'a' <= c && c <= 'z' {
				b[i] = c - 'a' + 'A'
			}
		}
		return string(b)
	}
	return cases.Upper(language.Und).String(s)
}
