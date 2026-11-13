package dvdsub

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/bitmap"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	substreamFirst = 0x20

	mpegStartCode  = "\x00\x00\x01"
	packStart      = 0xBA
	privateStream1 = 0xBD
	firstStream    = 0xBB

	// An MPEG-2 pack header has 14 bytes and up to 7 stuffing bytes. The byte after its start code begins with the bits 01.
	packHeaderSize     = 14
	packStuffingOffset = 13
	packStuffingMask   = 0x07
	mpeg2MarkerMask    = 0xC0
	mpeg2Marker        = 0x40

	// A PES packet starts with the start code, the stream code and the 2-byte packet length.
	pesHeaderSize     = 6
	pesHeaderDataSize = 8
	pesPayloadOffset  = 9
)

// ReadUnit joins the private stream 1 payloads of the sub-stream of track from filePos on, until they hold one whole
// subpicture unit, and returns the unit. The PES time stamps are not read. Every error is a *model.ParseError.
func ReadUnit(sub []byte, filePos int64, track int) ([]byte, error) {
	var unit []byte
	unitSize := -1
	position := int(min(filePos, int64(len(sub))))
	for unitSize < 0 || len(unit) < unitSize {
		code, err := streamCode(sub, position, filePos)
		if err != nil {
			return nil, err
		}
		if code == packStart {
			if position, err = skipPack(sub, position); err != nil {
				return nil, err
			}
			continue
		}
		end, err := packetEnd(sub, position, code, filePos)
		if err != nil {
			return nil, err
		}
		if code == privateStream1 {
			unit = appendPayload(unit, sub, position, end, track)
		}
		position = end
		if unitSize < 0 && len(unit) >= 2 {
			unitSize = int(binary.BigEndian.Uint16(unit))
		}
	}
	return unit[:unitSize], nil
}

// streamCode returns the stream code of the packet at position, which needs a start code and 6 header bytes.
func streamCode(sub []byte, position int, filePos int64) (byte, error) {
	if position+pesHeaderSize > len(sub) || string(sub[position:position+3]) != mpegStartCode {
		return 0, parseError(fmt.Sprintf("The .sub content has no complete subtitle packet at filepos %09x. "+
			"It ends or has no MPEG start code at byte %d.", filePos, position))
	}
	return sub[position+3], nil
}

// skipPack returns the position after the pack header at position.
func skipPack(sub []byte, position int) (int, error) {
	if position+packHeaderSize > len(sub) || sub[position+4]&mpeg2MarkerMask != mpeg2Marker {
		return 0, parseError("The .sub content has no MPEG-2 pack header at byte " + strconv.Itoa(position) + ".")
	}
	return position + packHeaderSize + int(sub[position+packStuffingOffset]&packStuffingMask), nil
}

// packetEnd returns the end of the PES packet at position. A stream code below the first PES stream code, such as
// the program end, is an error, because the unit is not complete.
func packetEnd(sub []byte, position int, code byte, filePos int64) (int, error) {
	if code < firstStream {
		return 0, parseError(fmt.Sprintf("The .sub content ends at byte %d before the subtitle packet at filepos %09x is complete.",
			position, filePos))
	}
	end := position + pesHeaderSize + int(binary.BigEndian.Uint16(sub[position+4:]))
	if end > len(sub) {
		return 0, parseError("The .sub packet at byte " + strconv.Itoa(position) + " is longer than the content.")
	}
	return end, nil
}

// appendPayload appends the payload of the private stream 1 packet from position to end to unit, when its
// sub-stream is the one of track.
func appendPayload(unit, sub []byte, position, end, track int) []byte {
	var headerData int
	if position+pesHeaderDataSize < len(sub) {
		headerData = int(sub[position+pesHeaderDataSize])
	}
	if start := position + pesPayloadOffset + headerData; start < end && int(sub[start]) == substreamFirst+track {
		unit = append(unit, sub[start+1:end]...)
	}
	return unit
}

// Unit is one decoded subpicture unit.
type Unit struct {
	// StartDelay is the delay of the start command after the time of the index entry. It is 0 without a start command.
	StartDelay time.Duration
	// StopDelay is the delay of the stop command after the time of the index entry. It holds no time when HasStop is false.
	StopDelay time.Duration
	// HasStop reports that the unit has a stop command.
	HasStop bool
	// Forced reports that the unit starts with the forced start command.
	Forced bool
	// Image is the picture on the screen of the index, nil for a unit that only clears the screen.
	Image *model.Image
}

// The commands of a control sequence.
const (
	cmdForcedStart = 0x00
	cmdStart       = 0x01
	cmdStop        = 0x02
	cmdColor       = 0x03
	cmdContrast    = 0x04
	cmdArea        = 0x05
	cmdFields      = 0x06
	cmdColorChange = 0x07
	cmdEnd         = 0xFF

	// contrastToAlpha turns a contrast from 0 to 15 into an alpha from 0 to 255.
	contrastToAlpha = 17
)

// decoder holds the state of the control sequences of one unit.
type decoder struct {
	unit      []byte
	started   bool
	start     time.Duration
	stopped   bool
	stop      time.Duration
	forced    bool
	colors    [4]int
	contrasts [4]int
	area      *[4]int
	fields    *[2]int
}

// DecodeUnit runs the control sequences of unit and decodes its bitmap with the colours of ix. Every error is a
// *model.ParseError.
func DecodeUnit(unit []byte, ix Index) (Unit, error) {
	d := decoder{unit: unit}
	if err := d.runSequences(); err != nil {
		return Unit{}, err
	}
	u := Unit{StartDelay: d.start, StopDelay: d.stop, HasStop: d.stopped, Forced: d.forced}
	if d.area == nil || d.fields == nil || d.area[1] < d.area[0] || d.area[3] < d.area[2] {
		return u, nil
	}
	var colors [4]uint32
	if ix.Custom != nil {
		colors = *ix.Custom
	} else {
		for i := range colors {
			colors[i] = ix.Palette[d.colors[i]]<<8 | uint32(d.contrasts[i]*contrastToAlpha)
		}
	}
	var err error
	u.Image, err = d.decodeImage(colors, ix)
	return u, err
}

func (d *decoder) runSequences() error {
	size := len(d.unit)
	var sequence int
	if size >= 4 {
		sequence = int(binary.BigEndian.Uint16(d.unit[2:]))
	}
	if sequence < 4 || sequence+4 > size {
		return parseError("The subtitle packet of " + strconv.Itoa(size) + " bytes has no valid control sequence offset.")
	}
	for {
		delay := int64(binary.BigEndian.Uint16(d.unit[sequence:]))
		next := int(binary.BigEndian.Uint16(d.unit[sequence+2:]))
		if err := d.runSequence(sequence+4, delayTime(delay)); err != nil {
			return err
		}
		if next <= sequence || next+4 > size {
			return nil
		}
		sequence = next
	}
}

// delayTime returns units of 1024 ticks of the 90 kHz clock as whole milliseconds, half up.
func delayTime(units int64) time.Duration {
	return time.Duration((units*1024+45)/90) * time.Millisecond
}

// runSequence runs the commands of one control sequence from position on. The commands that set the picture count
// only until a sequence has started the display. Later sequences change colours or the area over time, which one
// image cannot hold.
func (d *decoder) runSequence(position int, delay time.Duration) error {
	size := len(d.unit)
	shown := d.started
	for position < size {
		command := d.unit[position]
		position++
		if command == cmdEnd || command > cmdColorChange {
			return nil
		}
		argSize := argumentSize(command)
		if position+argSize > size {
			return parseError(fmt.Sprintf("The subtitle packet ends inside command %02x.", command))
		}
		if command == cmdColorChange {
			argSize = size
			if position+2 <= size {
				argSize = int(binary.BigEndian.Uint16(d.unit[position:]))
			}
		} else {
			d.apply(command, d.unit[position:position+argSize], delay, shown)
		}
		position += argSize
	}
	return nil
}

// apply runs a command of 0 to 6 with its arguments. The first start command and the first stop command set the
// times.
func (d *decoder) apply(command byte, args []byte, delay time.Duration, shown bool) {
	switch command {
	case cmdForcedStart, cmdStart:
		if !d.started {
			d.started, d.start = true, delay
		}
		d.forced = d.forced || command == cmdForcedStart
	case cmdStop:
		if !d.stopped {
			d.stopped, d.stop = true, delay
		}
	default:
		if !shown {
			d.setDisplay(command, args)
		}
	}
}

func argumentSize(command byte) int {
	switch command {
	case cmdColor, cmdContrast:
		return 2
	case cmdArea:
		return 6
	case cmdFields:
		return 4
	default:
		return 0
	}
}

// setDisplay runs a command that sets the colours, the contrast, the area or the field offsets.
func (d *decoder) setDisplay(command byte, a []byte) {
	switch command {
	case cmdColor:
		d.colors = nibbles(a)
	case cmdContrast:
		d.contrasts = nibbles(a)
	case cmdArea:
		d.area = &[4]int{
			int(a[0])<<4 | int(a[1])>>4,
			int(a[1]&0x0F)<<8 | int(a[2]),
			int(a[3])<<4 | int(a[4])>>4,
			int(a[4]&0x0F)<<8 | int(a[5]),
		}
	case cmdFields:
		d.fields = &[2]int{int(a[0])<<8 | int(a[1]), int(a[2])<<8 | int(a[3])}
	}
}

// nibbles returns the 4 nibbles of a colour or contrast argument by pixel value: the background, the pattern and the
// 2 emphasis pixels.
func nibbles(a []byte) [4]int {
	return [4]int{int(a[1] & 0x0F), int(a[1] >> 4), int(a[0] & 0x0F), int(a[0] >> 4)}
}

// decodeImage decodes the 2-bit run-length picture. The top field holds the even rows and the bottom field the odd rows.
func (d *decoder) decodeImage(colors [4]uint32, ix Index) (*model.Image, error) {
	x, y := d.area[0], d.area[2]
	width, height := d.area[1]-x+1, d.area[3]-y+1
	if width > model.MaxImageSide || height > model.MaxImageSide || width*height > model.MaxImagePixels {
		return nil, parseError("Cannot read the subtitle packet: an image of " + strconv.Itoa(width) + "x" + strconv.Itoa(height) +
			" pixels is larger than the limit of " + strconv.Itoa(model.MaxImageSide) + " pixels per side and " +
			strconv.Itoa(model.MaxImagePixels) + " pixels in total.")
	}
	pixels := make([]uint32, width*height)
	for i := range pixels {
		pixels[i] = colors[0]
	}
	for field, offset := range d.fields {
		nibble := offset * 2
		for row := field; row < height; row += 2 {
			if err := d.decodeRow(&nibble, row, pixels[row*width:(row+1)*width], colors); err != nil {
				return nil, err
			}
			nibble += nibble & 1
		}
	}
	return &model.Image{
		PNG:          bitmap.EncodeRGBA(bitmap.Pixels{Width: width, Height: height, RGBA: pixels}),
		X:            x,
		Y:            y,
		Width:        width,
		Height:       height,
		ScreenWidth:  ix.Width,
		ScreenHeight: ix.Height,
	}, nil
}

func (d *decoder) decodeRow(nibble *int, row int, pixels []uint32, colors [4]uint32) error {
	for column := 0; column < len(pixels); {
		code, err := d.readCode(nibble, row)
		if err != nil {
			return err
		}
		run := len(pixels) - column
		if code>>2 != 0 {
			run = min(code>>2, run)
		}
		for i := column; i < column+run; i++ {
			pixels[i] = colors[code&3]
		}
		column += run
	}
	return nil
}

// readCode reads one run-length code of 1 to 4 nibbles.
func (d *decoder) readCode(nibble *int, row int) (int, error) {
	code := 0
	for _, limit := range [...]int{0x4, 0x10, 0x40, 0x100} {
		if *nibble >= len(d.unit)*2 {
			return 0, parseError("The bitmap of the subtitle packet ends before line " + strconv.Itoa(row) + " is complete.")
		}
		b := int(d.unit[*nibble>>1])
		if *nibble&1 == 0 {
			b >>= 4
		}
		code = code<<4 | b&0x0F
		*nibble++
		if code >= limit {
			break
		}
	}
	return code, nil
}
