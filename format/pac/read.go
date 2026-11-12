package pac

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Pac.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Pac.cs), MIT.

const (
	headerLength   = 20
	recordStart    = 0x00
	recordMark     = 0x60
	lastRecordMark = 0x67 // Subtitle Edit reads the marks 60h to 67h
	endBlock       = 0xFF

	// The offsets of the parts of a record, from its first byte.
	offsetNumber   = 1
	offsetMark     = 3
	offsetTimeIn   = 4
	offsetTimeOut  = 8
	offsetLength   = 12
	offsetVertical = 14
	offsetText     = 15

	frameRate25 = 25
	frameRate30 = 30
)

//nolint:gochecknoglobals // the fixed header and footer of a new file
var (
	header        = append([]byte{0x01}, make([]byte, headerLength-1)...)
	defaultFooter = append(append([]byte{0xFF}, make([]byte, 11)...), append([]byte{0x11, 0x00}, "dummy end of file"...)...)
)

// storyLanguages maps the language codes of the LANG line of a story record, as Screen writes them.
//
//nolint:gochecknoglobals // lookup table of the story languages
var storyLanguages = map[string]CodePage{
	"DAR": Arabic, "OAR": Arabic, "ARA": Arabic, "ARB": Arabic, "ISR": Hebrew, "HEB": Hebrew,
	"GRC": Greek, "GRE": Greek, "ELL": Greek, "DRU": Cyrillic, "RUS": Cyrillic, "BGR": Cyrillic,
	"MKD": Cyrillic, "MNG": Cyrillic, "UKR": Cyrillic, "THA": Thai, "TRD": ChineseTraditional,
	"OCH": ChineseTraditional, "CHT": ChineseTraditional, "SIM": ChineseSimplified, "CHS": ChineseSimplified,
	"DJA": Japanese, "JPN": Japanese, "DKO": Korean, "KOR": Korean, "TUR": LatinTurkish,
	"CSK": LatinCzech, "CZE": LatinCzech,
}

var (
	storyRegexp = regexp.MustCompile(`[Ss][Tt][Oo][Rr][Yy]:`)
	langRegexp  = regexp.MustCompile(`[Ll][Aa][Nn][Gg]:([A-Za-z]{3})`)
)

type reader struct {
	st       *parsing.State
	rate     model.FrameRate
	codePage CodePage
	records  [][]byte
	footer   []byte
	story    []byte
	allLines [][]line
	cues     []model.Cue
}

// Read reads a PAC or PAC Unicode file from its raw bytes.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.PAC, err)
	}
	if !bytes.HasPrefix(content, header) {
		return nil, nil, &model.ParseError{Format: model.PAC, Msg: "A PAC file starts with the byte 01h and 19 zero bytes."}
	}
	rd := &reader{st: parsing.NewState(model.PAC, common)}
	if err := rd.splitRecords(content); err != nil {
		return nil, nil, err
	}
	head := slices.Clone(header)
	if len(rd.records) > 0 && storyRegexp.Match(rd.records[0][offsetText:]) {
		rd.story, rd.records = rd.records[0], rd.records[1:]
		head = append(head, rd.story...)
	}
	if err := rd.addCues(opts.CodePage); err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	parsing.AddParsed(s, rd.cues, nil)
	_ = s.SetFormatData(formatDataKey, rd.fileData(head)) // the data has the shape of the schema
	return s, rd.st.Warnings(), nil
}

// addCues chooses the code page and the frame rate, then reads each record as a cue.
func (rd *reader) addCues(option CodePage) error {
	rd.allLines = make([][]line, len(rd.records))
	for i, rec := range rd.records {
		rd.allLines[i] = splitLines(rec[offsetText:])
	}
	var err error
	if rd.codePage, err = rd.chooseCodePage(option); err != nil {
		return err
	}
	rd.rate = frameRateOf(rd.records)
	for i, rec := range rd.records {
		if err := rd.addCue(i, rec); err != nil {
			return err
		}
	}
	return nil
}

func (rd *reader) fileData(head []byte) *model.Data {
	first := int64(0)
	if len(rd.records) > 0 {
		first = int64(binary.LittleEndian.Uint16(rd.records[0][offsetNumber:]))
	}
	return model.NewData(
		model.Field{Key: "codePage", Value: string(rd.codePage)},
		model.Field{Key: "frameRate", Value: rd.rate.Num},
		model.Field{Key: "firstNumber", Value: first},
		model.Field{Key: "header", Value: hex.EncodeToString(head)},
		model.Field{Key: "footer", Value: hex.EncodeToString(rd.footer)},
	)
}

// le16 reads a little-endian number of up to 2 bytes at i, where missing bytes count as 0.
func le16(b []byte, i int) int {
	n := 0
	for k := 1; k >= 0; k-- {
		n <<= 8
		if i+k < len(b) {
			n |= int(b[i+k])
		}
	}
	return n
}

// splitRecords splits the bytes after the header into records and the end block. A record starts with 00h.
func (rd *reader) splitRecords(content []byte) error {
	offset := headerLength
	for offset < len(content) && content[offset] == recordStart {
		var mark byte
		if offset+offsetMark < len(content) {
			mark = content[offset+offsetMark]
		}
		end := offset + offsetText - 1 + le16(content, offset+offsetLength)
		if mark < recordMark || mark > lastRecordMark || end > len(content) || end < offset+offsetText {
			rd.footer = content[offset:]
			return rd.st.Fail(&model.ParseError{Format: model.PAC, Msg: "The PAC record at byte " + strconv.Itoa(offset) + " is not complete."},
				model.NoLine, len(rd.records), []string{hex.EncodeToString(rd.footer)})
		}
		rd.records = append(rd.records, content[offset:end])
		offset = end
	}
	rd.footer = content[offset:]
	if len(rd.footer) > 0 && rd.footer[0] != endBlock {
		return rd.st.Fail(&model.ParseError{Format: model.PAC, Msg: "The byte at " + strconv.Itoa(offset) + " is neither a PAC record nor the end block."},
			model.NoLine, len(rd.records), []string{hex.EncodeToString(rd.footer)})
	}
	return nil
}

// chooseCodePage takes the code page of the option, else of the LANG line of the story record, else detects it.
func (rd *reader) chooseCodePage(option CodePage) (CodePage, error) {
	if option != CodePageAuto {
		return option, nil
	}
	if m := langRegexp.FindSubmatch(rd.story); m != nil {
		if c, ok := storyLanguages[strings.ToUpper(string(m[1]))]; ok {
			return c, nil
		}
	}
	var runs [][]run
	for _, lines := range rd.allLines {
		for _, l := range lines {
			runs = append(runs, l.runs)
		}
	}
	c, reason := detectCodePage(runs)
	if reason == "" {
		return c, nil
	}
	msg := "The code page of the PAC file is not clear. " + reason + " Set the code page of the file, for example ISO-8859-7 for Greek."
	if !rd.st.Lenient() {
		return CodePageAuto, &model.ParseError{Format: model.PAC, Msg: msg}
	}
	rd.st.Warn(msg+" The parser read it as "+string(c)+".", model.NoLine, model.NoBlock, nil, model.Repaired)
	return c, nil
}

// frameRateOf returns 30 fps when a time code has a frame number from 25 to 29, else 25 fps. The file does not
// store the frame rate.
func frameRateOf(records [][]byte) model.FrameRate {
	for _, rec := range records {
		for _, offset := range [...]int{offsetTimeIn, offsetTimeOut} {
			if frames := le16(rec, offset+2) % 100; frames >= frameRate25 && frames <= 29 {
				return model.FrameRate30
			}
		}
	}
	return model.FrameRate25
}

func (rd *reader) addCue(index int, rec []byte) error {
	start, okStart := rd.time(rec[offsetTimeIn:])
	end, okEnd := rd.time(rec[offsetTimeOut:])
	if !okStart || !okEnd {
		return rd.st.Fail(&model.ParseError{Format: model.PAC, Msg: "Subtitle number " + strconv.Itoa(le16(rec, offsetNumber)) + " has a time code that is not valid."},
			model.NoLine, index, []string{hex.EncodeToString(rec)})
	}
	lines := rd.allLines[index]
	vp := int(rec[offsetVertical])
	alignment := byte(alignCenter)
	if len(lines) > 0 {
		alignment = lines[0].alignment
	}
	c := model.Cue{Start: start, End: end, Position: position(vp, alignment)}
	for _, l := range lines {
		text, _ := rd.codePage.decodeLine(l.runs)
		c.AddLine(markup(rd.codePage.rightToLeftLine(text)))
	}
	_ = c.SetFormatData(formatDataKey, model.NewData( // the data has the shape of the schema
		model.Field{Key: "verticalPosition", Value: int64(vp)},
		model.Field{Key: "alignment", Value: int64(alignment)},
		model.Field{Key: "text", Value: c.Text()},
		model.Field{Key: "bytes", Value: hex.EncodeToString(rec[offsetText:])},
	))
	rd.cues = append(rd.cues, c)
	return nil
}

// time reads a time code of 2 little-endian numbers: hours * 100 + minutes, and seconds * 100 + frames.
func (rd *reader) time(b []byte) (time.Duration, bool) {
	hoursMinutes, secondsFrames := le16(b, 0), le16(b, 2)
	t := model.Timecode{Hours: hoursMinutes / 100, Minutes: hoursMinutes % 100, Seconds: secondsFrames / 100, Frames: secondsFrames % 100}
	if t.Minutes > 59 || t.Seconds > 59 || int64(t.Frames) >= rd.rate.Num {
		return 0, false
	}
	return rd.rate.ClockDuration(t).Round(time.Millisecond), true
}

// The rows of the vertical position: 0 to 4 top, 5 to 8 middle, 9 and up bottom.
const (
	firstMiddleRow = 5
	firstBottomRow = 9
	gridRows       = 12
)

// position returns the position of a vertical position and an alignment byte. A vertical position above 11 gives
// no row.
func position(vp int, alignment byte) model.Position {
	band := 0
	switch {
	case vp < firstMiddleRow:
		band = 2
	case vp < firstBottomRow:
		band = 1
	}
	column := 2
	switch alignment & alignMask {
	case alignLeft:
		column = 1
	case alignRight:
		column = 3
	}
	a, _ := model.AlignmentAt(band, column)
	p := model.Position{Alignment: a}
	if vp < gridRows {
		p.Row, p.Grid = vp+1, model.Grid{Rows: gridRows}
	}
	return p
}

// markup converts a text line to core markup. "<" starts italics, and ">" or the end of the line ends them.
// An italics code takes the place of the space between it and a letter or digit.
func markup(text string) string {
	runes := spaceItalics(text)
	var runs []markupcore.Run
	start, italic := 0, false
	for i, r := range runes {
		if r != '<' && r != '>' {
			continue
		}
		runs = append(runs, markupcore.Run{Text: string(runes[start:i]), Italic: italic})
		start, italic = i+1, r == '<'
	}
	runs = append(runs, markupcore.Run{Text: string(runes[start:]), Italic: italic})
	return markupcore.StyleRuns(runs, true)
}

// spaceItalics puts a space before each "<" that follows a letter or digit, and after each ">" that a letter or
// digit follows.
func spaceItalics(text string) []rune {
	runes := []rune(text)
	out := make([]rune, 0, len(runes))
	for i, r := range runes {
		if r == '<' && i > 0 && isLetterOrNumber(runes[i-1]) {
			out = append(out, ' ')
		}
		out = append(out, r)
		if r == '>' && i+1 < len(runes) && isLetterOrNumber(runes[i+1]) {
			out = append(out, ' ')
		}
	}
	return out
}

func isLetterOrNumber(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }
