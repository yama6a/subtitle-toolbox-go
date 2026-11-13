package pac

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

const (
	maxNumber = 0xFFFF
	maxHours  = 654 // the 2 bytes hold hours * 100 + minutes
	// A new row of text in PAC Unicode starts with these 3 bytes, as Subtitle Edit writes them.
	unicodePrefix = "\x80\x80\x80"
	topRow        = 1
	middleRow     = 6
	lastRow       = gridRows - 1
)

// Write writes s as PAC, or as PAC Unicode when the code page is Unicode. It keeps the code page, the frame rate,
// the header and the footer that the reader stored, and the bytes of each cue that stay unchanged. Without a stored
// code page, a subtitle that Latin can hold gets Latin, else Unicode. The writer ignores LineEnding and BOM.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.PAC, common)
	if err != nil {
		return WriteReport{}, err
	}
	file, _ := s.FormatData(formatDataKey)
	var codePage CodePage
	if stored, ok := file.String("codePage"); ok && CodePage(stored).known() {
		codePage = CodePage(stored)
	} else {
		codePage = newCodePage(s)
	}
	rate := model.FrameRate25
	if stored, ok := file.Int("frameRate"); ok && stored == frameRate30 {
		rate = model.FrameRate30
	}
	number, ok := file.Int("firstNumber")
	if !ok || number < 0 {
		number = 0
	}

	body := slices.Clone(storedBytes(file, "header", header, validHeader))
	for i, c := range s.All() {
		record, err := appendRecord(body, c, i, number+int64(i), codePage, rate, common.StripTags)
		if err != nil {
			return WriteReport{}, err
		}
		body = record
	}
	body = append(body, storedBytes(file, "footer", defaultFooter, validFooter)...)
	return WriteReport{}, writing.WriteBytes(w, model.PAC, body)
}

// appendRecord appends the record of cue number index to body.
func appendRecord(body []byte, c model.Cue, index int, number int64, codePage CodePage, rate model.FrameRate, stripTags bool) ([]byte, error) {
	if number > maxNumber {
		return nil, &model.UnwritableError{Format: model.PAC, CueIndex: index, Reason: "PAC allows subtitle numbers up to 65535."}
	}
	vp, text := cueText(c, codePage, stripTags)
	if len(text) >= maxNumber {
		return nil, &model.UnwritableError{Format: model.PAC, CueIndex: index, Reason: "PAC allows cue text of up to 65534 bytes."}
	}
	body = append(body, recordStart)
	body = binary.LittleEndian.AppendUint16(body, uint16(number))
	body = append(body, recordMark)
	body = appendTimeCode(body, c.Start, rate)
	body = appendTimeCode(body, c.End, rate)
	body = binary.LittleEndian.AppendUint16(body, uint16(len(text)+1))
	body = append(body, byte(vp))
	return append(body, text...), nil
}

// storedBytes returns the bytes of the hex string of key when it decodes and ok accepts it, else fallback.
func storedBytes(d *model.Data, key string, fallback []byte, ok func([]byte) bool) []byte {
	if s, found := d.String(key); found {
		if b, err := hex.DecodeString(s); err == nil && ok(b) {
			return b
		}
	}
	return fallback
}

// validHeader reports whether b is the 20 header bytes, optionally followed by one complete record, the story record.
func validHeader(b []byte) bool {
	if !bytes.HasPrefix(b, header) {
		return false
	}
	rest := b[headerLength:]
	if len(rest) == 0 {
		return true
	}
	return len(rest) >= offsetText && rest[0] == recordStart && rest[offsetMark] >= recordMark && rest[offsetMark] <= lastRecordMark &&
		offsetText-1+le16(rest, offsetLength) == len(rest)
}

// validFooter reports whether b is empty or starts with the end block byte, so that a reader accepts it.
func validFooter(b []byte) bool { return len(b) == 0 || b[0] == endBlock }

// newCodePage returns Latin when it holds the text of every cue, else Unicode.
func newCodePage(s *model.Subtitle) CodePage {
	replacer := strings.NewReplacer("\u2026", "...", "\n", " ")
	for _, c := range s.All() {
		text := replacer.Replace(markupcore.PlainText(c.Text()))
		if bytes.Contains(Latin.encodeSingleByte(strings.ReplaceAll(text, replacement, "")), []byte(replacement)) {
			return Unicode
		}
	}
	return Latin
}

// storedCue is the format data that the reader stored for a cue.
type storedCue struct {
	vp        int
	alignment byte
	text      string
	bytes     []byte
}

func readStoredCue(c model.Cue) (storedCue, bool) {
	d, _ := c.FormatData(formatDataKey)
	vp, okVP := d.Int("verticalPosition")
	alignment, okAlignment := d.Int("alignment")
	text, okText := d.String("text")
	raw, okBytes := d.String("bytes")
	b, err := hex.DecodeString(raw)
	if !okVP || !okAlignment || !okText || !okBytes || err != nil || vp < 0 || vp > 255 || alignment < 0 || alignment > 255 {
		return storedCue{}, false
	}
	return storedCue{vp: int(vp), alignment: byte(alignment), text: text, bytes: b}, true
}

// keeps reports whether the cue still has the text and the position that the reader gave it.
func (s storedCue) keeps(c model.Cue) bool {
	read := position(s.vp, s.alignment)
	return s.text == c.Text() && read.Alignment == c.Position.Alignment && read.Row == c.Position.Row
}

// cueText returns the vertical position and the text bytes of a record: the stored bytes of a cue that keeps its
// text and position, else new bytes.
func cueText(c model.Cue, codePage CodePage, stripTags bool) (int, []byte) {
	stored, hasStored := readStoredCue(c)
	if hasStored && !stripTags && stored.keeps(c) {
		return stored.vp, stored.bytes
	}

	lines := markupcore.CarryOpenTags(mapRuby(c.Lines))
	vp, ok := rowVerticalPosition(c.Position, len(lines))
	if !ok {
		vp = newVerticalPosition(c.Position, len(lines))
		if hasStored && strings.Count(stored.text, "\n")+1 == len(lines) && bandOf(stored.vp) == bandOf(vp) {
			vp = stored.vp
		}
	}
	alignment := alignmentByte(c.Position.Alignment)

	var out []byte
	if codePage == Unicode {
		out = append(out, unicodePrefix...)
	}
	for _, l := range lines {
		out = append(out, lineStart, alignment, lineEnd)
		out = append(out, codePage.encodeLine(lineText(l, codePage, stripTags))...)
	}
	return vp, out
}

func alignmentByte(a model.Alignment) byte {
	switch a.Column() {
	case 1:
		return alignLeft
	case 3:
		return alignRight
	}
	return alignCenter
}

func mapRuby(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = markupcore.RubyAsText(l)
	}
	return out
}

// bandOf returns the alignment row of a vertical position, as Alignment.Row counts: 2 top, 1 middle, 0 bottom.
func bandOf(vp int) int {
	switch {
	case vp < firstMiddleRow:
		return 2
	case vp < firstBottomRow:
		return 1
	}
	return 0
}

// rowVerticalPosition returns the vertical position of the row of p on the grid of PAC, when the row exists and its
// vertical position is in the band of the alignment.
func rowVerticalPosition(p model.Position, lines int) (int, bool) {
	row, ok := p.RowOn(model.Grid{Rows: gridRows}, lines)
	if !ok || bandOf(row-1) != p.Alignment.Row() {
		return 0, false
	}
	return row - 1, true
}

// newVerticalPosition returns the vertical position of a cue without a usable row: the first row of its band.
func newVerticalPosition(p model.Position, lines int) int {
	switch p.Alignment.Row() {
	case 2:
		return topRow
	case 1:
		return middleRow
	}
	return max(firstBottomRow, lastRow+1-lines)
}

// lineText converts the core markup of a line to PAC text with the italics codes "<" and ">". A code takes the
// place of the space between it and a letter or digit. The end of the line ends italics, so a ">" there goes.
func lineText(line string, codePage CodePage, stripTags bool) string {
	runes := dropSpaces([]rune(italicsText(line, stripTags)))
	if n := len(runes); n > 0 && runes[n-1] == '>' {
		runes = runes[:n-1]
	}
	out := string(runes)
	if codePage != Unicode {
		out = strings.ReplaceAll(out, "\u2026", "...")
	}
	return codePage.rightToLeftLine(out)
}

// italicsText decodes the entities of a line and writes <i> as "<" and </i> as ">", with nesting counted. A literal
// "<" or ">" becomes "?". Other tags drop, all tags with stripTags.
func italicsText(line string, stripTags bool) string {
	var text strings.Builder
	italic := 0
	for i, token := range markupcore.SplitTags(line) {
		if i%2 == 0 {
			text.WriteString(strings.NewReplacer("<", replacement, ">", replacement).Replace(markupcore.DecodeEntities(token)))
			continue
		}
		tag := markupcore.ASCIILower(token)
		switch {
		case stripTags:
		case tag == "<i>":
			if italic == 0 {
				text.WriteByte('<')
			}
			italic++
		case tag == "</i>" && italic > 0:
			italic--
			if italic == 0 {
				text.WriteByte('>')
			}
		}
	}
	return text.String()
}

// dropSpaces removes a space before "<" after a letter or digit, and a space after ">" before a letter or digit.
func dropSpaces(runes []rune) []rune {
	return dropSpace(dropSpace(runes, func(before, after rune) bool {
		return after == '<' && isLetterOrNumber(before)
	}), func(before, after rune) bool {
		return before == '>' && isLetterOrNumber(after)
	})
}

// dropSpace removes each space for which drop returns true. drop gets the runes around the space, 0 at the ends.
func dropSpace(runes []rune, drop func(before, after rune) bool) []rune {
	out := make([]rune, 0, len(runes))
	for i, r := range runes {
		var before, after rune
		if i > 0 {
			before = runes[i-1]
		}
		if i+1 < len(runes) {
			after = runes[i+1]
		}
		if r != ' ' || !drop(before, after) {
			out = append(out, r)
		}
	}
	return out
}

// appendTimeCode appends the 2 little-endian numbers hours * 100 + minutes and seconds * 100 + frames.
func appendTimeCode(b []byte, d time.Duration, rate model.FrameRate) []byte {
	t := rate.TimecodeOf(rate.Frames(max(d, 0)))
	b = binary.LittleEndian.AppendUint16(b, uint16(min(t.Hours, maxHours)*100+t.Minutes))
	return binary.LittleEndian.AppendUint16(b, uint16(t.Seconds*100+t.Frames))
}
