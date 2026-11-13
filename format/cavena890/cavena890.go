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
	"bytes"
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

// variant is the text table of a file. Its value is the variant field of the cavena890 format data.
type variant string

const (
	latin        variant = "latin"
	scandinavian variant = "scandinavian"
	hebrew       variant = "hebrew"
	russian      variant = "russian"
	greek        variant = "greek"
	arabic       variant = "arabic"
	chinese      variant = "chinese"
)

// languages maps the language code of line 1 to a variant. The Arabic code counts for line 2.
//
//nolint:gochecknoglobals // lookup table of the language codes of Subtitle Edit
var languages = map[byte]variant{
	0x07: scandinavian, 0x1E: scandinavian, 0x28: scandinavian,
	0x8F: hebrew, 0x70: greek, 0x56: russian, 0x90: chinese, 0x91: chinese,
}

// fontPrefixes holds the font name prefix that also names a variant.
//
//nolint:gochecknoglobals // lookup table of the font names of Subtitle Edit
var fontPrefixes = map[variant]string{hebrew: "HEBNOA", greek: "GREEK", russian: "KYRIL", chinese: "CCKM44"}

// variantOf returns the variant of the language codes and the font names of header, in the order of Subtitle Edit.
func variantOf(header []byte) variant {
	language := languages[header[language1]]
	for _, v := range [...]variant{hebrew, greek, arabic, russian, chinese, scandinavian} {
		if names(header, language, v) {
			return v
		}
	}
	return latin
}

// names reports whether the language code of line 1 or a font name of header names v.
func names(header []byte, language, v variant) bool {
	fonts := [2][]byte{header[font1 : font1+fontSize], header[font2 : font2+fontSize]}
	if v == arabic {
		return header[language2] == arabicCode || string(fonts[0]) == "ARABIC"
	}
	prefix := []byte(fontPrefixes[v])
	return language == v || len(prefix) > 0 && (bytes.HasPrefix(fonts[0], prefix) || bytes.HasPrefix(fonts[1], prefix))
}

func (v variant) known() bool {
	switch v {
	case latin, scandinavian, hebrew, russian, greek, arabic, chinese:
		return true
	}
	return false
}

func (v variant) hasItalics() bool {
	return v == latin || v == scandinavian || v == chinese
}

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
