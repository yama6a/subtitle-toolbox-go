package ttml

import (
	"regexp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

var (
	paragraphVoiceRegexp = regexp.MustCompile(`^` + markupcore.VoiceTagPattern + `$`)
	voiceTagsRegexp      = regexp.MustCompile(`</?v[\t\n\v\f\r .>]`)
	endTagRegexp         = regexp.MustCompile(`^</([a-zA-Z]+)[\t\n\v\f\r ]*>$`)
	startTagRegexp       = regexp.MustCompile(`^<([a-zA-Z]+)([\t\n\v\f\r .][^>]*)?>$`)
	voiceClassRegexp     = regexp.MustCompile(`^\.[^\t\n\v\f\r ]*`)
)

// entry is an open tag of core markup with the <span> start tag it wrote, "" for a tag without a TTML style.
type entry struct {
	tag, span string
}

// resets holds the inherited styles that no open span repeats, which the content must turn off.
type resets struct {
	bold, italic, underline, strike, color bool
}

// converter holds the state of one content conversion.
type converter struct {
	w         *writer
	inherited style
	b         strings.Builder
	stack     []entry
	resets    resets
}

// content returns the core markup of text as TTML spans. A speaker of the whole text comes back apart, for the
// ttm:agent of the <p>.
func (w *writer) content(text string, inherited style) (agent string, hasAgent bool, out string) {
	agent, hasAgent, tokens := paragraphAgent(markupcore.SplitTags(text), text)
	c := &converter{w: w, inherited: inherited}
	for i, token := range tokens {
		if i%2 == 0 {
			c.text(token)
		} else {
			c.tag(token)
		}
	}
	for i := len(c.stack) - 1; i >= 0; i-- {
		if c.stack[i].span != "" {
			c.b.WriteString("</span>")
		}
	}
	out = c.b.String()
	if c.resets != (resets{}) {
		out = "<span" + w.resetAttributes(c.resets) + ">" + out + "</span>"
	}
	return agent, hasAgent, out
}

// paragraphAgent takes the speaker out of the tokens when one <v> tag after blank text starts the text and the text
// holds no other <v> tag.
func paragraphAgent(tokens []string, text string) (string, bool, []string) {
	if len(tokens) < 2 || strings.Trim(tokens[0], markupcore.PHPTrimSet) != "" || len(voiceTagsRegexp.FindAllStringIndex(text, 2)) != 1 {
		return "", false, tokens
	}
	m := paragraphVoiceRegexp.FindStringSubmatch(tokens[1])
	if m == nil {
		return "", false, tokens
	}
	return markupcore.DecodeEntities(strings.Trim(m[2], markupcore.PHPTrimSet)), true, tokens[2:]
}

func (c *converter) text(token string) {
	c.b.WriteString(c.w.text(token))
	if strings.Trim(token, markupcore.PHPTrimSet) != "" {
		c.resets.add(c.inherited, c.stack)
	}
}

func (c *converter) tag(token string) {
	if m := endTagRegexp.FindStringSubmatch(token); m != nil {
		c.b.WriteString(closeSpan(&c.stack, markupcore.ASCIILower(m[1])))
		return
	}
	m := startTagRegexp.FindStringSubmatch(token)
	if m == nil {
		return
	}
	tag := markupcore.ASCIILower(m[1])
	if tag == "v" {
		c.b.WriteString(closeSpan(&c.stack, "v"))
	}
	if span, ok := c.w.openSpan(tag, m[2]); ok {
		c.stack = append(c.stack, entry{tag, span})
		c.b.WriteString(span)
	}
}

//nolint:gochecknoglobals // the styles of the core markup style tags
var tagStyles = map[string][2]string{
	"b": {"fontWeight", "bold"},
	"i": {"fontStyle", "italic"},
	"u": {"textDecoration", "underline"},
	"s": {"textDecoration", "lineThrough"},
}

// openSpan returns the <span> start tag of a core markup tag, "" for a tag without a TTML style, and false for a
// tag outside the core markup.
func (w *writer) openSpan(tag, rest string) (string, bool) {
	if st, ok := tagStyles[tag]; ok {
		return w.span(st[0], st[1]), true
	}
	switch tag {
	case "font":
		return w.fontSpan(rest), true
	case "c":
		return w.classSpan(rest), true
	case "v":
		name := markupcore.DecodeEntities(strings.Trim(voiceClassRegexp.ReplaceAllLiteralString(rest, ""), markupcore.PHPTrimSet))
		if name == "" {
			return "", true
		}
		return "<span" + w.attribute(w.ttm+":agent", w.agentID(name)) + ">", true
	default:
		return "", false
	}
}

func (w *writer) fontSpan(rest string) string {
	color, _ := markupcore.FontColor(rest)
	color = markupcore.DecodeEntities(strings.Trim(color, markupcore.PHPTrimSet))
	// A value that TTMLColor cannot read stays as written, so no color drops.
	if c, ok := markupcore.TTMLColor(color); ok {
		color = c
	}
	if color == "" {
		return ""
	}
	return w.span("color", color)
}

func (w *writer) classSpan(rest string) string {
	classes := rest
	if i := strings.IndexAny(rest, " \t\n\v\f\r"); i >= 0 {
		classes = rest[:i]
	}
	color, background := markupcore.WebVTTClassColors(classes)
	var attrs string
	if color != "" {
		attrs += w.attribute(w.tts+":color", color)
	}
	if background != "" {
		attrs += w.attribute(w.tts+":backgroundColor", background)
	}
	if attrs == "" {
		return ""
	}
	return "<span" + attrs + ">"
}

func (w *writer) span(property, value string) string {
	return "<span" + w.attribute(w.tts+":"+property, value) + ">"
}

// closeSpan closes the innermost open entry of tag. The spans above it close too and open again, so mis-nested
// core markup keeps its styles.
func closeSpan(stack *[]entry, tag string) string {
	pos := -1
	for i, e := range *stack {
		if e.tag == tag {
			pos = i
		}
	}
	if pos < 0 {
		return ""
	}
	var b strings.Builder
	for i := len(*stack) - 1; i >= pos; i-- {
		if (*stack)[i].span != "" {
			b.WriteString("</span>")
		}
	}
	above := slices.Clone((*stack)[pos+1:])
	*stack = append((*stack)[:pos], above...)
	for _, e := range above {
		b.WriteString(e.span)
	}
	return b.String()
}

// add notes the inherited styles that the open spans do not repeat. A <c> span with a color counts as a color.
func (r *resets) add(inherited style, stack []entry) {
	open := map[string]bool{}
	for _, e := range stack {
		switch {
		case e.span == "":
		case e.tag == "c" && strings.Contains(e.span, ":color="):
			open["font"] = true
		default:
			open[e.tag] = true
		}
	}
	for _, f := range []struct {
		inherited bool
		tag       string
		reset     *bool
	}{
		{inherited.bold, "b", &r.bold},
		{inherited.italic, "i", &r.italic},
		{inherited.underline, "u", &r.underline},
		{inherited.strike, "s", &r.strike},
		{inherited.color != "", "font", &r.color},
	} {
		if f.inherited && !open[f.tag] {
			*f.reset = true
		}
	}
}

func (w *writer) resetAttributes(r resets) string {
	var out string
	if r.bold {
		out += w.attribute(w.tts+":fontWeight", "normal")
	}
	if r.italic {
		out += w.attribute(w.tts+":fontStyle", "normal")
	}
	var decorations []string
	if r.underline {
		decorations = append(decorations, "noUnderline")
	}
	if r.strike {
		decorations = append(decorations, "noLineThrough")
	}
	if len(decorations) > 0 {
		out += w.attribute(w.tts+":textDecoration", strings.Join(decorations, " "))
	}
	if r.color {
		out += w.attribute(w.tts+":color", "white")
	}
	return out
}
