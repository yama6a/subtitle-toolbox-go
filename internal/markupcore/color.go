package markupcore

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// webVTTColors holds the 8 color classes of WebVTT in the order of the default style sheet of the spec,
// https://www.w3.org/TR/webvtt1/#default-text-color.
var webVTTColors = [...]struct{ class, hex string }{ //nolint:gochecknoglobals // a lookup table
	{"white", "#ffffff"},
	{"lime", "#00ff00"},
	{"cyan", "#00ffff"},
	{"red", "#ff0000"},
	{"yellow", "#ffff00"},
	{"magenta", "#ff00ff"},
	{"blue", "#0000ff"},
	{"black", "#000000"},
}

// namedColors holds the 18 color names of TTML 1, https://www.w3.org/TR/ttml1/#style-value-namedColor.
// The html flag marks the 16 names of HTML 4.01, section 6.5. TTML adds magenta and cyan.
var namedColors = [...]struct { //nolint:gochecknoglobals // a lookup table
	name, hex string
	html      bool
}{
	{"black", "#000000", true},
	{"silver", "#c0c0c0", true},
	{"gray", "#808080", true},
	{"white", "#ffffff", true},
	{"maroon", "#800000", true},
	{"red", "#ff0000", true},
	{"purple", "#800080", true},
	{"fuchsia", "#ff00ff", true},
	{"magenta", "#ff00ff", false},
	{"green", "#008000", true},
	{"lime", "#00ff00", true},
	{"olive", "#808000", true},
	{"yellow", "#ffff00", true},
	{"navy", "#000080", true},
	{"blue", "#0000ff", true},
	{"teal", "#008080", true},
	{"aqua", "#00ffff", true},
	{"cyan", "#00ffff", false},
}

const ws = `[ \t\n\r\f\v]*`

var (
	fontColorRegexp = regexp.MustCompile(`(?i)\bcolor` + ws + `=` + ws + `(?:"([^"]*)"|'([^']*)'|([^ \t\n\r\f\v"'>]+))`)
	rgbRegexp       = regexp.MustCompile(`^rgba?\(` + ws + `(\d{1,3})` + ws + `,` + ws + `(\d{1,3})` + ws + `,` + ws + `(\d{1,3})` + ws +
		`(?:,` + ws + `(\d+(?:\.\d*)?|\.\d+)(%?)` + ws + `)?\)$`)
	// PHP ends this pattern with $, which also matches before a final LF.
	ttmlColorRegexp = regexp.MustCompile(`^(?:#[0-9a-fA-F]{6}(?:[0-9a-fA-F]{2})?|rgb\(` + ws + `\d{1,3}(?:` + ws + `,` + ws + `\d{1,3}){2}` + ws +
		`\)|rgba\(` + ws + `\d{1,3}(?:` + ws + `,` + ws + `\d{1,3}){3}` + ws + `\)|transparent)\n?$`)
	cTagRegexp    = regexp.MustCompile(`^<c((?:\.[^ \t\n\r\f\v.<>]*)*)(?:[ \t\n\r\f\v][^<>]*)?>$`)
	cEndRegexp    = regexp.MustCompile(`^</c` + ws + `>$`)
	fontTagRegexp = regexp.MustCompile(`(?i)^<font\b`)
	fontEndRegexp = regexp.MustCompile(`(?i)^</font` + ws + `>$`)
)

// NamedColor returns the "#rrggbb" value of a color name in any case. With htmlOnly, it reads only the 16 names of
// HTML 4.01, as the SAMI reader does. Otherwise it reads the 18 names of TTML.
func NamedColor(name string, htmlOnly bool) (string, bool) {
	name = ASCIILower(name)
	for _, c := range namedColors {
		if c.name == name && (c.html || !htmlOnly) {
			return c.hex, true
		}
	}
	return "", false
}

// FontColor returns the color attribute of a <font> tag as written, such as "#FF0000" for <FONT COLOR='#FF0000'>.
// The attrs hold the attributes or the whole tag. The value can have double quotes, single quotes or no quotes.
func FontColor(attrs string) (string, bool) {
	m := fontColorRegexp.FindStringSubmatch(attrs)
	if m == nil {
		return "", false
	}
	return m[1] + m[2] + m[3], true
}

// ColorHexWithAlpha returns a color as "#rrggbb", or as "#rrggbbaa" when it has an alpha channel, in lower case.
// It reads "#rgb", a value that starts with "#rrggbb" or "#rrggbbaa", "rgb(r, g, b)", "rgba(r, g, b, a)" and
// the 18 TTML color names, in any case and with entities decoded. The alpha of rgba() is a fraction such as 0.5,
// a percentage, or an integer from 0 to 255 as in TTML, so rgba(255, 0, 0, 1) is almost transparent.
func ColorHexWithAlpha(color string) (string, bool) {
	c := ASCIILower(strings.Trim(DecodeEntities(color), PHPTrimSet))
	if hex, ok := hexColor(c); ok {
		return hex, true
	}
	if m := rgbRegexp.FindStringSubmatch(c); m != nil {
		return rgbHex(m)
	}
	return NamedColor(c, false)
}

// hexColor reads a lower-case value that starts with "#rrggbb", "#rrggbbaa" not followed by a hex digit, or is "#rgb".
func hexColor(c string) (string, bool) {
	if !strings.HasPrefix(c, "#") {
		return "", false
	}
	digits := len(c) - 1 - len(strings.TrimLeft(c[1:], "0123456789abcdef"))
	switch {
	case digits == 8:
		return c[:9], true
	case digits >= 6:
		return c[:7], true
	case digits == 3 && len(c) == 4:
		return string([]byte{'#', c[1], c[1], c[2], c[2], c[3], c[3]}), true
	}
	return "", false
}

func rgbHex(m []string) (string, bool) {
	r, _ := strconv.Atoi(m[1])
	g, _ := strconv.Atoi(m[2])
	b, _ := strconv.Atoi(m[3])
	var alpha float64
	if m[4] != "" {
		alpha, _ = strconv.ParseFloat(m[4], 64)
		switch {
		case m[5] == "%":
			alpha = alpha / 100 * 255
		case strings.Contains(m[4], "."):
			alpha *= 255
		}
	}
	alpha = math.Round(alpha)
	if max(r, g, b) > 255 || alpha > 255 {
		return "", false
	}
	hex := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	if m[4] != "" {
		hex += fmt.Sprintf("%02x", int(alpha))
	}
	return hex, true
}

// ColorHex returns a color as "#rrggbb" in lower case. It reads the values of ColorHexWithAlpha and drops the alpha.
// This is the one color rule of all writers.
func ColorHex(color string) (string, bool) {
	hex, ok := ColorHexWithAlpha(color)
	if !ok {
		return "", false
	}
	return hex[:7], true
}

// TTMLColor returns a color for TTML output. A valid TTML color stays as written, a color name goes to lower case,
// and ColorHexWithAlpha converts other colors.
func TTMLColor(color string) (string, bool) {
	if ttmlColorRegexp.MatchString(color) {
		return color, true
	}
	name := ASCIILower(color)
	if _, ok := NamedColor(name, false); ok || name == "transparent" {
		return name, true
	}
	return ColorHexWithAlpha(color)
}

// WebVTTClassColors returns the text and background color of WebVTT classes such as ".bg_black.yellow", as
// "#rrggbb" or empty for none. When several color classes apply, the last one of the default style sheet wins.
func WebVTTClassColors(classes string) (color, background string) {
	names := strings.Split(classes, ".")
	for _, c := range webVTTColors {
		if slices.Contains(names, c.class) {
			color = c.hex
		}
		if slices.Contains(names, "bg_"+c.class) {
			background = c.hex
		}
	}
	return color, background
}

// WebVTTColorsToFont turns <c> tags with a color class into <font> tags, for formats that write <font color>.
// For example <c.yellow>Hi</c> becomes <font color="#ffff00">Hi</font>. Other <c> tags stay as they are.
func WebVTTColorsToFont(text string) string {
	if !strings.Contains(text, "<c") {
		return text
	}
	parts := SplitTags(text)
	var open []bool
	for i := 1; i < len(parts); i += 2 {
		if m := cTagRegexp.FindStringSubmatch(parts[i]); m != nil {
			color, _ := WebVTTClassColors(m[1])
			open = append(open, color != "")
			if color != "" {
				parts[i] = `<font color="` + color + `">`
			}
		} else if cEndRegexp.MatchString(parts[i]) && pop(&open) {
			parts[i] = "</font>"
		}
	}
	return strings.Join(parts, "")
}

// FontToWebVTTColors turns <font> tags into WebVTT <c> tags, for example <font color="#ffff00"> becomes <c.yellow>.
// It drops a <font> tag whose color has no WebVTT class or that has no color, with its end tag. It returns each
// dropped color that is not blank, as written.
func FontToWebVTTColors(text string) (string, []string) {
	if !strings.Contains(ASCIILower(text), "<font") {
		return text, nil
	}
	parts := SplitTags(text)
	var open []bool
	var dropped []string
	for i := 1; i < len(parts); i += 2 {
		switch {
		case fontTagRegexp.MatchString(parts[i]):
			class, color, drop := fontClass(parts[i])
			if drop {
				dropped = append(dropped, color)
			}
			open = append(open, class != "")
			parts[i] = ""
			if class != "" {
				parts[i] = "<c." + class + ">"
			}
		case fontEndRegexp.MatchString(parts[i]):
			parts[i] = ""
			if pop(&open) {
				parts[i] = "</c>"
			}
		}
	}
	return strings.Join(parts, ""), dropped
}

func fontClass(tag string) (class, color string, drop bool) {
	color, ok := FontColor(tag)
	if !ok {
		return "", "", false
	}
	class = webVTTColorClass(color)
	return class, color, class == "" && strings.Trim(color, PHPTrimSet) != ""
}

func webVTTColorClass(color string) string {
	hex, ok := ColorHex(color)
	if !ok {
		return ""
	}
	for _, c := range webVTTColors {
		if c.hex == hex {
			return c.class
		}
	}
	return ""
}

func pop(stack *[]bool) bool {
	s := *stack
	if len(s) == 0 {
		return false
	}
	*stack = s[:len(s)-1]
	return s[len(s)-1]
}

// BGRToRGB swaps the first and the last byte of a 6-digit hex color. ASS and MicroDVD write colors in BGR order.
func BGRToRGB(bgr string) string {
	return hexByte(bgr, 4) + hexByte(bgr, 2) + hexByte(bgr, 0)
}

// RGBToBGR swaps the first and the last byte of a 6-digit hex color.
func RGBToBGR(rgb string) string {
	return BGRToRGB(rgb)
}

// hexByte returns the 2 characters of s at start, or fewer at the end of s, as PHP substr does.
func hexByte(s string, start int) string {
	if start >= len(s) {
		return ""
	}
	return s[start:min(start+2, len(s))]
}
