// Package stbox reads, edits and writes subtitles.
//
// It is the facade of the module. Load, Parse and Write read and write every format,
// Detect and FormatFromPath find the format of a file, and Formats lists the registry.
// The types, constants and errors of package model appear here under the same names,
// as aliases and copies, so most callers import only stbox. Package model documents
// the methods of the aliased types.
//
// A program that needs one format can import its package under format/ alone.
package stbox

import "github.com/yama6a/subtitle-toolbox-go/model"

// Aliases of the model types. Package model documents their methods.
type (
	// Subtitle is a list of cues in start order, with comments, metadata and format data.
	Subtitle = model.Subtitle
	// Cue is one timed text block.
	Cue = model.Cue
	// Comment is text that a writer puts before a cue, or after the last cue.
	Comment = model.Comment
	// Alignment is the place of a cue on the screen.
	Alignment = model.Alignment
	// Format is the name of a subtitle format.
	Format = model.Format
	// LineEnding is the line ending that a writer uses.
	LineEnding = model.LineEnding
	// BOM says whether a writer starts its output with a UTF-8 BOM.
	BOM = model.BOM
	// ParseWarning is one problem that a parser skipped or repaired.
	ParseWarning = model.ParseWarning
	// WarningAction says what a parser did with a broken block.
	WarningAction = model.WarningAction
	// ParseError is an error of a reader that cannot read its input.
	ParseError = model.ParseError
	// UnwritableError is an error of a writer whose format cannot hold the subtitle.
	UnwritableError = model.UnwritableError
	// InvalidArgumentError is an error of an invalid option or argument.
	InvalidArgumentError = model.InvalidArgumentError
)

// The formats of the registry, in registry order.
const (
	ASS           = model.ASS
	CSV           = model.CSV
	JSON          = model.JSON
	LRC           = model.LRC
	MicroDVD      = model.MicroDVD
	MPSub         = model.MPSub
	SAMI          = model.SAMI
	SBV           = model.SBV
	SubRip        = model.SubRip
	SubViewer     = model.SubViewer
	TTML          = model.TTML
	TSV           = model.TSV
	PlainText     = model.PlainText
	MPL2          = model.MPL2
	TMPlayer      = model.TMPlayer
	QuickTimeText = model.QuickTimeText
	WebVTT        = model.WebVTT
)

// The 9 alignments of the numeric keypad grid.
const (
	BottomCenter = model.BottomCenter
	BottomLeft   = model.BottomLeft
	BottomRight  = model.BottomRight
	MiddleLeft   = model.MiddleLeft
	MiddleCenter = model.MiddleCenter
	MiddleRight  = model.MiddleRight
	TopLeft      = model.TopLeft
	TopCenter    = model.TopCenter
	TopRight     = model.TopRight
)

// The named metadata keys. Other keys are free.
const (
	MetadataTitle    = model.MetadataTitle
	MetadataAuthor   = model.MetadataAuthor
	MetadataArtist   = model.MetadataArtist
	MetadataAlbum    = model.MetadataAlbum
	MetadataLanguage = model.MetadataLanguage
)

// The line endings.
const (
	LF   = model.LF
	CRLF = model.CRLF
)

// The BOM rules.
const (
	BOMDefault = model.BOMDefault
	BOMAdd     = model.BOMAdd
	BOMRemove  = model.BOMRemove
)

// The warning actions.
const (
	Skipped  = model.Skipped
	Repaired = model.Repaired
)
