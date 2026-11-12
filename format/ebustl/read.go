// Package ebustl reads EBU STL, the binary subtitle exchange format of EBU Tech 3264.
//
// A file holds one GSI block of 1024 bytes, then TTI blocks of 128 bytes. A subtitle takes one or more TTI blocks.
package ebustl

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const formatDataKey = string(model.EBUSTL)

// ReadOptions holds the settings of the EBU STL reader.
type ReadOptions struct {
	// SubtractStartOfProgramme subtracts the GSI start of programme (TCP) from every cue time.
	// A time before it becomes 0.
	SubtractStartOfProgramme bool
}

type reader struct {
	st          *parsing.State
	rate        model.FrameRate
	offset      time.Duration
	cct         string
	maxRow      int
	teletext    bool
	columns     int
	cues        []model.Cue
	comments    []parsing.ParsedComment
	commentData []any
	groups      map[byte]bool
	firstIn     string
}

// Read reads an EBU STL file from its raw bytes.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.EBUSTL, err)
	}
	st := parsing.NewState(model.EBUSTL, common)
	content, err = cutIncompleteBlock(st, content)
	if err != nil {
		return nil, nil, err
	}
	gsi := readGSI(content[:gsiSize])
	rd, err := newReader(st, gsi, opts)
	if err != nil {
		return nil, nil, err
	}
	sets := readSets(content[gsiSize:])
	rd.addSets(sets)
	s := &model.Subtitle{}
	if title := strings.TrimRight(gsi.get("OPT"), " \x00"); title != "" {
		s.SetMetadata(model.MetadataTitle, title)
	}
	if lang, ok := languages[strings.ToUpper(gsi.get("LC"))]; ok {
		s.SetMetadata(model.MetadataLanguage, lang)
	}
	parsing.AddParsed(s, rd.cues, rd.comments)
	_ = s.SetFormatData(formatDataKey, rd.fileData(gsi, sets, opts, (len(content)-gsiSize)/ttiSize)) // the data has the shape of the schema
	return s, st.Warnings(), nil
}

// newReader checks the DFC and CCT fields of the GSI and sets the frame rate, the start of programme and the grid.
func newReader(st *parsing.State, gsi gsiValues, opts ReadOptions) (*reader, error) {
	rd := &reader{st: st, cct: gsi.get("CCT"), groups: map[byte]bool{}}
	switch dfc := gsi.get("DFC"); dfc {
	case "STL25.01":
		rd.rate = model.FrameRate25
	case "STL30.01":
		rd.rate = model.FrameRate30
	default:
		return nil, &model.ParseError{Format: model.EBUSTL, Msg: `The disk format code "` + dfc + `" is not STL25.01 or STL30.01.`}
	}
	if !slices.Contains([]string{"00", "01", "02", "03", "04"}, rd.cct) {
		return nil, &model.ParseError{Format: model.EBUSTL, Msg: `The character code table "` + rd.cct + `" is not 00, 01, 02, 03 or 04.`}
	}
	if opts.SubtractStartOfProgramme {
		rd.offset = startOfProgramme(rd.rate, gsi.get("TCP"))
	}
	rd.teletext = gsi.get("DSC") == "1" || gsi.get("DSC") == "2"
	rd.maxRow = teletextRows
	if rows := fieldInt(gsi.get("MNR")); !rd.teletext && rows > 0 {
		rd.maxRow = rows
	}
	rd.columns = defaultColumns
	if n := fieldInt(gsi.get("MNC")); n > 0 {
		rd.columns = n
	}
	return rd, nil
}

// fileData returns the file format data. blocks is the count of TTI blocks after a cut.
func (rd *reader) fileData(gsi gsiValues, sets [][][]byte, opts ReadOptions, blocks int) *model.Data {
	gsiData := model.NewData()
	for i, f := range gsiFields {
		gsiData.Set(f.name, gsi[i])
	}
	var first any
	if len(sets) > 0 {
		first = int64(binary.LittleEndian.Uint16(sets[0][0][ttiSN:]))
	}
	firstIn := cmp.Or(rd.firstIn, "00000000")
	return model.NewData(
		model.Field{Key: "gsi", Value: gsiData},
		model.Field{Key: "startOfProgrammeSubtracted", Value: opts.SubtractStartOfProgramme},
		model.Field{Key: "firstSubtitleNumber", Value: first},
		model.Field{Key: "comments", Value: nonNil(rd.commentData)},
		model.Field{Key: "counts", Value: model.NewData(
			model.Field{Key: "TNB", Value: int64(blocks)},
			model.Field{Key: "TNS", Value: int64(len(rd.cues))},
			model.Field{Key: "TNG", Value: int64(len(rd.groups))},
			model.Field{Key: "TCF", Value: firstIn},
		)},
	)
}

func nonNil(l []any) []any {
	if l == nil {
		return []any{}
	}
	return l
}

// cutIncompleteBlock fails on a file shorter than the GSI block. A partial last TTI block fails a strict read,
// and a lenient read cuts it.
func cutIncompleteBlock(st *parsing.State, content []byte) ([]byte, error) {
	if len(content) < gsiSize {
		return nil, &model.ParseError{Format: model.EBUSTL, Msg: "An EBU STL file starts with a GSI block of 1024 bytes."}
	}
	partial := (len(content) - gsiSize) % ttiSize
	if partial == 0 {
		return content, nil
	}
	complete := (len(content) - gsiSize) / ttiSize
	cut := gsiSize + complete*ttiSize
	err := st.Fail(&model.ParseError{Format: model.EBUSTL, Msg: "The TTI blocks of an EBU STL file must have 128 bytes each."},
		model.NoLine, complete, []string{hex.EncodeToString(content[cut:])})
	if err != nil {
		return nil, err
	}
	return content[:cut], nil
}

// readSets groups the TTI blocks into the sets of one subtitle. A set holds consecutive blocks with the same
// subtitle number, up to the block with extension block number FFh.
func readSets(tti []byte) [][][]byte {
	var sets [][][]byte
	var current [][]byte
	for len(tti) > 0 {
		block := tti[:ttiSize]
		tti = tti[ttiSize:]
		if current != nil && !bytes.Equal(current[0][ttiSN:ttiSN+2], block[ttiSN:ttiSN+2]) {
			sets = append(sets, current)
			current = nil
		}
		current = append(current, block)
		if block[ttiEBN] == lastBlock {
			sets = append(sets, current)
			current = nil
		}
	}
	if current != nil {
		sets = append(sets, current)
	}
	return sets
}

func (rd *reader) addSets(sets [][][]byte) {
	blockIndex := 0
	for _, blocks := range sets {
		first := blockIndex
		blockIndex += len(blocks)
		head := blocks[0]
		lines := rd.decodeLines(textBytes(blocks))
		hexes := make([]string, len(blocks))
		hexList := make([]any, len(blocks))
		for i, b := range blocks {
			hexes[i] = hex.EncodeToString(b)
			hexList[i] = hexes[i]
		}
		if head[ttiCF] == commentFlag {
			rd.addComment(lines, hexList)
			continue
		}
		if rd.st.Lenient() && !rd.validTimeCodes(head) {
			rd.st.Warn(fmt.Sprintf("Subtitle number %d has a time code that is not valid: %s to %s.",
				binary.LittleEndian.Uint16(head[ttiSN:]), timeCodeDigits(head[ttiTCI:]), timeCodeDigits(head[ttiTCO:])),
				model.NoLine, first, hexes, model.Skipped)
			continue
		}
		rd.groups[head[ttiSGN]] = true
		if rd.firstIn == "" {
			rd.firstIn = timeCodeDigits(head[ttiTCI:])
		}
		rd.cues = append(rd.cues, rd.cue(head, lines, hexList))
	}
}

func (rd *reader) addComment(lines []string, hexes []any) {
	var kept []string
	for _, l := range lines {
		if l = strings.Trim(l, markupcore.PHPTrimSet); l != "" {
			kept = append(kept, l)
		}
	}
	text := markupcore.PlainText(strings.Join(kept, "\n"))
	rd.comments = append(rd.comments, parsing.ParsedComment{Text: text, CuesBefore: len(rd.cues)})
	rd.commentData = append(rd.commentData, model.NewData(
		model.Field{Key: "text", Value: text},
		model.Field{Key: "blocks", Value: hexes},
	))
}

func (rd *reader) cue(head []byte, lines []string, hexes []any) model.Cue {
	c := model.Cue{
		Start:    rd.time(head[ttiTCI:]),
		End:      rd.time(head[ttiTCO:]),
		Position: rd.position(int(head[ttiVP]), head[ttiJC]),
	}
	for _, l := range lines {
		c.AddLine(l)
	}
	_ = c.SetFormatData(formatDataKey, model.NewData( // the data has the shape of the schema
		model.Field{Key: "subtitleGroupNumber", Value: int64(head[ttiSGN])},
		model.Field{Key: "cumulativeStatus", Value: int64(head[ttiCS])},
		model.Field{Key: "verticalPosition", Value: int64(head[ttiVP])},
		model.Field{Key: "justificationCode", Value: int64(head[ttiJC])},
		model.Field{Key: "text", Value: c.Text()},
		model.Field{Key: "blocks", Value: hexes},
	))
	return c
}

// validTimeCodes reports whether TCI and TCO keep hours to 23, minutes and seconds to 59 and the frames below the
// frame rate.
func (rd *reader) validTimeCodes(head []byte) bool {
	for _, tc := range [][]byte{head[ttiTCI : ttiTCI+timeCodeSize], head[ttiTCO : ttiTCO+timeCodeSize]} {
		if tc[0] > 23 || tc[1] > 59 || tc[2] > 59 || int64(tc[3]) >= rd.rate.Num/rd.rate.Den {
			return false
		}
	}
	return true
}

func (rd *reader) time(b []byte) time.Duration {
	d := rd.rate.ClockDuration(model.Timecode{Hours: int(b[0]), Minutes: int(b[1]), Seconds: int(b[2]), Frames: int(b[3])})
	return max(d-rd.offset, 0).Round(time.Millisecond)
}

// startOfProgramme returns the clock time of the TCP field, or 0 when it is not 8 ASCII digits.
func startOfProgramme(rate model.FrameRate, tcp string) time.Duration {
	if len(tcp) != 8 || strings.Trim(tcp, "0123456789") != "" {
		return 0
	}
	n := func(i int) int { return int(tcp[i]-'0')*10 + int(tcp[i+1]-'0') }
	return rate.ClockDuration(model.Timecode{Hours: n(0), Minutes: n(2), Seconds: n(4), Frames: n(6)})
}

func timeCodeDigits(b []byte) string {
	return fmt.Sprintf("%02d%02d%02d%02d", b[0], b[1], b[2], b[3])
}

// position returns the alignment from the vertical position and the justification code. The row and the grid
// are set only when the vertical position is on the grid.
func (rd *reader) position(vp int, jc byte) model.Position {
	p := model.Position{Alignment: alignmentOf(vp, jc, rd.maxRow)}
	if row := gridRow(vp, rd.teletext, rd.maxRow); row > 0 {
		p.Row, p.Grid = row, model.Grid{Rows: gridRows(rd.teletext, rd.maxRow), Columns: rd.columns}
	}
	return p
}

// alignmentOf returns the alignment of a vertical position and a justification code. The thirds of maxRow rows
// give the row, and the justification code gives the column.
func alignmentOf(vp int, jc byte, maxRow int) model.Alignment {
	column := 2
	switch jc {
	case 1:
		column = 1
	case 3:
		column = 3
	}
	band := 0
	switch {
	case 3*vp < maxRow:
		band = 2
	case 3*vp < 2*maxRow:
		band = 1
	}
	a, _ := model.AlignmentAt(band, column)
	return a
}

// gridRows returns the rows of the grid of a file: 23 for teletext, else the maximum row plus 1.
func gridRows(teletext bool, maxRow int) int {
	if teletext {
		return teletextRows
	}
	return maxRow + 1
}

// gridRow returns the grid row of a vertical position, or 0 when the position is outside the grid.
func gridRow(vp int, teletext bool, maxRow int) int {
	row := vp + 1
	if teletext {
		row = vp
	}
	if row < 1 || row > gridRows(teletext, maxRow) {
		return 0
	}
	return row
}

// textBytes joins the text fields of the blocks that carry subtitle text, EBN 00h to EFh and FFh, and removes
// the unused space code.
func textBytes(blocks [][]byte) []byte {
	var text []byte
	for _, b := range blocks {
		if b[ttiEBN] <= 0xEF || b[ttiEBN] == lastBlock {
			text = append(text, b[ttiHeaderSize:]...)
		}
	}
	return bytes.ReplaceAll(text, []byte{unusedSpace}, nil)
}

// teletextColor returns the color value of the alpha color code c, 0 to 7. White gives "".
func teletextColor(c int) string {
	return [...]string{"#000000", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", ""}[c]
}

// style is the state of the control codes while a text field decodes.
type style struct {
	color             int
	italic, underline bool
}

const white = 7

// apply changes the style by the control code c.
func (st *style) apply(c byte) {
	switch {
	case c < 0x08:
		st.color = int(c)
	case c == 0x80:
		st.italic = true
	case c == 0x81:
		st.italic = false
	case c == 0x82:
		st.underline = true
	case c == 0x83:
		st.underline = false
	}
}

// decodeLines converts a text field to cue lines in core markup. A teletext control code takes the place of a
// space. Italics and underline last until their off code, and the color returns to white at each new row.
func (rd *reader) decodeLines(b []byte) []string {
	var lines []string
	var runs []markupcore.Run
	st := style{color: white}
	start := 0
	flush := func(end int) {
		runs = append(runs, markupcore.Run{
			Text: decodeText(rd.cct, b[start:end]), Color: teletextColor(st.color), Italic: st.italic, Underline: st.underline,
		})
	}
	for i, c := range b {
		if c >= 0x20 && c < 0x80 || c >= 0xA0 {
			continue
		}
		flush(i)
		start = i + 1
		if c < 0x20 {
			runs = append(runs, markupcore.Run{Text: " "})
		}
		st.apply(c)
		if c == 0x8A {
			lines = append(lines, markupcore.StyleRuns(runs, true))
			runs, st.color = nil, white
		}
	}
	flush(len(b))
	return append(lines, markupcore.StyleRuns(runs, true))
}
