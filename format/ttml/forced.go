package ttml

import (
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

// paragraphForced reports whether the cue of a paragraph is forced: by the paragraph, else its nearest container,
// else its region, or by one of its spans.
func (r *reader) paragraphForced(p *xmlraw.Node, sc scope, region string, hasRegion bool) bool {
	forced, ok := r.forcedDisplay(p)
	if !ok && sc.forced != nil {
		forced, ok = *sc.forced, true
	}
	if !ok {
		forced, _ = r.regionForcedDisplay(region, hasRegion)
	}
	return forced || r.hasForcedSpan(p, forced)
}

// forcedDisplay reads itts:forcedDisplay of n, else of the styles that n references. The last reference that sets
// it wins. See https://www.w3.org/TR/ttml-imsc1.1/#forceddisplay.
func (r *reader) forcedDisplay(n *xmlraw.Node) (bool, bool) {
	return r.forcedDisplayAt(n, 0)
}

func (r *reader) forcedDisplayAt(n *xmlraw.Node, depth int) (bool, bool) {
	for _, a := range ttmldoc.Attributes(n) {
		if n.AttrNamespaceURI(a) == ttmldoc.NSIMSCStyle && a.Name.Local == "forcedDisplay" || ttmldoc.UndeclaredName(n, a) == "itts:forcedDisplay" {
			return strings.Trim(a.Value, markupcore.TrimSet) == "true", true
		}
	}
	forced, found := false, false
	if depth < ttmldoc.MaxStyleDepth {
		for _, id := range ttmldoc.StyleIDs(n) {
			if ref, ok := r.styles.Styles[id]; ok {
				if f, ok := r.forcedDisplayAt(ref, depth+1); ok {
					forced, found = f, true
				}
			}
		}
	}
	return forced, found
}

// regionForcedDisplay reads itts:forcedDisplay of a region, else of its first nested <style> that sets it.
func (r *reader) regionForcedDisplay(id string, ok bool) (bool, bool) {
	region, found := r.styles.Regions[id]
	if !ok || !found {
		return false, false
	}
	if forced, ok := r.forcedDisplay(region); ok {
		return forced, true
	}
	for _, c := range region.Children {
		if ttmldoc.IsElement(c, r.ns, "style") {
			if forced, ok := r.forcedDisplay(c); ok {
				return forced, true
			}
		}
	}
	return false, false
}

// hasForcedSpan reports whether a span of n is forced, with inheritance inside the spans.
func (r *reader) hasForcedSpan(n *xmlraw.Node, inherited bool) bool {
	for _, c := range n.Children {
		if !ttmldoc.IsElement(c, r.ns, "span") {
			continue
		}
		forced, ok := r.forcedDisplay(c)
		if !ok {
			forced = inherited
		}
		if forced || r.hasForcedSpan(c, forced) {
			return true
		}
	}
	return false
}
