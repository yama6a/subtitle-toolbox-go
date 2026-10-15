package model

import (
	"strconv"
	"time"
)

// CommonReadOptions holds the read options that every format package reads.
type CommonReadOptions struct {
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	// A lenient read with warnings returns a nil error.
	Lenient bool
	// LastCueDuration is the length of a last cue without an end. Nil means 5 s.
	LastCueDuration *time.Duration
}

// CommonWriteOptions holds the write options that every format package reads.
type CommonWriteOptions struct {
	// LineEnding is the line ending of the output. The zero value is LF.
	LineEnding LineEnding
	// BOM says whether the output starts with a UTF-8 BOM. The zero value keeps the format default.
	BOM BOM
	// StripTags writes the text without markup. False keeps the markup that the format can hold.
	StripTags bool
}

// LineEnding is the line ending that a writer uses.
type LineEnding uint8

// The line endings. LF is the zero value.
const (
	LF LineEnding = iota
	CRLF
)

// String returns "lf" or "crlf".
func (l LineEnding) String() string {
	switch l {
	case LF:
		return "lf"
	case CRLF:
		return "crlf"
	}
	return "LineEnding(" + strconv.Itoa(int(l)) + ")"
}

// BOM says whether a writer starts its output with a UTF-8 BOM.
type BOM uint8

// The BOM rules. BOMDefault is the zero value and keeps the default of the format.
const (
	BOMDefault BOM = iota
	BOMAdd
	BOMRemove
)

// String returns "default", "add" or "remove".
func (b BOM) String() string {
	switch b {
	case BOMDefault:
		return "default"
	case BOMAdd:
		return "add"
	case BOMRemove:
		return "remove"
	}
	return "BOM(" + strconv.Itoa(int(b)) + ")"
}
