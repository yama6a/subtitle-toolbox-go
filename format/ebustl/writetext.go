package ebustl

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/codepage"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const (
	italicsOff   = 0x81
	underlineOff = 0x83
)

var fontOpenRegexp = regexp.MustCompile(`^<font\b[^>]*>$`)

// teletextColorCode returns the alpha color code 0 to 7 of a "#rrggbb" color, or -1.
func teletextColorCode(hex string) int {
	return slices.Index([]string{"#000000", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"}, hex)
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

// gsiBlock returns the 1024 bytes of the GSI block. The title, the language and the counts of the written blocks
// replace the stored values when they differ. It also returns the characters of the fields written as "?".
func (wr *writer) gsiBlock(s *model.Subtitle, gsi map[string]string, file *model.Data, sets []set) ([]byte, []Replacement) {
	var cues []set
	for _, st := range sets {
		if !st.comment {
			cues = append(cues, st)
		}
	}
	updateCounts(gsi, file, sets, cues)
	if longest := longestRow(cues, wr.cct); longest > fieldInt(gsi["MNC"]) {
		gsi["MNC"] = strconv.Itoa(min(longest, maxColumns))
	}
	title, _ := s.Metadata(model.MetadataTitle)
	if title != strings.TrimRight(gsi["OPT"], " \x00") {
		gsi["OPT"] = title
	}
	lang, hasLang := s.Metadata(model.MetadataLanguage)
	tag, hasTag := languages[strings.ToUpper(gsi["LC"])]
	if _, ok := gsi["LC"]; !ok || hasLang != hasTag || lang != tag {
		gsi["LC"] = languageCode(lang, hasLang)
	}
	if _, ok := gsi["CD"]; !ok {
		gsi["CD"] = time.Now().UTC().Format("060102")
	}
	if _, ok := gsi["RD"]; !ok {
		gsi["RD"] = gsi["CD"]
	}
	return encodeGSI(gsi)
}

// updateCounts sets TNB, TNS, TNG and TCF from the written sets. A stored value stays when it equals the count that the
// reader stored.
func updateCounts(gsi map[string]string, file *model.Data, sets, cues []set) {
	groups := map[byte]bool{}
	for _, st := range cues {
		groups[st.blocks[0][ttiSGN]] = true
	}
	blocks := 0
	for _, st := range sets {
		blocks += len(st.blocks)
	}
	first := "00000000"
	if len(cues) > 0 {
		first = timeCodeDigits(cues[0].blocks[0][ttiTCI:])
	}
	stored, _ := file.Object("counts")
	for _, f := range []struct {
		name string
		want any
		text string
	}{
		{"TNB", int64(blocks), fmt.Sprintf("%05d", blocks)},
		{"TNS", int64(len(cues)), fmt.Sprintf("%05d", len(cues))},
		{"TNG", int64(len(groups)), fmt.Sprintf("%03d", len(groups))},
		{"TCF", first, first},
	} {
		got, _ := stored.Get(f.name)
		if _, ok := gsi[f.name]; !ok || got != f.want {
			gsi[f.name] = f.text
		}
	}
}

// encodeGSI encodes each field with the code page of CPN, pads it with spaces and cuts it to its length.
func encodeGSI(gsi map[string]string) ([]byte, []Replacement) {
	table := gsiTable(gsi["CPN"])
	var replaced []Replacement
	block := make([]byte, 0, gsiSize)
	for _, f := range gsiFields {
		field := make([]byte, 0, f.length)
		for _, r := range gsi[f.name] {
			if len(field) == f.length {
				break
			}
			c, ok := table.EncodeRune(r)
			if !ok {
				c = codepage.Replacement
				replaced = append(replaced, Replacement{CueIndex: -1, Field: f.name, Character: r})
			}
			field = append(field, c)
		}
		block = append(block, field...)
		block = append(block, bytes.Repeat([]byte{' '}, f.length-len(field))...)
	}
	return block, replaced
}

// longestRow returns the number of displayable bytes of the longest row of the cue sets.
func longestRow(cues []set, cct string) int {
	longest := 0
	for _, st := range cues {
		for _, row := range bytes.Split(textBytes(st.blocks), []byte{newRow}) {
			n := 0
			for _, c := range row {
				if c >= 0x20 && c <= 0x7F || c >= 0xA0 && (cct != "00" || c <= 0xC0 || c >= 0xD0) {
					n++
				}
			}
			longest = max(longest, n)
		}
	}
	return longest
}

// languageCode returns the LC code of a language tag: the first code with the same tag in any case, else with the
// same primary subtag, else "00".
func languageCode(lang string, ok bool) string {
	if !ok {
		return "00"
	}
	codes := make([]string, 0, len(languages))
	for code := range languages {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	for _, want := range []string{lang, strings.SplitN(lang, "-", 2)[0]} {
		for _, code := range codes {
			if strings.EqualFold(languages[code], want) {
				return code
			}
		}
	}
	return "00"
}
