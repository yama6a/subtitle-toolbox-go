package mp4

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	textLengthSize = 2
	styleCountSize = 2
	styleRecord    = 12
	maxModifiers   = boxHeaderSize + styleCountSize + 65535*styleRecord
)

// styleRun is a record of a styl box: the first and the last character of a run and its face style flags.
type styleRun struct {
	start, end int
	flags      byte
}

// tx3gCues reads the text of each sample as a cue. An empty sample is a gap.
func (f *File) tx3gCues(w *walker) ([]model.Cue, error) {
	var cues []model.Cue
	for {
		s, ok, err := w.next()
		if err != nil || !ok {
			return cues, err
		}
		if err := w.outside(s); err != nil {
			return nil, err
		}
		lines, err := f.sampleLines(w, s)
		if err != nil {
			return nil, err
		}
		if len(lines) > 0 {
			cues = append(cues, model.Cue{Start: millis(s.start), End: millis(s.end), Lines: lines, Forced: w.t.info.Forced})
		}
	}
}

// sampleLines returns the lines of the text of a tx3g sample, with the styl runs as tags.
// It reads the length, the text and at most maxModifiers bytes of modifier boxes.
func (f *File) sampleLines(w *walker, s sample) ([]string, error) {
	if s.size < textLengthSize {
		return nil, nil
	}
	head, err := readAt(f.r, s.offset, textLengthSize)
	if err != nil {
		return nil, err
	}
	length := uint64(binary.BigEndian.Uint16(head))
	where := " of the sample of track " + strconv.Itoa(w.number) + " at byte " + strconv.FormatUint(s.offset, 10)
	if length > uint64(s.size)-textLengthSize {
		return nil, parseError("The text" + where + " is longer than the sample.")
	}
	if length == 0 {
		return nil, nil
	}
	raw, err := readAt(f.r, s.offset+textLengthSize, length)
	if err != nil {
		return nil, err
	}
	text, ok := decodeText(raw)
	if !ok {
		return nil, parseError("The text" + where + " is not valid UTF-8.")
	}
	var runs []styleRun
	if rest := uint64(s.size) - textLengthSize - length; rest >= boxHeaderSize {
		modifiers, err := readAt(f.r, s.offset+textLengthSize+length, min(rest, maxModifiers))
		if err != nil {
			return nil, err
		}
		runs = readStyles(modifiers)
	}
	out := markupcore.NormalizeLineEnds(addStyleTags(text, runs))
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// decodeText returns the text of a tx3g sample. 3GPP TS 26.245 allows only UTF-8 and UTF-16 with a byte order mark.
func decodeText(raw []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(raw, []byte{0xFE, 0xFF}), bytes.HasPrefix(raw, []byte{0xFF, 0xFE}):
		if len(raw)%2 != 0 {
			return "", false
		}
		order := binary.ByteOrder(binary.BigEndian)
		if raw[0] == 0xFF {
			order = binary.LittleEndian
		}
		units := make([]uint16, 0, len(raw)/2-1)
		for i := 2; i < len(raw); i += 2 {
			units = append(units, order.Uint16(raw[i:]))
		}
		return string(utf16.Decode(units)), true
	default:
		raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
		return string(raw), utf8.Valid(raw)
	}
}

// readStyles returns the runs of the first styl box. A damaged modifier box gives no runs.
func readStyles(modifiers []byte) []styleRun {
	for offset := uint64(0); offset+boxHeaderSize <= uint64(len(modifiers)); {
		size := uint64(binary.BigEndian.Uint32(modifiers[offset:]))
		if size < boxHeaderSize {
			return nil
		}
		if string(modifiers[offset+4:offset+8]) != "styl" {
			offset += size
			continue
		}
		data := modifiers[offset+boxHeaderSize : min(offset+size, uint64(len(modifiers)))]
		if len(data) < styleCountSize {
			return nil
		}
		count := min(int(binary.BigEndian.Uint16(data)), (len(data)-styleCountSize)/styleRecord)
		runs := make([]styleRun, count)
		for i := range runs {
			r := data[styleCountSize+i*styleRecord:]
			runs[i] = styleRun{start: int(binary.BigEndian.Uint16(r)), end: int(binary.BigEndian.Uint16(r[2:])), flags: r[6]}
		}
		return runs
	}
	return nil
}

// addStyleTags escapes each character of text and wraps each run in b, i and u tags.
// The start and the end of a run count characters, and the end is exclusive.
func addStyleTags(text string, runs []styleRun) string {
	chars := make([]string, 0, len(text))
	for _, r := range text {
		chars = append(chars, markupcore.EscapeText(string(r)))
	}
	slices.SortStableFunc(runs, func(a, b styleRun) int { return cmp.Compare(a.start, b.start) })
	covered := 0
	for _, r := range runs {
		start, end := max(r.start, covered), min(r.end, len(chars))
		var open, closing string
		for _, face := range [...]struct {
			flag byte
			tag  string
		}{{0x01, "b"}, {0x02, "i"}, {0x04, "u"}} {
			if r.flags&face.flag != 0 {
				open += "<" + face.tag + ">"
				closing = "</" + face.tag + ">" + closing
			}
		}
		if start >= end || open == "" {
			continue
		}
		chars[start] = open + chars[start]
		chars[end-1] += closing
		covered = end
	}
	return strings.Join(chars, "")
}
