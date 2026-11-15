package ttml

import (
	"math"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/internal/ttmldoc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// alignment returns the alignment of a region and a text alignment. Start and end depend on the text direction,
// so they give no alignment. See https://www.w3.org/TR/ttml2/#style-attribute-displayAlign.
func (r *reader) alignment(region string, hasRegion bool, textAlign string, hasTextAlign bool) (model.Alignment, bool) {
	if _, ok := r.styles.Regions[region]; hasRegion && !ok || !hasRegion && len(r.styles.Regions) > 0 {
		return model.BottomCenter, false
	}
	props := r.styles.RegionProperties(region, hasRegion)
	if !hasTextAlign {
		textAlign = propOr(props, "textAlign", "start")
	}
	column := map[string]int{"left": 1, "center": 2, "right": 3}[strings.Trim(textAlign, markupcore.TrimSet)]
	row, ok := r.row(props)
	if column == 0 || !ok {
		return model.BottomCenter, false
	}
	return model.AlignmentAt(row, column)
}

// offset returns the anchor point of the region in percent of the root container, or nil. The region needs an
// explicit tts:origin. The vertical anchor is the one that row uses. The horizontal anchor is the origin, the
// middle or the far edge of the region by the column of alignment a.
func (r *reader) offset(region string, hasRegion bool, a model.Alignment) *model.Offset {
	props := r.styles.RegionProperties(region, hasRegion)
	if origin, ok := props["origin"]; !ok || strings.Trim(origin, markupcore.TrimSet) == "auto" {
		return nil
	}
	var anchor [2]float64
	for axis := range anchor {
		origin, originOK := r.axisLength(props["origin"], axis, 0)
		extent, extentOK := r.axisLength(propOr(props, "extent", "auto"), axis, 1)
		if !originOK || !extentOK {
			return nil
		}
		share := [...]float64{0, 0.5, 1}[a.Column()-1]
		if axis == 1 {
			share = map[string]float64{"before": 0, "center": 0.5, "after": 1}[strings.Trim(propOr(props, "displayAlign", "before"), markupcore.TrimSet)]
		}
		if anchor[axis] = origin + extent*share; math.IsNaN(anchor[axis]) {
			return nil
		}
	}
	o := model.Offset{X: percent(anchor[0]), Y: percent(anchor[1])}
	if !position.OffsetFits(a, &o) {
		return nil
	}
	return position.ReadOffset(a, o)
}

// percent turns a fraction of the root container into percent, clamped to 0 to 100 and rounded to 3 decimals.
func percent(fraction float64) float64 {
	return min(max(fraction*100, 0), 100)
}

// row returns the grid row of the anchor of a region: below 1/3 of the height the top row, above 2/3 the bottom row.
func (r *reader) row(props ttmldoc.Properties) (int, bool) {
	origin, originOK := r.axisLength(propOr(props, "origin", "auto"), 1, 0)
	extent, extentOK := r.axisLength(propOr(props, "extent", "auto"), 1, 1)
	if !originOK || !extentOK {
		return 0, false
	}
	var anchor float64
	switch strings.Trim(propOr(props, "displayAlign", "before"), markupcore.TrimSet) {
	case "before":
		anchor = origin
	case "center":
		anchor = origin + extent/2
	case "after":
		anchor = origin + extent
	default:
		return 0, false
	}
	switch {
	case anchor < 1.0/3:
		return 2, true
	case anchor > 2.0/3:
		return 0, true
	default:
		return 1, true
	}
}

// axisLength returns the length of an origin or extent value on an axis as a fraction of the root container. The
// axis is 0 for the first length and the width, and 1 for the second length and the height.
func (r *reader) axisLength(value string, axis int, auto float64) (float64, bool) {
	parts := ttmldoc.SplitSpace(value)
	if len(parts) == 1 && parts[0] == "auto" {
		return auto, true
	}
	if len(parts) != 2 {
		return 0, false
	}
	m := lengthRegexp.FindStringSubmatch(parts[axis])
	if m == nil {
		return 0, false
	}
	length := ttmldoc.LeadingFloat(m[1])
	switch m[2] {
	case "%":
		return length / 100, true
	case "c":
		return r.cellLength(length, axis)
	default:
		return r.pixelLength(length, axis)
	}
}

// cellLength returns a length in cells as a fraction of the width or height, by the columns or rows of ttp:cellResolution.
func (r *reader) cellLength(length float64, axis int) (float64, bool) {
	v, ok := r.parameter("cellResolution")
	if !ok {
		v = "32 15"
	}
	resolution := spaceRegexp.Split(strings.Trim(v, markupcore.TrimSet), -1)
	if len(resolution) != 2 || leadingInt(resolution[axis]) <= 0 {
		return 0, false
	}
	return length / float64(leadingInt(resolution[axis])), true
}

// pixelLength returns a length in pixels as a fraction of the width or height of tts:extent on <tt>, else on <body>.
func (r *reader) pixelLength(length float64, axis int) (float64, bool) {
	extent, ok := ttmldoc.StylingAttributes(r.root)["extent"]
	if !ok && r.body != nil {
		extent, ok = ttmldoc.StylingAttributes(r.body)["extent"]
	}
	m := rootExtentRegexp.FindStringSubmatch(extent)
	if !ok || m == nil || ttmldoc.LeadingFloat(m[1+axis]) <= 0 {
		return 0, false
	}
	return length / ttmldoc.LeadingFloat(m[1+axis]), true
}

func propOr(props ttmldoc.Properties, name, fallback string) string {
	if v, ok := props[name]; ok {
		return v
	}
	return fallback
}
