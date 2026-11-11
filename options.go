package stbox

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// ReadOptions holds the settings of a read. The zero value reads with the defaults.
type ReadOptions struct {
	// Format is the format of the input. The zero value detects the format.
	Format Format
	// Encoding is the name of the source encoding, one of Encodings, compared without case. The zero value detects
	// the encoding. A BOM and UTF-16 without a BOM win over it, and valid UTF-8 without a NUL byte stays UTF-8.
	// A binary format ignores it.
	Encoding string
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	// A lenient read with warnings returns a nil error.
	Lenient bool
	// LastCueDuration is the length of a last cue without an end. Nil means 5 s.
	LastCueDuration *time.Duration
	// SAMI holds the settings of the SAMI reader. The zero value reads the first class of the STYLE block that a
	// <P> uses.
	SAMI sami.ReadOptions
	// MicroDVD holds the settings of the MicroDVD reader. The zero value takes the frame rate of a {1}{1}<fps> line.
	MicroDVD microdvd.ReadOptions
	// CSV holds the settings of the CSV and TSV reader. The zero value reads the columns by their header names and
	// detects the delimiter.
	CSV csv.ReadOptions
	// SCC holds the settings of the SCC reader. The zero value reads channel 1 and gives one cue per screen of
	// roll-up captions.
	SCC scc.ReadOptions
	// EBUSTL holds the settings of the EBU STL reader. The zero value keeps the times of the file.
	EBUSTL ebustl.ReadOptions
	// SpruceSTL holds the settings of the Spruce STL reader. The zero value guesses 25 or 30 fps from the frame numbers.
	SpruceSTL sprucestl.ReadOptions
}

// WriteOptions holds the settings of a write. The zero value writes with the defaults.
type WriteOptions struct {
	// LineEnding is the line ending of the output. The zero value is LF.
	LineEnding LineEnding
	// BOM says whether the output starts with a UTF-8 BOM. The zero value keeps the format default.
	BOM BOM
	// StripTags writes the text without markup. False keeps the markup that the format can hold.
	StripTags bool
	// JSON holds the settings of the library JSON writer. The zero value writes compact JSON with format data.
	JSON stboxjson.WriteOptions
	// ASS holds the settings of the ASS and SSA writer. The zero value writes \k karaoke tags and keeps the styles.
	ASS ass.WriteOptions
	// MicroDVD holds the settings of the MicroDVD writer. The zero value takes the frame rate that the reader stored.
	MicroDVD microdvd.WriteOptions
	// MPSub holds the settings of the MPSub writer. The zero value writes seconds and FORMAT=TIME.
	MPSub mpsub.WriteOptions
	// PlainText holds the settings of the plain text writer. The zero value joins the lines of a cue and the cues
	// of a paragraph with a space, starts a paragraph at a gap of 2 s and writes no times.
	PlainText plaintext.WriteOptions
	// CSV holds the settings of the CSV and TSV writer. The zero value keeps the layout that the reader stored.
	CSV csv.WriteOptions
	// SubViewer holds the settings of the SubViewer writer. The zero value writes the stored version, else SubViewer 2.
	SubViewer subviewer.WriteOptions
	// SpruceSTL holds the settings of the Spruce STL writer. The zero value takes the frame rate that the reader stored, else 25.
	SpruceSTL sprucestl.WriteOptions
}

// WriteReport holds the changes that a writer made to fit the subtitle into its format.
type WriteReport struct {
	// WebVTT is non-nil after a WebVTT write, also when it holds no dropped color.
	WebVTT *webvtt.WriteReport
}

// ReadInfo holds the facts of a read that are not subtitle content.
type ReadInfo struct {
	// Format is the format that the read used.
	Format Format
	// SourceEncoding is the encoding of the input: ReadOptions.Encoding as written, or the canonical name of the
	// encoding that the read picked, for example "UTF-8" or "Windows-1252". It is "" for a binary format.
	SourceEncoding string
	// Warnings holds every warning of the read, in read order. A strict read can hold warnings too,
	// for repairs that a reader makes in both modes.
	Warnings []ParseWarning
}

// Validate checks every field of o, also the fields of formats that a read does not use.
func (o ReadOptions) Validate() error {
	if o.Encoding != "" {
		if _, err := textenc.Lookup(o.Encoding); err != nil {
			return &InvalidArgumentError{
				Option: "ReadOptions.Encoding",
				Msg:    "The encoding " + strconv.Quote(o.Encoding) + " is unknown.",
			}
		}
	}
	if o.LastCueDuration != nil && *o.LastCueDuration < 0 {
		return &InvalidArgumentError{
			Option: "ReadOptions.LastCueDuration",
			Msg:    fmt.Sprintf("The last cue duration must be 0 or more, got %s.", *o.LastCueDuration),
		}
	}
	if err := o.SAMI.Validate(); err != nil {
		return err
	}
	if err := o.MicroDVD.Validate(); err != nil {
		return err
	}
	if err := o.CSV.Validate(); err != nil {
		return err
	}
	if err := o.SCC.Validate(); err != nil {
		return err
	}
	return o.SpruceSTL.Validate()
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
	if err := o.ASS.Validate(); err != nil {
		return err
	}
	if err := o.MicroDVD.Validate(); err != nil {
		return err
	}
	if err := o.MPSub.Validate(); err != nil {
		return err
	}
	if err := o.PlainText.Validate(); err != nil {
		return err
	}
	if err := o.CSV.Validate(); err != nil {
		return err
	}
	if err := o.SubViewer.Validate(); err != nil {
		return err
	}
	return o.SpruceSTL.Validate()
}

func (o ReadOptions) common() model.CommonReadOptions {
	return model.CommonReadOptions{Lenient: o.Lenient, LastCueDuration: o.LastCueDuration}
}

func (o WriteOptions) common() model.CommonWriteOptions {
	return model.CommonWriteOptions{LineEnding: o.LineEnding, BOM: o.BOM, StripTags: o.StripTags}
}

// Encodings returns the canonical names that ReadOptions.Encoding accepts, without the aliases of docs/encodings.md.
func Encodings() []string {
	return textenc.Names()
}
