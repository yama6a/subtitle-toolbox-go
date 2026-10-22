package ttml

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const notWellFormed = "The file is not well-formed XML."

var (
	xmlStartRegexp  = regexp.MustCompile(`<(?:\?xml[\t\n\v\f\r ]|!--|!DOCTYPE[\t\n\v\f\r ]|` + rootPattern + `[\t\n\v\f\r />])`)
	lineBreakRegexp = regexp.MustCompile(`\r\n?|\n`)
	eolRegexp       = regexp.MustCompile(`\r+\n|\r`)
	entityRegexp    = regexp.MustCompile(`&([A-Za-z][A-Za-z0-9]*);`)
	referenceRegexp = regexp.MustCompile(`^(?:amp|lt|gt|quot|apos|#[0-9]+|#x[0-9A-Fa-f]+);`)
	openBreakRegexp = regexp.MustCompile(`(?i)<(?:[A-Za-z_][\w.-]*:)?br(?:[\t\n\v\f\r ][^<>]*)?>`)
)

//nolint:gochecknoglobals // the entities that XML defines
var xmlEntities = []string{"amp", "lt", "gt", "quot", "apos"}

// load parses content. Lenient mode repairs HTML entities and markup that is not well-formed.
func (r *reader) load(content string) (*xmlraw.Node, error) {
	repaired, skipped := skipLeadingText(content)
	if skipped != "" {
		if err := r.skipTextBeforeXML(content, skipped); err != nil {
			return nil, err
		}
	}

	doc, err := parse(repaired, false)
	if err == nil {
		return doc, nil
	}
	if !r.st.Lenient() {
		return nil, r.parseError(model.NoLine, notWellFormed)
	}

	replaced, first := replaceHTMLEntities(repaired)
	if first != "" {
		before, _, _ := strings.Cut(repaired, first)
		line := 1 + strings.Count(before, "\n")
		r.st.Warn("The file has HTML entities that XML does not define, such as \""+first+"\". The parser read them as characters.",
			line, model.NoBlock, nil, model.Repaired)
		if doc, err = parse(replaced, false); err == nil {
			return doc, nil
		}
	}
	return r.recoverDocument(replaced, err)
}

// recoverDocument escapes each & that starts no XML entity or character reference, closes each <br>, and lets the
// recover mode of the parser repair the rest. Without the first 2 steps the recover mode loses the text after a bare
// & or puts the text after an open <br> inside it.
func (r *reader) recoverDocument(content string, firstErr error) (*xmlraw.Node, error) {
	doc, err := parse(closeBreaks(escapeAmpersands(content)), true)
	if err != nil {
		return nil, r.parseError(model.NoLine, notWellFormed)
	}
	msg, line := "", model.NoLine
	var serr *xmlraw.SyntaxError
	if errors.As(firstErr, &serr) {
		msg = " " + upperFirst(strings.TrimRight(serr.Msg, ". \n")) + "."
		line = serr.Line
	}
	r.st.Warn(notWellFormed+msg+" The parser repaired it.", line, model.NoBlock, nil, model.Repaired)
	return doc, nil
}

func parse(content string, recovering bool) (*xmlraw.Node, error) {
	return xmlraw.Parse(strings.NewReader(content), xmlraw.ParseOptions{Recover: recovering})
}

// skipLeadingText removes the text before the XML declaration, a comment, a DOCTYPE or the root element. Line
// breaks replace that text after the XML declaration, so the parser still reports the line numbers of the file.
// It returns the new content and the removed text without white space at its ends.
func skipLeadingText(content string) (string, string) {
	trimmed := strings.TrimLeft(content, markupcore.PHPTrimSet)
	start := len(content) - len(trimmed)
	if !strings.HasPrefix(trimmed, "<") {
		loc := xmlStartRegexp.FindStringIndex(content)
		if loc == nil {
			return content, ""
		}
		start = loc[0]
	}
	if start == 0 {
		return content, ""
	}

	prefix := content[:start]
	breaks := strings.Repeat("\n", len(lineBreakRegexp.FindAllStringIndex(prefix, -1)))
	content = content[start:]
	if strings.HasPrefix(content, "<?xml") {
		if end := strings.Index(content, "?>"); end >= 0 {
			return content[:end+2] + breaks + content[end+2:], strings.Trim(prefix, markupcore.PHPTrimSet)
		}
	}
	return breaks + content, strings.Trim(prefix, markupcore.PHPTrimSet)
}

func (r *reader) skipTextBeforeXML(content, skipped string) error {
	before, _, _ := strings.Cut(content, skipped)
	line := 1 + strings.Count(before, "\n")
	lines := strings.Split(eolRegexp.ReplaceAllLiteralString(skipped, "\n"), "\n")
	msg := "The file has text before the XML: \"" + lines[0] + "\"."
	if !r.st.Lenient() {
		return r.parseError(line, msg)
	}
	r.st.Warn(msg+" The parser skipped it.", line, model.NoBlock, lines, model.Repaired)
	return nil
}

// replaceHTMLEntities replaces the HTML named entities that XML does not define with decimal character references,
// one per code point. The parser never loads a DTD, so no file can define these entities itself. It returns the
// new content and the first entity that it replaced.
func replaceHTMLEntities(content string) (string, string) {
	first := ""
	replaced := entityRegexp.ReplaceAllStringFunc(content, func(entity string) string {
		name := entity[1 : len(entity)-1]
		text, ok := markupcore.NamedEntity(name)
		if !ok || slices.Contains(xmlEntities, name) {
			return entity
		}
		if first == "" {
			first = entity
		}
		var b strings.Builder
		for _, r := range text {
			b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
		}
		return b.String()
	})
	return replaced, first
}

// escapeAmpersands replaces each & that starts no &amp;, &lt;, &gt;, &quot;, &apos;, &#digits; or &#xhex; with &amp;.
func escapeAmpersands(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i+1])
		s = s[i+1:]
		if !referenceRegexp.MatchString(s) {
			b.WriteString("amp;")
		}
	}
}

// closeBreaks closes each <br> or <prefix:br> start tag, in any case, whose > has no / before it.
func closeBreaks(s string) string {
	return openBreakRegexp.ReplaceAllStringFunc(s, func(tag string) string {
		if tag[len(tag)-2] == '/' {
			return tag
		}
		return tag[:len(tag)-1] + "/>"
	})
}

func upperFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
