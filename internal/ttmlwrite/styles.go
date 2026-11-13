package ttmlwrite

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
)

// MaxStyleDepth limits the style references that Sheet and the reader follow, because styles can refer to each other in a
// loop.
const MaxStyleDepth = 20

var (
	hexColorRegexp = regexp.MustCompile(`^#([0-9a-f]{6})([0-9a-f]{2})?$`)
	rgbColorRegexp = regexp.MustCompile(`^rgba?\([\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*,[\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*,` +
		`[\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*(?:,[\t\n\v\f\r ]*\d{1,3}[\t\n\v\f\r ]*)?\)$`)
)

// Style is the part of the TTML style that core markup holds. An empty color means none.
type Style struct {
	bold, italic, underline, strike bool
	color                           string
}

// Properties maps the local names of styling attributes, such as "fontWeight", to their values.
type Properties map[string]string

// Sheet resolves the styles that become core markup.
type Sheet struct {
	ns      string
	Styles  map[string]*xmlraw.Node // the <style> elements of the head by xml:id
	Regions map[string]*xmlraw.Node // the <region> elements of the head by xml:id
}

// NewSheet returns the styles and regions of head, the elements in the root namespace ns.
func NewSheet(ns string, head *xmlraw.Node) Sheet {
	s := Sheet{ns: ns}
	s.Styles = s.elementsByID(head, "style")
	s.Regions = s.elementsByID(head, "region")
	return s
}

// elementsByID returns the elements of local name in the root namespace below head by their xml:id. The first
// element of an ID wins.
func (s Sheet) elementsByID(head *xmlraw.Node, local string) map[string]*xmlraw.Node {
	out := map[string]*xmlraw.Node{}
	if head == nil {
		return out
	}
	for n := range head.Descendants() {
		id := XMLAttr(n, "id")
		if _, ok := out[id]; !ok && id != "" && IsElement(n, s.ns, local) {
			out[id] = n
		}
	}
	return out
}

// OwnProperties returns the properties of the styles that n references and of its own styling attributes.
// The own attributes win.
func (s Sheet) OwnProperties(n *xmlraw.Node) Properties {
	return s.ownPropertiesAt(n, 0)
}

func (s Sheet) ownPropertiesAt(n *xmlraw.Node, depth int) Properties {
	out := Properties{}
	if depth < MaxStyleDepth {
		for _, id := range StyleIDs(n) {
			if ref, ok := s.Styles[id]; ok {
				maps.Copy(out, s.ownPropertiesAt(ref, depth+1))
			}
		}
	}
	maps.Copy(out, StylingAttributes(n))
	return out
}

// RegionProperties returns the properties of a region: its own, then those of each nested <style>, then its own
// styling attributes again. See https://www.w3.org/TR/ttml2/#semantics-style-inheritance-content.
func (s Sheet) RegionProperties(id string, ok bool) Properties {
	region, found := s.Regions[id]
	if !ok || !found {
		return Properties{}
	}
	out := s.OwnProperties(region)
	for _, c := range region.Children {
		if IsElement(c, s.ns, "style") {
			maps.Copy(out, s.OwnProperties(c))
		}
	}
	maps.Copy(out, StylingAttributes(region))
	return out
}

// StyleIDs returns the IDs of the unprefixed style attribute.
func StyleIDs(n *xmlraw.Node) []string {
	v, _ := PlainAttr(n, "style")
	return SplitSpace(v)
}

// StylingAttributes returns the attributes of n in a styling namespace by local name. An attribute tts:x whose
// prefix is not declared counts too.
func StylingAttributes(n *xmlraw.Node) Properties {
	out := Properties{}
	for _, a := range Attributes(n) {
		if slices.Contains(stylingNamespaces, n.AttrNamespaceURI(a)) {
			out[a.Name.Local] = a.Value
		} else if name := UndeclaredName(n, a); strings.HasPrefix(name, "tts:") {
			out[name[len("tts:"):]] = a.Value
		}
	}
	return out
}

// Apply returns s with the properties applied. See https://www.w3.org/TR/ttml2/#semantics-style-resolution-processing-sss.
func (s Style) Apply(props Properties) Style {
	for name, value := range props {
		value = strings.Trim(value, markupcore.PHPTrimSet)
		switch name {
		case "fontWeight":
			s.bold = value == "bold"
		case "fontStyle":
			s.italic = value == "italic" || value == "oblique"
		case "textDecoration":
			s = s.decorate(value)
		case "color":
			// White is the default text color of every player, so it adds no markup.
			if s.color = normalizeColor(value); s.color == "#ffffff" {
				s.color = ""
			}
		}
	}
	return s
}

// normalizeColor returns a color as "#rrggbb" without its alpha channel, because core markup has none, or "".
// See https://www.w3.org/TR/ttml2/#style-value-color.
func normalizeColor(color string) string {
	color = markupcore.ASCIILower(strings.Trim(color, markupcore.PHPTrimSet))
	if m := hexColorRegexp.FindStringSubmatch(color); m != nil {
		return "#" + m[1]
	}
	if m := rgbColorRegexp.FindStringSubmatch(color); m != nil {
		hex := "#"
		for _, channel := range m[1:4] {
			n, _ := strconv.Atoi(channel)
			hex += fmt.Sprintf("%02x", min(n, 255))
		}
		return hex
	}
	hex, _ := markupcore.NamedColor(color, false)
	return hex
}

func (s Style) Run(text string) markupcore.Run {
	return markupcore.Run{Text: text, Color: s.color, Bold: s.bold, Italic: s.italic, Underline: s.underline, Strike: s.strike}
}

// SplitSpace splits s at runs of white space and drops the empty parts.
func SplitSpace(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune("\t\n\v\f\r ", r) })
}

// decorate applies the tokens of the tts:textDecoration value. none turns underline and line-through off first.
func (s Style) decorate(value string) Style {
	tokens := SplitSpace(value)
	if slices.Contains(tokens, "none") {
		s.underline, s.strike = false, false
	}
	s.underline = slices.Contains(tokens, "underline") || s.underline && !slices.Contains(tokens, "noUnderline")
	s.strike = slices.Contains(tokens, "lineThrough") || s.strike && !slices.Contains(tokens, "noLineThrough")
	return s
}
