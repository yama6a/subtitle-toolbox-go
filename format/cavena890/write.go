package cavena890

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Write writes s as Cavena 890. A new file gets the Latin table and the header of Subtitle Edit. A file that Read
// returned keeps its variant and header, and a cue with its text unchanged keeps its blocks. A new cue gets the next
// number after the one before it, 16 more while the result stays at or below 65535, else 1 more. When 65535 is used
// up, the write fails with a *model.UnwritableError.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.Cavena890, common)
	if err != nil {
		return WriteReport{}, err
	}
	file, _ := s.FormatData(formatDataKey)
	v := latin
	if stored, _ := file.String("variant"); variant(stored).known() {
		v = variant(stored)
	}
	header := storedHeader(file)

	var blocks [][]byte
	number := 0
	for i, c := range s.All() {
		cueBlocks, err := blocksOf(c, i, v, common.StripTags)
		if err != nil {
			return WriteReport{}, err
		}
		if number, err = nextNumber(number, int(binary.BigEndian.Uint16(cueBlocks[0][numberAt:])), i); err != nil {
			return WriteReport{}, err
		}
		for _, block := range cueBlocks {
			binary.BigEndian.PutUint16(block[numberAt:], uint16(number))
			blocks = append(blocks, block)
		}
	}

	return WriteReport{}, writeFile(w, header, blocks, file)
}

// writeFile writes the header with the block count, then the blocks. It cuts the last 4 bytes when the file has
// blocks and the stored shortEnd is true or missing.
func writeFile(w io.Writer, header []byte, blocks [][]byte, file *model.Data) error {
	for _, at := range counts {
		binary.LittleEndian.PutUint16(header[at:], uint16(min(len(blocks), maxNumber)))
	}
	out := slices.Concat(header, bytes.Join(blocks, nil))
	if shortEnd, ok := file.Bool("shortEnd"); len(blocks) > 0 && (shortEnd || !ok) {
		out = out[:len(out)-blockEnd]
	}
	return writing.WriteBytes(w, model.Cavena890, out)
}

// storedHeader returns the stored header when it decodes to 388 bytes, else the header of a new file.
func storedHeader(file *model.Data) []byte {
	stored, _ := file.String("header")
	if b, err := hex.DecodeString(stored); err == nil && len(b) == headerSize {
		return b
	}
	return newHeader()
}

// nextNumber returns the number of the cue at cueIndex. A stored number stays when it is greater than previous. Else
// it is previous plus 16, or plus 1 when that passes 65535.
func nextNumber(previous, stored, cueIndex int) (int, error) {
	switch {
	case stored > previous:
		return stored, nil
	case previous+numberStep <= maxNumber:
		return previous + numberStep, nil
	case previous+1 <= maxNumber:
		return previous + 1, nil
	}
	return 0, &model.UnwritableError{Format: model.Cavena890, CueIndex: cueIndex, Reason: "Cavena 890 allows subtitle numbers up to 65535."}
}

// blocksOf returns the stored blocks of an unchanged cue with the new times and alignment, or new blocks of 2 lines
// each. A new block has the number 0. A last single line goes into the lower line, and a cue without lines gives one
// empty block.
func blocksOf(c model.Cue, cueIndex int, v variant, stripTags bool) ([][]byte, error) {
	times := slices.Concat(frameBytes(c.Start), frameBytes(c.End))
	code := alignmentCode(c.Position.Alignment)
	if stored := storedBlocks(c); stored != nil && !stripTags {
		if alignmentOf(stored[0][alignmentAt]) == alignmentOf(code) {
			code = stored[0][alignmentAt]
		}
		for _, block := range stored {
			copy(block[startAt:], append(times, code))
		}
		return stored, nil
	}

	lines := make([]string, len(c.Lines))
	for i, line := range c.Lines {
		lines[i] = markupcore.RubyAsText(line)
	}
	lines = markupcore.CarryOpenTags(lines)
	if len(lines) == 0 {
		lines = []string{""}
	}
	return newBlocks(lines, slices.Concat(times, []byte{code}), v, stripTags, cueIndex)
}

// newBlocks returns blocks of 2 lines each. timesAndAlignment is the 7 bytes from offset 2 of a block.
func newBlocks(lines []string, timesAndAlignment []byte, v variant, stripTags bool, cueIndex int) ([][]byte, error) {
	var blocks [][]byte
	for pair := range slices.Chunk(lines, 2) {
		upper, lower := "", pair[0]
		if len(pair) == 2 {
			upper, lower = pair[0], pair[1]
		}
		upperBytes, err := v.encode(upper, stripTags, cueIndex)
		if err != nil {
			return nil, err
		}
		lowerBytes, err := v.encode(lower, stripTags, cueIndex)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, slices.Concat([]byte{0, 0}, timesAndAlignment, make([]byte, 7), upperBytes, make([]byte, 6), lowerBytes, blockEndBytes))
	}
	return blocks, nil
}

func alignmentCode(a model.Alignment) byte {
	switch a.Column() {
	case 1:
		return alignLeft
	case 3:
		return alignRight
	default:
		return alignCenter
	}
}

// storedBlocks returns the blocks that Read stored in c when the text of c is the text at read time. It returns nil
// when the data is missing, or when a value is not the shape that Read writes.
func storedBlocks(c model.Cue) [][]byte {
	data, _ := c.FormatData(formatDataKey)
	text, hasText := data.String("text")
	hexes, hasBlocks := data.Strings("blocks")
	if !hasText || !hasBlocks || len(hexes) == 0 || text != c.Text() {
		return nil
	}
	blocks := make([][]byte, len(hexes))
	for i, h := range hexes {
		b, err := hex.DecodeString(h)
		if err != nil || len(b) != blockSize {
			return nil
		}
		blocks[i] = b
	}
	return blocks
}

// frameBytes returns the frame number of d at 25 fps as 3 big-endian bytes, capped at FFFFFFh.
func frameBytes(d time.Duration) []byte {
	frame := min(model.FrameRate25.Frames(max(d, 0)), maxFrame)
	return []byte{byte(frame >> 16), byte(frame >> 8), byte(frame)}
}
