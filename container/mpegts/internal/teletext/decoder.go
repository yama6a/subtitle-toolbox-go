// Package teletext decodes teletext subtitle pages (ETS 300 706) from PES data (EN 300 472).
package teletext

import (
	"math/bits"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

const (
	dataUnitSize = 44
	lastRow      = 23
	colorWhite   = 7
	endBox       = 0x0A
	startBox     = 0x0B
	noGroup      = -1
)

//nolint:gochecknoglobals // lookup table
var colors = [...]string{"#000000", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff"}

// State is the text that a page shows from Start on. No lines clear the page.
type State struct {
	// Start is the PTS after the unwrap, in 90 kHz ticks.
	Start int64
	// Lines holds core markup: escaped text and font color tags.
	Lines []string
}

// Decoder reads one page from the PES packets of a teletext stream.
type Decoder struct {
	page          int
	magazine      int
	number        int
	languageGroup int

	receiving      bool
	start          int64
	rows           [lastRow + 1][]byte
	boxed          bool
	nationalOption int
	pageGroup      int
	magazineGroup  int

	states []State
	err    error
}

// NewDecoder returns a decoder for the page magazine<<8 | pageByte, with magazine 0 for 8.
// number is the page number in messages. languageGroup is the group of the track language.
func NewDecoder(page, number, languageGroup int) *Decoder {
	return &Decoder{
		page: page, magazine: page >> 8, number: number, languageGroup: languageGroup,
		pageGroup: noGroup, magazineGroup: noGroup,
	}
}

// Add reads the data of one PES packet that has the time pts.
func (d *Decoder) Add(data []byte, pts int64) {
	if len(data) == 0 || data[0] < 0x10 || data[0] > 0x1F {
		return
	}
	for off := 1; off+2 <= len(data) && d.err == nil; off += 2 + int(data[off+1]) {
		if unit := data[off:]; isTeletextUnit(unit) {
			var packet [42]byte
			for i, b := range unit[4:46] {
				packet[i] = bits.Reverse8(b)
			}
			d.packet(packet[:], pts)
		}
	}
}

// isTeletextUnit reports whether the data unit at the start of unit holds a teletext packet of 42 bytes.
func isTeletextUnit(unit []byte) bool {
	return (unit[0] == 0x02 || unit[0] == 0x03) && unit[1] == dataUnitSize && len(unit) >= 2+dataUnitSize
}

// Finish ends the page that is still being received and returns the states of the page.
// The error is the one of the first Arabic or Hebrew page.
func (d *Decoder) Finish() ([]State, error) {
	if d.receiving {
		d.endTransmission()
	}
	if d.err != nil {
		return nil, d.err
	}
	return d.states, nil
}

func (d *Decoder) packet(p []byte, pts int64) {
	h0, h1 := hamming84[p[0]], hamming84[p[1]]
	if h0 < 0 || h1 < 0 {
		return
	}
	address := int(h0) | int(h1)<<4
	magazine, row := address&0x07, address>>3
	switch {
	case row == 0:
		d.header(p, magazine, pts)
	case magazine != d.magazine:
	case row <= lastRow && d.receiving:
		d.rows[row] = slices.Clone(p[2:42])
	case row == 28 && d.receiving || row == 29:
		d.characterSetGroup(p, row)
	}
}

// header ends the transmission of the page and starts it again when the header is the one of the page.
func (d *Decoder) header(p []byte, magazine int, pts int64) {
	var v [8]int
	valid := true
	for i := range v {
		v[i] = int(hamming84[p[2+i]])
		valid = valid && v[i] >= 0
	}
	units, tens, subcode2, subcode4, control := v[0], v[1], v[3], v[5], v[7]
	// A header with 2 bit errors still ends the page of its magazine, so the rows after it do not join the page.
	if d.receiving && (magazine == d.magazine || valid && control&0x01 != 0) {
		d.endTransmission()
	}
	if !valid || magazine<<8|tens<<4|units != d.page {
		return
	}
	d.receiving = true
	d.start = pts
	d.boxed = subcode4&0x0C != 0
	d.nationalOption = control >> 1
	d.pageGroup = noGroup
	if subcode2&0x08 != 0 {
		d.rows = [lastRow + 1][]byte{}
	}
}

// characterSetGroup reads the group of the default G0 set from triplet 1 of X/28/0 format 1 or M/29/0.
func (d *Decoder) characterSetGroup(p []byte, row int) {
	designation := hamming84[p[2]]
	triplet, ok := hamming2418(p[3], p[4], p[5])
	if (designation != 0 && designation != 4) || !ok || row == 28 && triplet&0x0F != 0 {
		return
	}
	group := triplet >> 10 & 0x0F
	if row == 28 {
		d.pageGroup = group
	} else {
		d.magazineGroup = group
	}
}

func (d *Decoder) endTransmission() {
	d.receiving = false
	group := d.languageGroup
	if d.magazineGroup != noGroup {
		group = d.magazineGroup
	}
	if d.pageGroup != noGroup {
		group = d.pageGroup
	}
	chars, ok := g0(group, d.nationalOption)
	if !ok {
		d.err = errRightToLeft(d.number)
		return
	}
	var lines []string
	for _, row := range d.rows {
		if row == nil {
			continue
		}
		if line := d.rowText(row, chars); line != "" {
			lines = append(lines, line)
		}
	}
	d.states = append(d.states, State{Start: d.start, Lines: lines})
}

type run struct {
	color int
	text  strings.Builder
}

// rowText returns the text of a row with font tags for colors other than white. A boxed page shows only the text
// in boxes. A spacing attribute takes a cell and shows as a space.
func (d *Decoder) rowText(row []byte, chars []rune) string {
	var runs []*run
	color, inBox := colorWhite, false
	for _, b := range row {
		code := cellCode(b)
		ch := " "
		if code >= 0x20 && (!d.boxed || inBox) {
			ch = markupcore.EscapeText(string(chars[code-0x20]))
		}
		runs = appendCell(runs, color, ch)
		// The colors and the box attributes act from the next cell on.
		if code <= colorWhite {
			color = code
		}
		inBox = code == startBox || inBox && code != endBox
	}
	return renderRuns(runs)
}

// cellCode returns the 7-bit code of a byte with odd parity, else the code of a space.
func cellCode(b byte) int {
	if bits.OnesCount8(b)%2 == 1 {
		return int(b & 0x7F)
	}
	return 0x20
}

// appendCell adds a character to the last run, or starts a run of the color at the first character that is not a space.
func appendCell(runs []*run, color int, ch string) []*run {
	switch {
	case ch != " " && (len(runs) == 0 || runs[len(runs)-1].color != color):
		r := &run{color: color}
		r.text.WriteString(ch)
		return append(runs, r)
	case len(runs) > 0:
		runs[len(runs)-1].text.WriteString(ch)
	}
	return runs
}

func renderRuns(runs []*run) string {
	var out strings.Builder
	for _, r := range runs {
		text := r.text.String()
		words := strings.TrimRight(text, " ")
		if r.color == colorWhite {
			out.WriteString(words)
		} else {
			out.WriteString(`<font color="` + colors[r.color] + `">` + words + "</font>")
		}
		out.WriteString(text[len(words):])
	}
	return strings.TrimRight(out.String(), " ")
}
