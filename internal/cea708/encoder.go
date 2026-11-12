package cea708

import (
	"maps"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
)

const (
	// A service block holds at most 31 bytes.
	maxBlock = 31
	// The longest command is DFn with its 6 parameters.
	minCapacity = 7
	// A 16:9 window holds 42 columns. A window definition holds at most 16 rows and 64 columns.
	columns    = 42
	maxRows    = 16
	maxColumns = 64
	// Window style 1 and pen style 1, the predefined styles of a pop-on caption.
	styleBits       = 0x09
	transparentFill = 0xC0
	solidBlack      = 0x00
)

// anchorPercent is the percent of the screen for the top, middle and bottom row and for the left, center and
// right column of an anchor point.
var anchorPercent = [3]byte{10, 50, 90} //nolint:gochecknoglobals // constant table

// justify holds the justify bits of the window attributes for the left, center and right column.
var justify = [3]byte{0x0C, 0x0E, 0x0D} //nolint:gochecknoglobals // constant table

// Caption is one cue for the encoder.
type Caption struct {
	Start, End  time.Duration
	Lines       []string // core markup
	AnchorPoint int      // 0 to 8
}

// Block is the service 1 data of one frame.
type Block struct {
	Frame int64
	Data  []byte
}

type styledLine []cea608.Char

type encoder struct {
	frames   map[int64][][]byte
	capacity int
}

// Blocks encodes the captions as service 1 blocks. Each caption loads into a hidden window, which shows at its
// start. Windows 0 and 1 take turns. frameOf turns a time into a frame number of the output rate.
// A block holds whole commands and at most min(capacity, 31) bytes. A capacity below 7 counts as 7.
func Blocks(captions []Caption, frameOf func(time.Duration) int64, capacity int) []Block {
	e := &encoder{frames: map[int64][][]byte{}, capacity: max(min(capacity, maxBlock), minCapacity)}
	type shown struct {
		window int
		end    int64
	}
	var previous *shown
	lastShow := int64(math.MinInt64)
	index := 0
	for _, c := range captions {
		lines := styledLines(c.Lines, false)
		if len(lines) == 0 {
			continue
		}
		window := index % 2
		index++
		start := frameOf(c.Start)
		if previous != nil && previous.end < start {
			e.place([][]byte{command(cmdDLW, 1<<previous.window)}, previous.end)
			previous = nil
		}
		start = e.placeBefore(load(window, lines, c.AnchorPoint), start, lastShow)
		show := command(cmdDSW, 1<<window)
		if previous != nil {
			show = append(command(cmdDLW, 1<<previous.window), show...)
		}
		start = e.place([][]byte{show}, start)
		previous = &shown{window: window, end: max(start+1, frameOf(c.End))}
		lastShow = start
	}
	if previous != nil {
		e.place([][]byte{command(cmdDLW, 1<<previous.window)}, previous.end)
	}

	blocks := make([]Block, 0, len(e.frames))
	for _, frame := range slices.Sorted(maps.Keys(e.frames)) {
		var data []byte
		for _, c := range e.frames[frame] {
			data = append(data, c...)
		}
		blocks = append(blocks, Block{Frame: frame, Data: data})
	}
	return blocks
}

// styledLines splits the caption text into styled characters. A line over 42 characters wraps once.
// A window holds at most 16 rows, so the lines after the 16th drop.
func styledLines(text []string, wrapped bool) []styledLine {
	plain := make([]string, len(text))
	for i, l := range text {
		plain[i] = markupcore.RubyAsText(l)
	}
	var lines []styledLine
	for _, l := range markupcore.CarryOpenTags(plain) {
		chars := cea608.StyledCharacters(l)
		if len(chars) > columns && !wrapped {
			return styledLines(linewrap.Wrap(plain, columns, maxRows, false), true)
		}
		if len(chars) > 0 {
			lines = append(lines, chars[:min(len(chars), maxColumns)])
		}
	}
	return lines[:min(len(lines), maxRows)]
}

// load returns the commands that define the window hidden and write the lines into it.
func load(window int, lines []styledLine, point int) [][]byte {
	width := 0
	for _, l := range lines {
		width = max(width, len(l))
	}
	commands := [][]byte{
		command(cmdDLW, 1<<window),
		command(byte(cmdDF0+window), 0x18, 0x80|anchorPercent[point/3], anchorPercent[point%3],
			byte(point<<4|(len(lines)-1)), byte(width-1), styleBits),
		command(cmdSWA, transparentFill, 0x00, justify[point%3], 0x00),
	}
	var current *cea608.Char
	for row, l := range lines {
		commands = append(commands, command(cmdSPL, byte(row), 0))
		for _, c := range l {
			if c.R != ' ' && (current == nil || !sameStyle(*current, c)) {
				var flags byte
				if c.Italic {
					flags |= 0x80
				}
				if c.Underline {
					flags |= 0x40
				}
				commands = append(commands, command(cmdSPA, 0x01, flags), command(cmdSPC, color(c.Color), solidBlack, 0x00))
				current = &c
			}
			commands = append(commands, character(c.R))
		}
	}
	return commands
}

func sameStyle(a, b cea608.Char) bool {
	return a.Color == b.Color && a.Italic == b.Italic && a.Underline == b.Underline
}

// color returns the 6-bit color of a CEA-608 color index: 2 bits each of red, green and blue.
func color(index int) byte {
	hex := cea608.Color(index)
	var bits byte
	for i := 1; i < 7; i += 2 {
		channel, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		bits = bits<<2 | byte(channel/85)
	}
	return bits
}

// character returns the bytes of a character: G0, the music note, G1, G2 after EXT1, or the CEA-608 replacement of
// a character that neither set holds.
func character(r rune) []byte {
	if b, ok := direct(r); ok {
		return b
	}
	to, ok := cea608.Transliterate(r)
	if !ok {
		return []byte{'?'}
	}
	var out []byte
	for _, t := range to {
		b, ok := direct(t)
		if !ok {
			b = []byte{'?'}
		}
		out = append(out, b...)
	}
	return out
}

// direct returns the bytes of a character that G0, G1 or G2 holds.
func direct(r rune) ([]byte, bool) {
	switch {
	case r >= 0x20 && r < 0x7F || r >= 0xA0 && r <= 0xFF:
		return []byte{byte(r)}, true
	case r == '\u266A':
		return []byte{musicNote}, true
	}
	if code, ok := g2Codes[r]; ok {
		return []byte{codeEXT1, code}, true
	}
	return nil, false
}

func command(code byte, parameters ...byte) []byte {
	return append([]byte{code}, parameters...)
}

// placeBefore puts the commands into the free bytes of the frames after `after` and before `before`, the last
// command as late as possible. It returns before. When the frames between are too few, the commands follow
// `after` in order, and it returns the frame after the last one.
func (e *encoder) placeBefore(commands [][]byte, before, after int64) int64 {
	assigned := make([]int64, len(commands))
	used := map[int64]int{}
	frame := before - 1
	for i := len(commands) - 1; i >= 0; i-- {
		for frame > after && frame >= 0 && e.free(frame)-used[frame] < len(commands[i]) {
			frame--
		}
		if frame <= after || frame < 0 {
			return e.place(commands, max(after+1, 0)) + 1
		}
		assigned[i] = frame
		used[frame] += len(commands[i])
	}
	for i, frame := range assigned {
		e.frames[frame] = append(e.frames[frame], commands[i])
	}
	return before
}

// place puts the commands into the free bytes of the frames from `from` on, in order.
// It returns the frame of the last command.
func (e *encoder) place(commands [][]byte, from int64) int64 {
	frame := from
	for _, c := range commands {
		for e.free(frame) < len(c) {
			frame++
		}
		e.frames[frame] = append(e.frames[frame], c)
	}
	return frame
}

func (e *encoder) free(frame int64) int {
	free := e.capacity
	for _, c := range e.frames[frame] {
		free -= len(c)
	}
	return free
}
