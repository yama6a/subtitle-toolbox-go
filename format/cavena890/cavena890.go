// Package cavena890 reads and writes Cavena 890, the binary subtitle file of the Cavena systems that Scandinavian
// broadcasters use.
//
// A file holds a header of 388 bytes, then blocks of 128 bytes. Each block holds the subtitle number, the start and
// end frame at 25 fps, the alignment and 2 text lines of 51 bytes. The last block may lack its last 4 bytes. The
// format has no public spec and no signature, so a file loads by its extension .890.
//
// The language codes and the font names of the header name the text table, called the variant. The reader keeps the
// header, the variant and the blocks of each cue in the cavena890 format data, so a round trip keeps them.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
package cavena890

import (
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.Cavena890)

const (
	headerSize = 388
	blockSize  = 128
	textSize   = 51
	blockEnd   = 4
)

// Offsets in the header.
const (
	language1 = 146
	language2 = 147
	font1     = 187
	font2     = 246
	fontSize  = 6
)

// counts are the header offsets of the block count, a uint16 in little-endian order.
//
//nolint:gochecknoglobals // fixed header offsets
var counts = [...]int{179, 208, 210}

// Offsets in a block.
const (
	numberAt    = 0
	startAt     = 2
	endAt       = 5
	alignmentAt = 8
	line1At     = 16
	line2At     = 73
)

const (
	alignLeft   = 0x50
	alignRight  = 0x52
	alignCenter = 0x54
)

const (
	filler      = 0x7F
	dash        = 0xBE
	italicOn    = 0x88
	italicOff   = 0x98
	firstColor  = 0xF1
	lastColor   = 0xF8
	english     = 0x09
	arabicCode  = 0x80
	maxNumber   = 0xFFFF
	numberStep  = 16
	maxFrame    = 0xFFFFFF
	newFontName = "HLV23N.V"
)

// ReadOptions holds the settings of the Cavena 890 reader. It has none.
type ReadOptions struct{}

// WriteOptions holds the settings of the Cavena 890 writer. It has none.
type WriteOptions struct{}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

func alignmentOf(code byte) model.Alignment {
	switch code {
	case alignLeft:
		return model.BottomLeft
	case alignRight:
		return model.BottomRight
	default:
		return model.BottomCenter
	}
}

// newHeader returns the header that Subtitle Edit writes for a new file.
//
// Copied from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
func newHeader() []byte {
	zeros := func(n int) string { return strings.Repeat("\x00", n) }
	font := zeros(14-len(newFontName)) + newFontName
	h := zeros(134) + "\xA0\x05\x04\x03\x06\x06\x08\x90\x00\x00\x00\x00" + string([]byte{english, english}) +
		zeros(24) + "\x08\x90\x00\x00\x00\x00\x00" + "\x00\x00" + font + zeros(13) +
		zeros(4) + zeros(34) + newFontName + "\x3D\x8D" + "10:00:00:00" + zeros(26) + "BTD" + zeros(92)
	return []byte(h)
}
