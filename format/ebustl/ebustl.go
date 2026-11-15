// Package ebustl reads EBU STL, the binary subtitle exchange format of EBU Tech 3264.
//
// A file holds one GSI block of 1024 bytes, then TTI blocks of 128 bytes. A subtitle takes one or more TTI blocks.
package ebustl

import (
	"bytes"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.EBUSTL)

// ReadOptions holds the settings of the EBU STL reader.
type ReadOptions struct {
	// SubtractStartOfProgramme subtracts the GSI start of programme (TCP) from every cue time.
	// A time before it becomes 0.
	SubtractStartOfProgramme bool
}

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

// Detect reports whether content starts with 3 ASCII digits and STL25.01 or STL30.01. It reads only the first
// 11 bytes, so the BOM and line ending steps of stbox.Detect do not change the result.
func Detect(content []byte) bool {
	if len(content) < 11 {
		return false
	}
	for _, c := range content[:3] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return bytes.HasPrefix(content[3:], []byte("STL25.01")) || bytes.HasPrefix(content[3:], []byte("STL30.01"))
}
