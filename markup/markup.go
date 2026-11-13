package markup

import "github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"

// StripAllTags removes every tag, word timestamps too. Entities stay.
func StripAllTags(text string) string {
	return markupcore.StripAllTags(text)
}

// KeepTags removes every tag whose name is not in names. The name is the text after "<" or "</" up to white
// space, "/" or ">", compared without case, so <c.yellow> has the name c.yellow.
func KeepTags(text string, names ...string) string {
	return markupcore.KeepTags(text, names...)
}

// DecodeEntities decodes the entities of text, such as &lt;, &eacute; or &#233;, as PHP html_entity_decode does with
// ENT_QUOTES and ENT_HTML5. A named entity needs its ";". A numeric entity decodes only to a code point that HTML5
// allows in a document, so &#13; and &#128; stay as written.
func DecodeEntities(text string) string {
	return markupcore.DecodeEntities(text)
}

// EscapeText escapes &, < and > as &amp;, &lt; and &gt;. It keeps every other byte, also of invalid UTF-8.
func EscapeText(text string) string {
	return markupcore.EscapeText(text)
}

// PlainText removes the tags and decodes the entities, for formats without markup. It does not trim.
func PlainText(text string) string {
	return markupcore.PlainText(text)
}

// HasVisibleText reports whether a line holds more than white space outside its tags. An entity counts as text.
func HasVisibleText(lines []string) bool {
	return markupcore.HasVisibleText(lines)
}

// VisibleLength returns the number of characters of PlainText after trimming space, tab, LF, CR, NUL and VT.
// A character is a grapheme cluster. The soft hyphen, the zero-width space, the BOM and the bidi controls count 0. Invalid UTF-8 counts its bytes.
func VisibleLength(text string) int {
	return markupcore.VisibleLength(text)
}

// MapTextRuns calls fn for each text run between tags that is not empty, with &lt;, &gt; and &amp; decoded.
// First and last mark the first and the last such run of the line. A changed run is escaped again like its raw form.
// An unchanged run keeps its bytes.
func MapTextRuns(lines []string, fn func(text string, first, last bool) string) []string {
	return markupcore.MapTextRuns(lines, fn)
}

// VoiceTag returns a <v> tag for the speaker name with &, < and > escaped and quotes kept.
// The class holds voice classes such as ".loud".
func VoiceTag(name, class string) string {
	return markupcore.VoiceTag(name, class)
}
