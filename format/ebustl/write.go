package ebustl

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/codepage"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	writeFrameRateOption = "WriteOptions.EBUSTL.FrameRate"
	maxSubtitleNumber    = 0xFFFF
	maxTextBlocks        = 0xF1
	maxColumns           = 99
	textFieldSize        = ttiSize - ttiHeaderSize
	newRow               = 0x8A
	italicsOn            = 0x80
	underlineOn          = 0x82
	userDataBlock        = 0xFE
)

// WriteOptions holds the settings of the EBU STL writer.
type WriteOptions struct {
	// FrameRate is the output frame rate, 25 or 30. The zero value takes the rate of the stored DFC, else 25.
	FrameRate model.FrameRate
}

// Validate returns an error that matches model.ErrInvalidArgument unless FrameRate is zero, 25 or 30.
func (o WriteOptions) Validate() error {
	if o.FrameRate.IsZero() || o.FrameRate.Equal(model.FrameRate25) || o.FrameRate.Equal(model.FrameRate30) {
		return nil
	}
	return &model.InvalidArgumentError{
		Option: writeFrameRateOption,
		Msg:    "The EBU STL formatter writes 25 or 30 fps, got " + o.FrameRate.String() + ".",
	}
}

// WriteReport tells which character code table the writer used and which characters it wrote as "?".
type WriteReport struct {
	// CharacterCodeTable is the table of the text, "00" to "04".
	CharacterCodeTable string
	// TableChosen is true when the writer chose the table, false when the stored GSI named it.
	TableChosen bool
	// Replaced lists each character that the output cannot hold, one entry per character and place, in output order.
	Replaced []Replacement
}

// Replacement is one character that the output cannot hold.
type Replacement struct {
	// CueIndex is the index of the cue, -1 for a comment or a GSI field.
	CueIndex int
	// Field is the GSI field name such as "OPT", empty for the text of a cue or a comment.
	Field string
	// Character is the character that the writer wrote as "?".
	Character rune
}

// set holds the TTI blocks of one subtitle or comment.
type set struct {
	blocks  [][]byte
	comment bool
}

type writer struct {
	common   model.CommonWriteOptions
	rate     model.FrameRate
	offset   time.Duration
	cct      string
	maxRow   int
	teletext bool
	replaced []Replacement
}

// storedComment is a comment of the stored file data with its blocks.
type storedComment struct {
	text   string
	blocks [][]byte
}

// Write writes s as EBU STL. The writer ignores the line ending and the BOM rule of common, because the output is
// binary.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.EBUSTL, common)
	if err != nil {
		return WriteReport{}, err
	}
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	file, _ := s.FormatData(formatDataKey)
	wr, gsi, report, err := newWriter(s, file, common, opts)
	if err != nil {
		return WriteReport{}, err
	}
	sets, err := wr.sets(s, storedComments(file))
	if err != nil {
		return WriteReport{}, err
	}
	number := 1
	if first, ok := file.Int("firstSubtitleNumber"); ok {
		number = int(first)
	}
	var tti []byte
	for _, st := range sets {
		if number > maxSubtitleNumber {
			return WriteReport{}, &model.UnwritableError{Format: model.EBUSTL, CueIndex: -1, Reason: "EBU STL allows subtitle numbers up to " + strconv.Itoa(maxSubtitleNumber) + "."}
		}
		for _, b := range st.blocks {
			binary.LittleEndian.PutUint16(b[ttiSN:], uint16(number))
			tti = append(tti, b...)
		}
		number++
	}
	gsiBlock, gsiReplaced := wr.gsiBlock(s, gsi, file, sets)
	report.Replaced = slices.Concat(gsiReplaced, wr.replaced)
	if err := writing.WriteBytes(w, model.EBUSTL, append(gsiBlock, tti...)); err != nil {
		return WriteReport{}, err
	}
	return report, nil
}

// newWriter returns the writer, the GSI fields that the write starts from and the report with its table.
func newWriter(s *model.Subtitle, file *model.Data, common model.CommonWriteOptions, opts WriteOptions) (*writer, map[string]string, WriteReport, error) {
	gsi := storedGSI(file)
	report := WriteReport{CharacterCodeTable: gsi["CCT"]}
	if _, ok := gsi["CCT"]; !ok {
		report.CharacterCodeTable, report.TableChosen = chooseTable(s), true
		gsi["CCT"] = report.CharacterCodeTable
	}
	if !slices.Contains([]string{"00", "01", "02", "03", "04"}, gsi["CCT"]) {
		return nil, nil, WriteReport{}, &model.InvalidArgumentError{Msg: `The character code table "` + gsi["CCT"] + `" is not 00, 01, 02, 03 or 04.`}
	}
	storedRate := storedFrameRate(gsi["DFC"])
	rate := cmp.Or(opts.FrameRate, storedRate, model.FrameRate25)
	gsi["DFC"] = "STL25.01"
	if rate.Equal(model.FrameRate30) {
		gsi["DFC"] = "STL30.01"
	}
	wr := &writer{common: common, rate: rate, cct: gsi["CCT"]}
	if subtracted, _ := file.Bool("startOfProgrammeSubtracted"); subtracted {
		wr.offset = startOfProgramme(cmp.Or(storedRate, rate), gsi["TCP"])
	}
	wr.teletext = gsi["DSC"] == "1" || gsi["DSC"] == "2"
	wr.maxRow = teletextRows
	if rows := fieldInt(gsi["MNR"]); !wr.teletext && rows > 0 {
		wr.maxRow = rows
	}
	return wr, gsi, report, nil
}

// storedGSI returns the stored GSI fields by name. A missing field has no entry.
func storedGSI(file *model.Data) map[string]string {
	gsi := map[string]string{}
	if stored, ok := file.Object("gsi"); ok {
		for _, f := range gsiFields {
			if v, ok := stored.String(f.name); ok {
				gsi[f.name] = v
			}
		}
	}
	for k, v := range map[string]string{
		"CPN": "850", "DSC": "1", "RN": "00", "MNC": "40", "MNR": "23", "TCS": "1", "TCP": "00000000", "TND": "1", "DSN": "1",
	} {
		if _, ok := gsi[k]; !ok {
			gsi[k] = v
		}
	}
	return gsi
}

func storedFrameRate(dfc string) model.FrameRate {
	switch dfc {
	case "STL25.01":
		return model.FrameRate25
	case "STL30.01":
		return model.FrameRate30
	default:
		return model.FrameRate{}
	}
}

func storedComments(file *model.Data) []storedComment {
	list, _ := file.List("comments")
	var out []storedComment
	for _, item := range list {
		c, ok := item.(*model.Data)
		if !ok {
			continue
		}
		text, _ := c.String("text")
		hexes, _ := c.Strings("blocks")
		out = append(out, storedComment{text: text, blocks: decodeBlocks(hexes)})
	}
	return out
}

// decodeBlocks decodes hex blocks. It returns nil unless every block is a valid hex string of 128 bytes.
func decodeBlocks(hexes []string) [][]byte {
	blocks := make([][]byte, len(hexes))
	for i, h := range hexes {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != ttiSize {
			return nil
		}
		blocks[i] = b
	}
	return blocks
}

// sets returns the sets in output order: the comments before each cue, then the cue.
func (wr *writer) sets(s *model.Subtitle, stored []storedComment) ([]set, error) {
	n := s.Len()
	cues := make([]model.Cue, 0, n)
	for _, c := range s.All() {
		cues = append(cues, c)
	}
	comments := make([][]string, n+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, n)
		comments[at] = append(comments[at], c.Text)
	}
	var sets []set
	for i := range n + 1 {
		for _, text := range comments[i] {
			var at time.Duration
			switch {
			case i < n:
				at = cues[i].Start
			case n > 0:
				at = cues[n-1].End
			}
			tc := wr.timeCode(at)
			blocks, err := wr.commentBlocks(text, &stored, tc)
			if err != nil {
				return nil, err
			}
			sets = append(sets, set{blocks: blocks, comment: true})
		}
		if i < n {
			blocks, err := wr.cueBlocks(i, cues[i])
			if err != nil {
				return nil, err
			}
			sets = append(sets, set{blocks: blocks})
		}
	}
	return sets, nil
}

// commentBlocks returns the stored blocks of the first unused comment with the same text, else new blocks.
func (wr *writer) commentBlocks(text string, stored *[]storedComment, tc [4]byte) ([][]byte, error) {
	for i, c := range *stored {
		if c.text == text && len(c.blocks) > 0 {
			*stored = slices.Delete(*stored, i, i+1)
			return c.blocks, nil
		}
	}
	lines := strings.Split(text, "\n")
	rows := make([][]byte, len(lines))
	for i, line := range lines {
		rows[i] = wr.encode(line, -1)
	}
	vp, jc := wr.alignedPosition(model.BottomCenter, len(lines))
	h := header{tci: tc, tco: tc, vp: vp, jc: jc, comment: true}
	return wr.textBlocks(bytes.Join(rows, []byte{newRow}), h, -1)
}

// cueBlocks returns the blocks of a cue: the stored blocks with a patched header when the text is unchanged,
// else the stored user data blocks and the new text blocks.
func (wr *writer) cueBlocks(index int, c model.Cue) ([][]byte, error) {
	data, _ := c.FormatData(formatDataKey)
	h := header{tci: wr.timeCode(c.Start), tco: wr.timeCode(c.End)}
	if n, ok := data.Int("subtitleGroupNumber"); ok {
		h.sgn = byte(n)
	}
	if n, ok := data.Int("cumulativeStatus"); ok {
		h.cs = byte(n)
	}
	h.vp, h.jc = wr.cuePosition(c, data)
	hexes, _ := data.Strings("blocks")
	blocks := decodeBlocks(hexes)
	if stored, _ := data.String("text"); len(blocks) > 0 && !wr.common.StripTags && stored == c.Text() {
		return patchHeaders(blocks, h), nil
	}
	var user [][]byte
	for _, b := range blocks {
		if b[ttiEBN] == userDataBlock {
			user = append(user, b)
		}
	}
	text, err := wr.textBlocks(wr.encodeText(c, index), h, index)
	if err != nil {
		return nil, err
	}
	return append(patchHeaders(user, h), text...), nil
}

// cuePosition returns the vertical position and the justification code of a cue. The stored pair stays when it
// still gives the alignment and the row of the cue.
func (wr *writer) cuePosition(c model.Cue, data *model.Data) (vp, jc byte) {
	if vp, jc, ok := wr.storedPosition(c, data); ok {
		return vp, jc
	}
	lines := max(len(c.Lines), 1)
	grid := model.Grid{Rows: gridRows(wr.teletext, wr.maxRow)}
	if row, ok := c.Position.RowOn(grid, lines); ok {
		pos := row
		if !wr.teletext {
			pos--
		}
		if pos >= 0 && pos <= 0xFF && alignmentOf(pos, 2, wr.maxRow).Row() == c.Position.Alignment.Row() {
			return byte(pos), justification(c.Position.Alignment)
		}
	}
	return wr.alignedPosition(c.Position.Alignment, lines)
}

// storedPosition returns the stored vertical position and justification code when both exist, give the alignment of
// the cue and give the row of the cue.
func (wr *writer) storedPosition(c model.Cue, data *model.Data) (vp, jc byte, ok bool) {
	storedVP, okVP := data.Int("verticalPosition")
	storedJC, okJC := data.Int("justificationCode")
	if !okVP || !okJC || storedVP < 0 || storedVP > 0xFF || storedJC < 0 || storedJC > 0xFF {
		return 0, 0, false
	}
	holds := alignmentOf(int(storedVP), byte(storedJC), wr.maxRow) == c.Position.Alignment &&
		c.Position.Row == gridRow(int(storedVP), wr.teletext, wr.maxRow)
	return byte(storedVP), byte(storedJC), holds
}

func justification(a model.Alignment) byte {
	switch a.Column() {
	case 1:
		return 1
	case 3:
		return 3
	default:
		return 2
	}
}

// alignedPosition returns the vertical position and the justification code that put the first row of a cue of
// lines rows at the top, in the middle or at the bottom of the rows.
func (wr *writer) alignedPosition(a model.Alignment, lines int) (vp, jc byte) {
	firstRow := 0
	if wr.teletext {
		firstRow = 1
	}
	lastRow := max(firstRow, wr.maxRow-max(lines, 1)+1)
	var row int
	switch a.Row() {
	case 2:
		row = firstRow
	case 1:
		row = (firstRow + lastRow) / 2
	default:
		row = lastRow
	}
	return byte(min(row, 0xFF)), justification(a)
}

// header holds the TTI header fields that the writer sets. The subtitle number comes later.
type header struct {
	sgn, cs, vp, jc byte
	tci, tco        [4]byte
	comment         bool
}

func (h header) bytes() []byte {
	b := make([]byte, ttiHeaderSize)
	b[ttiSGN], b[ttiEBN], b[ttiCS] = h.sgn, lastBlock, h.cs
	copy(b[ttiTCI:], h.tci[:])
	copy(b[ttiTCO:], h.tco[:])
	b[ttiVP], b[ttiJC] = h.vp, h.jc
	if h.comment {
		b[ttiCF] = commentFlag
	}
	return b
}

// patchHeaders copies the SGN, CS, TCI, TCO, VP and JC of h into each block that holds the value of the first
// block for that field. Other values and the EBN stay.
func patchHeaders(blocks [][]byte, h header) [][]byte {
	if len(blocks) == 0 {
		return nil
	}
	want := h.bytes()
	first := blocks[0]
	out := make([][]byte, len(blocks))
	for i, b := range blocks {
		out[i] = bytes.Clone(b)
	}
	for _, f := range [][2]int{{ttiSGN, 1}, {ttiCS, 1}, {ttiTCI, timeCodeSize}, {ttiTCO, timeCodeSize}, {ttiVP, 1}, {ttiJC, 1}} {
		old, now := first[f[0]:f[0]+f[1]], want[f[0]:f[0]+f[1]]
		if bytes.Equal(old, now) {
			continue
		}
		old = bytes.Clone(old)
		for _, b := range out {
			if bytes.Equal(b[f[0]:f[0]+f[1]], old) {
				copy(b[f[0]:], now)
			}
		}
	}
	return out
}

// textBlocks splits text into 112-byte fields. The last field holds at least one 8Fh. In table 00 a field that
// would end with a diacritic ends one byte early, so the mark stays with its letter.
func (wr *writer) textBlocks(text []byte, h header, cueIndex int) ([][]byte, error) {
	var fields [][]byte
	for len(text) >= textFieldSize {
		n := textFieldSize
		if wr.cct == "00" && isDiacriticByte(text[n-1]) {
			n--
		}
		fields = append(fields, padField(text[:n]))
		text = text[n:]
	}
	fields = append(fields, padField(text))
	if len(fields) > maxTextBlocks {
		return nil, &model.UnwritableError{
			Format: model.EBUSTL, CueIndex: cueIndex,
			Reason: "The cue text needs more than " + strconv.Itoa(maxTextBlocks) + " TTI blocks.",
		}
	}
	blocks := make([][]byte, len(fields))
	for i, f := range fields {
		b := append(h.bytes(), f...)
		b[ttiEBN] = byte(i)
		if i == len(fields)-1 {
			b[ttiEBN] = lastBlock
		}
		blocks[i] = b
	}
	return blocks, nil
}

// isDiacriticByte reports whether b is a diacritic code of ISO 6937, C1h to CFh without C9h.
func isDiacriticByte(b byte) bool {
	return b >= 0xC1 && b <= 0xCF && b != 0xC9
}

func padField(b []byte) []byte {
	return append(bytes.Clone(b), bytes.Repeat([]byte{unusedSpace}, textFieldSize-len(b))...)
}

// timeCode returns the 4 time code bytes of d plus the start of programme.
func (wr *writer) timeCode(d time.Duration) [4]byte {
	t := wr.rate.TimecodeOf(wr.rate.Frames(max(d+wr.offset, 0)))
	return [4]byte{byte(min(t.Hours, 0xFF)), byte(t.Minutes), byte(t.Seconds), byte(t.Frames)}
}

// encode encodes text with the table of the file and records each character written as "?".
func (wr *writer) encode(text string, cueIndex int) []byte {
	out, replaced := encodeWith(wr.cct, text)
	for _, r := range replaced {
		wr.replaced = append(wr.replaced, Replacement{CueIndex: cueIndex, Character: r})
	}
	return out
}

// encodeWith encodes text with the character code table cct and returns the code points written as "?". A "?" in
// text is not one of them.
func encodeWith(cct, text string) ([]byte, []rune) {
	table := textTable(cct)
	if table == nil {
		return codepage.EncodeISO6937Replaced(text)
	}
	var replaced []rune
	out := make([]byte, 0, len(text))
	for _, r := range text {
		c, ok := table.EncodeRune(r)
		if !ok {
			c = codepage.Replacement
			replaced = append(replaced, r)
		}
		out = append(out, c)
	}
	return out, replaced
}

// textTable returns the code page of a character code table, nil for table 00 (ISO 6937).
func textTable(cct string) *codepage.Table {
	switch cct {
	case "01":
		return codepage.ISO8859_5
	case "02":
		return codepage.ISO8859_6
	case "03":
		return codepage.ISO8859_7_1987
	case "04":
		return codepage.ISO8859_8_1988
	default:
		return nil
	}
}

// chooseTable returns the character code table that writes the fewest "?" for the text of the cues and comments.
// Table 00 wins a tie. It counts "?" in the encoded bytes, a "?" of the text too.
func chooseTable(s *model.Subtitle) string {
	var texts []string
	for _, c := range s.All() {
		texts = append(texts, markupcore.PlainText(c.Text()))
	}
	for _, c := range s.Comments() {
		texts = append(texts, c.Text)
	}
	text := strings.ReplaceAll(strings.Join(texts, " "), "\n", " ")
	best, fewest := "", 0
	for _, cct := range []string{"00", "01", "02", "03", "04"} {
		encoded, _ := encodeWith(cct, text)
		if n := bytes.Count(encoded, []byte{codepage.Replacement}); best == "" || n < fewest {
			best, fewest = cct, n
		}
	}
	return best
}
