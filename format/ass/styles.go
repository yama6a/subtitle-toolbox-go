package ass

import (
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/ssatag"
)

// flagFields maps the core tag names to the style fields that turn them on, in the order b, i, u, s.
//
//nolint:gochecknoglobals // lookup table of the style flags
var flagFields = [...]struct{ tag, field string }{
	{"b", "Bold"}, {"i", "Italic"}, {"u", "Underline"}, {"s", "StrikeOut"},
}

// findStyle returns the style of name. A leading "*" is ignored and the last style of a name wins, as in libass.
// "Default" matches in any case.
func findStyle(styles []record, name string) (record, bool) {
	name = strings.TrimLeft(phpTrim(name), "*")
	for i := len(styles) - 1; i >= 0; i-- {
		styleName, _ := styles[i].field("Name")
		styleName = strings.TrimLeft(styleName, "*")
		if styleName == name || equalFold(name, "Default") && equalFold(styleName, "Default") {
			return styles[i], true
		}
	}
	return record{}, false
}

// eventStyle returns the style of an event. An unknown name falls back to the Default style, and without one
// the event has no style.
func eventStyle(styles []record, name string) record {
	if s, ok := findStyle(styles, name); ok {
		return s
	}
	s, _ := findStyle(styles, "Default")
	return s
}

// styleTags returns the core tag names that a style turns on.
func styleTags(style record) []string {
	var tags []string
	for _, f := range flagFields {
		v, _ := style.field(f.field)
		if v = phpTrim(v); v == "-1" || v == "1" {
			tags = append(tags, f.tag)
		}
	}
	return tags
}

// styleAlignment returns the numpad alignment of a style, or 0 for bottom center and for a missing or invalid
// value. The styles of a [V4 Styles] section hold the legacy SSA codes.
func styleAlignment(style record, legacy bool) int {
	v, _ := style.field("Alignment")
	v = phpTrim(v)
	if v == "" || len(v) > 2 || strings.Trim(v, "0123456789") != "" {
		return 0
	}
	var n int
	if legacy {
		n, _ = ssatag.Alignment(`\a` + v)
	} else {
		n, _ = strconv.Atoi(v)
	}
	if n < 1 || n > 9 || n == 2 {
		return 0
	}
	return n
}
