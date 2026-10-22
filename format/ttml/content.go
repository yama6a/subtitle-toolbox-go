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

// content returns the core markup of text as TTML spans. A speaker of the whole text comes back apart, for the
// ttm:agent of the <p>.
func (w *writer) content(text string, inherited style) (agent string, hasAgent bool, out string) {
	tokens := markupcore.SplitTags(text)
	if len(tokens) > 1 && strings.Trim(tokens[0], markupcore.PHPTrimSet) == "" && len(voiceTagsRegexp.FindAllStringIndex(text, 2)) == 1 {
		if m := paragraphVoiceRegexp.FindStringSubmatch(tokens[1]); m != nil {
			agent, hasAgent, tokens = markupcore.DecodeEntities(strings.Trim(m[2], markupcore.PHPTrimSet)), true, tokens[2:]
		}
	}

	var b strings.Builder
	var stack []entry
	var r resets
	for i, token := range tokens {
		if i%2 == 0 {
			b.WriteString(w.text(token))
			if strings.Trim(token, markupcore.PHPTrimSet) != "" {
				r.add(inherited, stack)
			}
			continue
		}
		if m := endTagRegexp.FindStringSubmatch(token); m != nil {
			b.WriteString(closeSpan(&stack, markupcore.ASCIILower(m[1])))
			continue
		}
		m := startTagRegexp.FindStringSubmatch(token)
		if m == nil {
			continue
		}
		tag := markupcore.ASCIILower(m[1])
		if tag == "v" {
			b.WriteString(closeSpan(&stack, "v"))
		}
		if span, ok := w.openSpan(tag, m[2]); ok {
			stack = append(stack, entry{tag, span})
			b.WriteString(span)
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].span != "" {
			b.WriteString("</span>")
		}
	}
	out = b.String()
	if r != (resets{}) {
		out = "<span" + w.resetAttributes(r) + ">" + out + "</span>"
	}
	return agent, hasAgent, out
}

// openSpan returns the <span> start tag of a core markup tag, "" for a tag without a TTML style, and false for a
// tag outside the core markup.
func (w *writer) openSpan(tag, rest string) (string, bool) {
	switch tag {
	case "b":
		return w.span("fontWeight", "bold"), true
	case "i":
		return w.span("fontStyle", "italic"), true
	case "u":
		return w.span("textDecoration", "underline"), true
	case "s":
		return w.span("textDecoration", "lineThrough"), true
	case "font":
		color, _ := markupcore.FontColor(rest)
		color = markupcore.DecodeEntities(strings.Trim(color, markupcore.PHPTrimSet))
		// A value that TTMLColor cannot read stays as written, so no color drops.
		if c, ok := markupcore.TTMLColor(color); ok {
			color = c
		}
		if color == "" {
			return "", true
		}
		return w.span("color", color), true
	case "c":
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
			return "", true
		}
		return "<span" + attrs + ">", true
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
		if e.span == "" {
			continue
		}
		if e.tag == "c" && strings.Contains(e.span, ":color=") {
			open["font"] = true
		} else {
			open[e.tag] = true
		}
	}
	r.bold = r.bold || inherited.bold && !open["b"]
	r.italic = r.italic || inherited.italic && !open["i"]
	r.underline = r.underline || inherited.underline && !open["u"]
	r.strike = r.strike || inherited.strike && !open["s"]
	r.color = r.color || inherited.color != "" && !open["font"]
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
