package sami

import (
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	defaultClass = "SUBTTL"
	invalidUTF8  = "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding."
)

var (
	otherClassRuleRegexp = regexp.MustCompile(`\n?[ \t]*\.([A-Za-z_][\w-]*)\s*\{[^}]*\}[ \t]*`)
	notClassByteRegexp   = regexp.MustCompile(`[^A-Za-z0-9]`)
	bareValueRegexp      = regexp.MustCompile(`\A[\w.-]+\z`)

	attributeEscaper = strings.NewReplacer("&", "&amp;", `"`, "&quot;", "'", "&#039;", "<", "&lt;", ">", "&gt;") //nolint:gochecknoglobals // a replacer holds no state
)

// writer holds the class of one write. hasClass false writes <P> without a class.
type writer struct {
	class     string
	hasClass  bool
	stripTags bool
}

// Write writes s as SAMI with one language class. A line break inside a line becomes <br>.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	file, _ := s.FormatData(formatDataKey)
	language, hasLanguage := s.Metadata(model.MetadataLanguage)
	style, hasStyle := file.String("style")
	wr := writer{stripTags: common.StripTags}
	if class, ok := file.String("class"); hasStyle || ok {
		wr.class, wr.hasClass = class, ok
	} else {
		wr.class, wr.hasClass = classFor(language), true
	}
	if hasStyle {
		style = wr.keepOnlyClass(style)
	} else {
		style = wr.defaultStyle(language, hasLanguage)
	}

	body := head(s, file, style)
	if !utf8.Valid(body) {
		return unwritable(-1)
	}

	cues := s.Cues()
	for i, c := range cues {
		mark := len(body)
		end := milliseconds(c.End)
		body = append(body, "<SYNC Start="+strconv.FormatInt(milliseconds(c.Start), 10)+">"...)
		body = append(body, wr.paragraphs(c)...)
		body = append(body, '\n')
		if i+1 == len(cues) || milliseconds(cues[i+1].Start) > end {
			body = append(body, "<SYNC Start="+strconv.FormatInt(end, 10)+">"+wr.openParagraph(nil)+"&nbsp;\n"...)
		}
		if !utf8.Valid(body[mark:]) {
			return unwritable(i)
		}
	}
	body = append(body, "</BODY>\n</SAMI>\n"...)
	return writing.Write(w, model.SAMI, body, common, false)
}

func head(s *model.Subtitle, file *model.Data, style string) []byte {
	body := []byte("<SAMI>\n<HEAD>\n")
	if title, ok := s.Metadata(model.MetadataTitle); ok {
		body = append(body, "<TITLE>"+markupcore.EscapeText(title)+"</TITLE>\n"...)
	}
	if param, ok := file.String("samiParam"); ok {
		body = append(body, "<SAMIParam>"+param+"</SAMIParam>\n"...)
	}
	return append(body, `<STYLE TYPE="text/css">`+style+"</STYLE>\n</HEAD>\n<BODY>\n"...)
}

func unwritable(cue int) error {
	return &model.UnwritableError{Format: model.SAMI, CueIndex: cue, Reason: invalidUTF8, Err: model.ErrInvalidUTF8}
}

func milliseconds(d time.Duration) int64 {
	return max(d.Round(time.Millisecond).Milliseconds(), 0)
}

// classFor returns the letters and digits of the language in upper case plus CC, such as KOKRCC for ko-KR.
func classFor(language string) string {
	letters := notClassByteRegexp.ReplaceAllLiteralString(language, "")
	if letters == "" {
		return defaultClass
	}
	return asciiUpper(letters) + "CC"
}

func (wr writer) defaultStyle(language string, hasLanguage bool) string {
	rules := "Name: Subtitles;"
	if hasLanguage {
		rules = "Name: " + language + "; lang: " + language + ";"
	}
	return "<!--\nP { font-family: Arial; text-align: center; }\n." + wr.class + " { " + rules + " }\n-->"
}

// keepOnlyClass removes the rules of the other classes from a stored style, because a player shows the first class
// of the STYLE block by default.
func (wr writer) keepOnlyClass(style string) string {
	if !wr.hasClass {
		return style
	}
	lower := markupcore.ASCIILower(wr.class)
	return otherClassRuleRegexp.ReplaceAllStringFunc(style, func(rule string) string {
		m := otherClassRuleRegexp.FindStringSubmatch(rule)
		if markupcore.ASCIILower(m[1]) == lower {
			return rule
		}
		return ""
	})
}

func (wr writer) paragraphs(c model.Cue) string {
	data, _ := c.FormatData(formatDataKey)
	if stored, ok := storedParagraphs(data); ok && !wr.stripTags {
		if lines, ok := data.Strings("lines"); ok && slices.Equal(lines, c.Lines) {
			var b strings.Builder
			for _, p := range stored {
				b.WriteString(wr.openParagraph(p.attrs) + writeNbsp(p.html))
			}
			return b.String()
		}
	}

	lines := make([]string, len(c.Lines))
	for i, line := range c.Lines {
		line = markupcore.RubyAsText(line)
		if wr.stripTags {
			line = markupcore.StripAllTags(line)
		} else {
			line = markupcore.KeepTags(markupcore.WebVTTColorsToFont(line), "b", "i", "u", "s", "font")
		}
		lines[i] = writeNbsp(breakLines(line))
	}
	if len(lines) == 0 {
		return wr.openParagraph(nil) + "&nbsp;"
	}
	// An HTML parser keeps a tag open into the next <P>, so a SubRip cue such as "<i>never closed" would style the
	// next cue.
	html := strings.Join(lines, "<br>")
	return wr.openParagraph(nil) + html + markupcore.CloseCoreTags(markupcore.OpenCoreTags(html, nil))
}

// storedParagraphs returns the paragraphs of the cue data. False means that the data holds none.
func storedParagraphs(data *model.Data) ([]paragraph, bool) {
	v, ok := data.Get("paragraphs")
	if !ok {
		return nil, false
	}
	list, ok := values(v)
	if !ok {
		return nil, false
	}
	out := make([]paragraph, 0, len(list))
	for _, item := range list {
		d, ok := item.(*model.Data)
		if !ok {
			return nil, false
		}
		var p paragraph
		p.html, _ = d.String("html")
		if attributes, ok := d.Object("attributes"); ok {
			for _, name := range attributes.Keys() {
				value, _ := attributes.String(name)
				p.attrs = append(p.attrs, attr{name: name, value: value, hasValue: true})
			}
		}
		out = append(out, p)
	}
	return out, true
}

// values returns the items of a list, or of an object in key order.
func values(v any) ([]any, bool) {
	switch v := v.(type) {
	case []any:
		return v, true
	case *model.Data:
		out := make([]any, 0, v.Len())
		for _, k := range v.Keys() {
			item, _ := v.Get(k)
			out = append(out, item)
		}
		return out, true
	}
	return nil, false
}

func (wr writer) openParagraph(attrs []attr) string {
	tag := "<P"
	if wr.hasClass {
		tag += " Class=" + wr.class
	}
	for _, a := range attrs {
		value := a.value
		if !bareValueRegexp.MatchString(value) {
			value = `"` + attributeEscaper.Replace(value) + `"`
		}
		tag += " " + asciiUpper(a.name) + "=" + value
	}
	return tag + ">"
}

// breakLines writes each line break of line as <br>. That is an LF, an entity that core markup decodes to LF such
// as &NewLine;, and a reference that the reader decodes to CR or LF such as &#13;.
func breakLines(line string) string {
	var b strings.Builder
	for i, part := range markupcore.SplitLineBreaks(line) {
		if i > 0 {
			b.WriteString("<br>")
		}
		for part != "" {
			j := strings.IndexByte(part, '&')
			if j < 0 {
				b.WriteString(part)
				break
			}
			b.WriteString(part[:j])
			text, n := reference(part[j:])
			if text == "\n" || text == "\r" {
				b.WriteString("<br>")
			} else {
				b.WriteString(part[j : j+n])
			}
			part = part[j+n:]
		}
	}
	return b.String()
}

func writeNbsp(html string) string {
	return strings.ReplaceAll(html, nbsp, "&nbsp;")
}
