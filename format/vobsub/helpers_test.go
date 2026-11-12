package vobsub_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const ms = time.Millisecond

// The commands of a control sequence.
const (
	forcedStart = 0x00
	start       = 0x01
	stop        = 0x02
	colorChange = 0x07
)

// sequence is one control sequence. delay counts in units of 1024 ticks of 90 kHz. params adds the commands that
// set the colours, the contrast, the area and the field offsets, before commands.
type sequence struct {
	delay    int
	params   bool
	commands []byte
}

// picture is a unit as the writer below packs it. rows hold the pixel values 0 to 3.
type picture struct {
	rows      [][]byte
	x, y      int
	colors    [4]int
	contrasts [4]int
	sequences []sequence
	// bottomFirst stores the bottom field before the top field.
	bottomFirst bool
}

// runLengthRow encodes a row as run-length codes of 1 to 4 nibbles, padded to a whole byte.
func runLengthRow(row []byte) []byte {
	var nibbles []byte
	for i := 0; i < len(row); {
		j := i
		for j < len(row) && row[j] == row[i] && j-i < 255 {
			j++
		}
		if j == len(row) && j-i >= 64 {
			nibbles = append(nibbles, 0, 0, 0, row[i])
			break
		}
		run, code := j-i, (j-i)<<2|int(row[i])
		digits := 4
		switch {
		case run < 4:
			digits = 1
		case run < 16:
			digits = 2
		case run < 64:
			digits = 3
		}
		for d := digits - 1; d >= 0; d-- {
			nibbles = append(nibbles, byte(code>>(4*d)&0x0F))
		}
		i = j
	}
	if len(nibbles)%2 == 1 {
		nibbles = append(nibbles, 0)
	}
	var out []byte
	for i := 0; i < len(nibbles); i += 2 {
		out = append(out, nibbles[i]<<4|nibbles[i+1])
	}
	return out
}

func u16(values ...int) []byte {
	var out []byte
	for _, v := range values {
		out = binary.BigEndian.AppendUint16(out, uint16(v))
	}
	return out
}

// unit returns the subpicture unit of p.
func (p picture) unit() []byte {
	var top, bottom []byte
	for i, row := range p.rows {
		if i%2 == 0 {
			top = append(top, runLengthRow(row)...)
		} else {
			bottom = append(bottom, runLengthRow(row)...)
		}
	}
	x2, y2 := p.x+len(p.rows[0])-1, p.y+len(p.rows)-1
	params := []byte{0x03, byte(p.colors[3]<<4 | p.colors[2]), byte(p.colors[1]<<4 | p.colors[0])}
	params = append(params, 0x04, byte(p.contrasts[3]<<4|p.contrasts[2]), byte(p.contrasts[1]<<4|p.contrasts[0]))
	params = append(params, 0x05, byte(p.x>>4), byte(p.x&0x0F<<4|x2>>8), byte(x2), byte(p.y>>4), byte(p.y&0x0F<<4|y2>>8), byte(y2))
	params = append(params, 0x06)
	pixels := slicesConcat(top, bottom)
	if p.bottomFirst {
		pixels = slicesConcat(bottom, top)
		params = append(params, u16(4+len(bottom), 4)...)
	} else {
		params = append(params, u16(4, 4+len(top))...)
	}

	offset := 4 + len(pixels)
	var control []byte
	for i, s := range p.sequences {
		var body []byte
		if s.params {
			body = append(body, params...)
		}
		body = append(append(body, s.commands...), 0xFF)
		next := offset
		if i < len(p.sequences)-1 {
			next += 4 + len(body)
		}
		control = append(control, u16(s.delay, next)...)
		control = append(control, body...)
		offset = next
	}
	unit := slicesConcat(pixels, control)
	return slicesConcat(u16(4+len(unit), 4+len(pixels)), unit)
}

func slicesConcat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

// packet is a unit for the writer of the .sub content.
type packet struct {
	unit  []byte
	track int
	// chunk is the largest payload of one PES packet. 0 puts the unit into one packet.
	chunk int
	// other puts a PES packet of the next sub-stream before each chunk.
	other bool
}

// packHeader is an MPEG-2 pack header of 14 bytes with no stuffing.
const packHeader = "\x00\x00\x01\xBA\x44\x00\x04\x00\x04\x01\x01\x89\xC3\xF8"

// pes returns a private stream 1 PES packet of the sub-stream with the payload.
func pes(substream int, payload []byte) []byte {
	body := slicesConcat([]byte{0x81, 0x00, 0x00, byte(substream)}, payload)
	return slicesConcat([]byte("\x00\x00\x01\xBD"), u16(len(body)), body)
}

// program returns the .sub content of the packets and the file position of each. Each packet starts with a pack
// header and a padding packet follows its last PES packet.
func program(packets ...packet) ([]byte, []int) {
	var sub []byte
	positions := make([]int, 0, len(packets))
	for _, p := range packets {
		positions = append(positions, len(sub))
		for rest := p.unit; len(rest) > 0; {
			n := len(rest)
			if p.chunk > 0 {
				n = min(n, p.chunk)
			}
			sub = append(sub, packHeader...)
			if p.other {
				sub = append(sub, pes(0x21+p.track, []byte("noise"))...)
			}
			sub = append(sub, pes(0x20+p.track, rest[:n])...)
			rest = rest[n:]
		}
		sub = append(sub, "\x00\x00\x01\xBE\x00\x02\xFF\xFF"...)
	}
	return sub, positions
}

// clock writes d as h:mm:ss:mmm of a timestamp line.
func clock(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d:%03d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60, d.Milliseconds()%1000)
}

// palette is the 16 colours that the tests use: entry i is the colour with the red part 16 * i.
const palette = "000000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, a00000, b00000, c00000, d00000, e00000, f00000"

// index returns an .idx file with the lines before the first track and the given lines after them.
func index(head string, lines ...string) []byte {
	return []byte("# VobSub index file, v7 (do not modify this line!)\nsize: 720x576\n" + head + strings.Join(lines, "\n") + "\n")
}

// idxFor returns the .idx file of one track with the units at the given times.
func idxFor(track int, times []time.Duration, positions []int) []byte {
	lines := make([]string, 0, 1+len(times))
	lines = append(lines, fmt.Sprintf("id: en, index: %d", track))
	for i, at := range times {
		lines = append(lines, fmt.Sprintf("timestamp: %s, filepos: %09x", clock(at), positions[i]))
	}
	return index("palette: "+palette+"\n", lines...)
}

type cue struct {
	Start, End time.Duration
	Forced     bool
	X, Y, W, H int
}

func cuesOf(s *model.Subtitle) []cue {
	var out []cue
	for _, c := range s.All() {
		out = append(out, cue{c.Start, c.End, c.Forced, c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height})
	}
	return out
}

func read(t *testing.T, sub, idx []byte, opts vobsub.ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	opts.IDX = idx
	return vobsub.Read(bytes.NewReader(sub), model.CommonReadOptions{}, opts)
}

func mustRead(t *testing.T, sub, idx []byte) *model.Subtitle {
	t.Helper()

	s, _, err := read(t, sub, idx, vobsub.ReadOptions{})
	require.NoError(t, err)
	return s
}

// pixel returns the colour of the pixel at x, y of the PNG of a cue as 0xRRGGBBAA.
func pixel(t *testing.T, c model.Cue, x, y int) uint32 {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(c.Image.PNG))
	require.NoError(t, err)
	px := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return uint32(px.R)<<24 | uint32(px.G)<<16 | uint32(px.B)<<8 | uint32(px.A)
}

func hex9(n int) string {
	return fmt.Sprintf("%09x", n)
}

// rawUnit returns a unit of one control sequence with the commands. The command FF ends the sequence.
func rawUnit(commands ...byte) []byte {
	body := slicesConcat(u16(0, 0), commands, []byte{0xFF})
	return slicesConcat(u16(4+len(body), 4), body)
}
