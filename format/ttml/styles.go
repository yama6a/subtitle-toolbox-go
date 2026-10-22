package ttml

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
)

// maxStyleDepth limits the style references that the reader follows, because styles can refer to each other in a
// loop.
const maxStyleDepth = 20

var (
	spaceRegexp    = regexp.MustCompile(`[\t\n\v\f\r ]+`)
	hexColorRegexp = regexp.MustCompile(`^#([0-9a-f]{6})([0-9a-f]{2})?$`)
	rgbColorRegexp = regexp.MustCompile(`^rgba?\([\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*,[\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*,` +
		`[\t\n\v\f\r ]*(\d{1,3})[\t\n\v\f\r ]*(?:,[\t\n\v\f\r ]*\d{1,3}[\t\n\v\f\r ]*)?\)$`)
)

// style is the part of the TTML style that core markup holds. An empty color means none.
type style struct {
	bold, italic, underline, strike bool
	color                           string
}

// properties maps the local names of styling attributes, such as "fontWeight", to their values.
type properties map[string]string

// styles resolves the styles that become core markup.
type styles struct {
	ns      string
	styles  map[string]*xmlraw.Node
	regions map[string]*xmlraw.Node
}

func newStyles(ns string, head *xmlraw.Node) styles {
	s := styles{ns: ns}
	s.styles = s.elementsByID(head, "style")
	s.regions = s.elementsByID(head, "region")
	return s
}

// elementsByID returns the elements of local name in the root namespace below head by their xml:id. The first
// element of an ID wins.
func (s styles) elementsByID(head *xmlraw.Node, local string) map[string]*xmlraw.Node {
	out := map[string]*xmlraw.Node{}
	if head == nil {
		return out
	}
	for n := range head.Descendants() {
		id := xmlAttr(n, "id")
		if _, ok := out[id]; !ok && id != "" && isElement(n, s.ns, local) {
			out[id] = n
		}
	}
	return out
}

// ownProperties returns the properties of the styles that n references and of its own styling attributes.
// The own attributes win.
func (s styles) ownProperties(n *xmlraw.Node) properties {
	return s.ownPropertiesAt(n, 0)
}

func (s styles) ownPropertiesAt(n *xmlraw.Node, depth int) properties {
	out := properties{}
	if depth < maxStyleDepth {
		for _, id := range styleIDs(n) {
			if ref, ok := s.styles[id]; ok {
				maps.Copy(out, s.ownPropertiesAt(ref, depth+1))
			}
		}
	}
	maps.Copy(out, stylingAttributes(n))
	return out
}

// regionProperties returns the properties of a region: its own, then those of each nested <style>, then its own
// styling attributes again. See https://www.w3.org/TR/ttml2/#semantics-style-inheritance-content.
func (s styles) regionProperties(id string, ok bool) properties {
	region, found := s.regions[id]
	if !ok || !found {
		return properties{}
	}
	out := s.ownProperties(region)
	for _, c := range region.Children {
		if isElement(c, s.ns, "style") {
			maps.Copy(out, s.ownProperties(c))
		}
	}
	maps.Copy(out, stylingAttributes(region))
	return out
}

// styleIDs returns the IDs of the unprefixed style attribute.
func styleIDs(n *xmlraw.Node) []string {
	v, _ := plainAttr(n, "style")
	return splitSpace(v)
}

// stylingAttributes returns the attributes of n in a styling namespace by local name. An attribute tts:x whose
// prefix is not declared counts too.
func stylingAttributes(n *xmlraw.Node) properties {
	out := properties{}
	for _, a := range attributes(n) {
		if slices.Contains(stylingNamespaces, n.AttrNamespaceURI(a)) {
			out[a.Name.Local] = a.Value
		} else if name := undeclaredName(n, a); strings.HasPrefix(name, "tts:") {
			out[name[len("tts:"):]] = a.Value
		}
	}
	return out
}

// apply returns s with the properties applied. See https://www.w3.org/TR/ttml2/#semantics-style-resolution-processing-sss.
func (s style) apply(props properties) style {
	for name, value := range props {
		value = strings.Trim(value, markupcore.PHPTrimSet)
		switch name {
		case "fontWeight":
			s.bold = value == "bold"
		case "fontStyle":
			s.italic = value == "italic" || value == "oblique"
		case "textDecoration":
			s = s.decorate(spaceRegexp.Split(value, -1))
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
	color = asciiLower(strings.Trim(color, markupcore.PHPTrimSet))
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

func (s style) run(text string) markupcore.Run {
	return markupcore.Run{Text: text, Color: s.color, Bold: s.bold, Italic: s.italic, Underline: s.underline, Strike: s.strike}
}

// splitSpace splits s at runs of white space and drops the empty parts.
func splitSpace(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune("\t\n\v\f\r ", r) })
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

// decorate applies the tokens of tts:textDecoration. none turns underline and line-through off first.
func (s style) decorate(tokens []string) style {
	if slices.Contains(tokens, "none") {
		s.underline, s.strike = false, false
	}
	s.underline = slices.Contains(tokens, "underline") || s.underline && !slices.Contains(tokens, "noUnderline")
	s.strike = slices.Contains(tokens, "lineThrough") || s.strike && !slices.Contains(tokens, "noLineThrough")
	return s
}
