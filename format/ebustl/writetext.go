package ebustl

import (
	"bytes"
	"regexp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

var fontOpenRegexp = regexp.MustCompile(`^<font\b[^>]*>$`)

// teletextColorCode returns the alpha color code 0 to 7 of a "#rrggbb" color, or -1.
func teletextColorCode(hex string) int {
	return slices.Index(teletextColors[:], hex)
}

// encodeText converts the lines of a cue to text field codes. Each line is one row.
func (wr *writer) encodeText(c model.Cue, index int) []byte {
	lines := make([]string, len(c.Lines))
	for i, l := range c.Lines {
		lines[i] = markupcore.RubyAsText(l)
	}
	carried := markupcore.CarryOpenTags(lines)
	rows := make([][]byte, len(carried))
	for i, line := range carried {
		rows[i] = wr.encodeRow(line, index)
	}
	return bytes.Join(rows, []byte{newRow})
}

// rowStyle is the state of the tags while a row is written.
type rowStyle struct {
	italic, under int
	// colors is the stack of open <font> tags. -1 stands for a font without a teletext color.
	colors []int
}

func (r *rowStyle) topColor() int {
	if len(r.colors) == 0 {
		return -1
	}
	return r.colors[len(r.colors)-1]
}

// code returns the bytes of a lower-case tag, EBU Tech 3264 appendix 2. Italics and underline take 80h to 83h, the
// 8 teletext colors take 00h to 07h.
func (r *rowStyle) code(tag string) []byte {
	switch {
	case fontOpenRegexp.MatchString(tag):
		return r.openFont(tag)
	case tag == "<i>":
		return toggle(&r.italic, 1, italicsOn)
	case tag == "</i>":
		return toggle(&r.italic, -1, italicsOff)
	case tag == "<u>":
		return toggle(&r.under, 1, underlineOn)
	case tag == "</u>":
		return toggle(&r.under, -1, underlineOff)
	case tag == "</font>":
		return r.closeFont()
	}
	return nil
}

// toggle changes a nesting counter by delta and returns code when the counter goes from 0 to 1 or from 1 to 0.
func toggle(counter *int, delta int, code byte) []byte {
	if delta < 0 && *counter == 0 {
		return nil
	}
	*counter += delta
	if *counter == 0 || *counter == 1 && delta > 0 {
		return []byte{code}
	}
	return nil
}

func (r *rowStyle) openFont(tag string) []byte {
	color := -1
	if attr, ok := markupcore.FontColor(tag); ok {
		if hex, ok := markupcore.ColorHex(attr); ok {
			color = teletextColorCode(hex)
		}
	}
	if color < 0 {
		r.colors = append(r.colors, r.topColor())
		return nil
	}
	r.colors = append(r.colors, color)
	return []byte{byte(color)}
}

// closeFont pops the color and returns the color now on top, white when none, when it differs from the popped one.
func (r *rowStyle) closeFont() []byte {
	if len(r.colors) == 0 {
		return nil
	}
	closed := r.topColor()
	r.colors = r.colors[:len(r.colors)-1]
	current := r.topColor()
	if current < 0 {
		current = white
	}
	if closed < 0 || closed == current {
		return nil
	}
	return []byte{byte(current)}
}

// encodeRow writes one line.
func (wr *writer) encodeRow(line string, index int) []byte {
	var out []byte
	var style rowStyle
	for i, token := range markupcore.SplitTags(markupcore.WebVTTColorsToFont(line)) {
		switch {
		case i%2 == 0:
			out = append(out, wr.encode(markupcore.DecodeEntities(token), index)...)
		case !wr.common.StripTags:
			out = append(out, style.code(strings.ToLower(token))...)
		}
	}
	out = mergeColorSpaces(out)
	for len(out) > 0 && out[len(out)-1] <= white {
		out = out[:len(out)-1]
	}
	if style.italic > 0 {
		out = append(out, italicsOff)
	}
	if style.under > 0 {
		out = append(out, underlineOff)
	}
	return out
}

// mergeColorSpaces removes one space next to a color code, because the code shows as a space.
func mergeColorSpaces(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == ' ' && i+1 < len(b) && b[i+1] <= white:
			out = append(out, b[i+1])
			i++
		case b[i] <= white && i+1 < len(b) && b[i+1] == ' ':
			out = append(out, b[i])
			i++
		default:
			out = append(out, b[i])
		}
	}
	return out
}
