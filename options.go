package stbox

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// ReadOptions holds the settings of a read. The zero value reads with the defaults.
type ReadOptions struct {
	// Format is the format of the input. The zero value detects the format.
	Format Format
	// Encoding is the name of the source encoding. The zero value detects the encoding.
	Encoding string
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	// A lenient read with warnings returns a nil error.
	Lenient bool
	// LastCueDuration is the length of a last cue without an end. Nil means 5 s.
	LastCueDuration *time.Duration
}

// WriteOptions holds the settings of a write. The zero value writes with the defaults.
type WriteOptions struct {
	// LineEnding is the line ending of the output. The zero value is LF.
	LineEnding LineEnding
	// BOM says whether the output starts with a UTF-8 BOM. The zero value keeps the format default.
	BOM BOM
	// StripTags writes the text without markup. False keeps the markup that the format can hold.
	StripTags bool
}

// ReadInfo holds the facts of a read that are not subtitle content.
type ReadInfo struct {
	// Format is the format that the read used.
	Format Format
	// SourceEncoding is the encoding of the input, "" for a binary format.
	SourceEncoding string
	// Warnings holds every warning of the read, in read order. A strict read can hold warnings too,
	// for repairs that a reader makes in both modes.
	Warnings []ParseWarning
}

// Validate checks every field of o, also the fields of formats that a read does not use.
func (o ReadOptions) Validate() error {
	if o.LastCueDuration != nil && *o.LastCueDuration < 0 {
		return &InvalidArgumentError{
			Option: "ReadOptions.LastCueDuration",
			Msg:    fmt.Sprintf("The last cue duration must be 0 or more, got %s.", *o.LastCueDuration),
		}
	}
	return nil
}

// Validate checks every field of o, also the fields of formats that a write does not use.
func (o WriteOptions) Validate() error {
	if o.LineEnding > CRLF {
		return &InvalidArgumentError{
			Option: "WriteOptions.LineEnding",
			Msg:    "The line ending " + strconv.Itoa(int(o.LineEnding)) + " is unknown.",
		}
	}
	if o.BOM > BOMRemove {
		return &InvalidArgumentError{
			Option: "WriteOptions.BOM",
			Msg:    "The BOM rule " + strconv.Itoa(int(o.BOM)) + " is unknown.",
		}
	}
	return nil
}

func (o ReadOptions) common() model.CommonReadOptions {
	return model.CommonReadOptions{Lenient: o.Lenient, LastCueDuration: o.LastCueDuration}
}

func (o WriteOptions) common() model.CommonWriteOptions {
	return model.CommonWriteOptions{LineEnding: o.LineEnding, BOM: o.BOM, StripTags: o.StripTags}
}
