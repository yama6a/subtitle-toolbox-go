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

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Aliases of the model types. Package model documents their methods.
type (
	// Subtitle is a list of cues in start order, with comments, metadata and format data.
	Subtitle = model.Subtitle
	// Cue is one timed text block.
	Cue = model.Cue
	// Word is a timed part of the text of a cue, usually one word.
	Word = model.Word
	// Image is the PNG bitmap of an image cue and its place on the screen.
	Image = model.Image
	// Comment is text that a writer puts before a cue, or after the last cue.
	Comment = model.Comment
	// SyncPoint is a time in the subtitle and the time where it belongs, for SyncByPoints.
	SyncPoint = model.SyncPoint
	// Alignment is the place of a cue on the screen.
	Alignment = model.Alignment
	// Position is where a cue shows on the screen: an alignment, a row and column on a grid, and an offset.
	Position = model.Position
	// Grid is a caption grid of rows and columns.
	Grid = model.Grid
	// Offset is a point in percent of the video width and height.
	Offset = model.Offset
	// FrameRate is a frame rate as an exact fraction, with a drop-frame flag.
	FrameRate = model.FrameRate
	// Timecode is a SMPTE time code label.
	Timecode = model.Timecode
	// DropFrameMode selects the time code labels of the SCC and MCC writers.
	DropFrameMode = model.DropFrameMode
	// Format is the name of a subtitle format.
	Format = model.Format
	// LineEnding is the line ending that a writer uses.
	LineEnding = model.LineEnding
	// BOM says whether a writer starts its output with a UTF-8 BOM.
	BOM = model.BOM
	// Container is a video file format that holds subtitle tracks.
	Container = model.Container
	// Track is a subtitle track of a container.
	Track = model.Track
	// TrackError is the error of a container read without a track number, when the file has no subtitle track or
	// more than one.
	TrackError = model.TrackError
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
	// ReplaceOptions holds the settings of Subtitle.ReplaceText.
	ReplaceOptions = model.ReplaceOptions
	// StripOptions holds the settings of Subtitle.StripFormatting.
	StripOptions = model.StripOptions
	// CaseMode is the target case of Subtitle.ChangeCase.
	CaseMode = model.CaseMode
	// DialogueDashStyle is the dash, and the space after it, that starts a dialogue line.
	DialogueDashStyle = model.DialogueDashStyle
	// CueLimits holds the size and time limits of one cue.
	CueLimits = model.CueLimits
	// MergeShortCuesOptions holds the settings of Subtitle.MergeShortCues.
	MergeShortCuesOptions = model.MergeShortCuesOptions
)

// The formats of the registry, in registry order.
const (
	ASS           = model.ASS
	Cavena890     = model.Cavena890
	CSV           = model.CSV
	ITT           = model.ITT
	JSON          = model.JSON
	AWSTranscribe = model.AWSTranscribe
	LRC           = model.LRC
	MCC           = model.MCC
	MicroDVD      = model.MicroDVD
	MPSub         = model.MPSub
	PAC           = model.PAC
	PGS           = model.PGS
	SAMI          = model.SAMI
	SBV           = model.SBV
	SCC           = model.SCC
	SubRip        = model.SubRip
	EBUSTL        = model.EBUSTL
	SubViewer     = model.SubViewer
	TTML          = model.TTML
	DCP           = model.DCP
	TSV           = model.TSV
	PlainText     = model.PlainText
	MPL2          = model.MPL2
	TMPlayer      = model.TMPlayer
	QuickTimeText = model.QuickTimeText
	SpruceSTL     = model.SpruceSTL
	VobSub        = model.VobSub
	WebVTT        = model.WebVTT
	Whisper       = model.Whisper
	// PodcastTranscript is the format of the Podcasting 2.0 JSON transcript.
	PodcastTranscript = model.PodcastTranscript
	// YouTubeChapters is the format of the chapter lines of a YouTube description.
	YouTubeChapters = model.YouTubeChapters
)

// The containers that a read can name.
const (
	Matroska = model.Matroska
	MP4      = model.MP4
	MPEGTS   = model.MPEGTS
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

// The values of ParseWarning.Line and ParseWarning.Block for a warning without a line or a block.
const (
	NoLine  = model.NoLine
	NoBlock = model.NoBlock
)

// The case modes of ChangeCase.
const (
	CaseUpper    = model.CaseUpper
	CaseLower    = model.CaseLower
	CaseSentence = model.CaseSentence
)

// The dialogue dash styles. DialogueDashHyphenSpace is the zero value.
const (
	DialogueDashHyphenSpace        = model.DialogueDashHyphenSpace
	DialogueDashHyphen             = model.DialogueDashHyphen
	DialogueDashUnicodeHyphen      = model.DialogueDashUnicodeHyphen
	DialogueDashUnicodeHyphenSpace = model.DialogueDashUnicodeHyphenSpace
	DialogueDashEnDash             = model.DialogueDashEnDash
	DialogueDashEnDashSpace        = model.DialogueDashEnDashSpace
	DialogueDashEmDash             = model.DialogueDashEmDash
	DialogueDashEmDashSpace        = model.DialogueDashEmDashSpace
)

// The defaults of CueLimits and MergeShortCuesOptions.
const (
	DefaultMaxCharactersPerLine = model.DefaultMaxCharactersPerLine
	DefaultMaxLinesPerCue       = model.DefaultMaxLinesPerCue
	DefaultMinDuration          = model.DefaultMinDuration
	DefaultMaxDuration          = model.DefaultMaxDuration
	DefaultMergeMaxGap          = model.DefaultMergeMaxGap
)

// The DropFrameMode values.
const (
	DropFrameKeep = model.DropFrameKeep
	DropFrameOn   = model.DropFrameOn
	DropFrameOff  = model.DropFrameOff
)

// The named frame rates. Treat them as read-only.
//
//nolint:gochecknoglobals // re-exported named frame rate
var (
	FrameRate23976  = model.FrameRate23976
	FrameRate24     = model.FrameRate24
	FrameRate25     = model.FrameRate25
	FrameRate2997   = model.FrameRate2997
	FrameRate2997DF = model.FrameRate2997DF
	FrameRate30     = model.FrameRate30
	FrameRate50     = model.FrameRate50
	FrameRate5994   = model.FrameRate5994
	FrameRate5994DF = model.FrameRate5994DF
	FrameRate60     = model.FrameRate60
)

// ParseTimecode reads the label HH:MM:SS:FF or HH:MM:SS;FF.
func ParseTimecode(s string) (Timecode, error) {
	return model.ParseTimecode(s)
}

// ParseTime returns the time of a time string such as "00:01:02,500", "01:02.5" or, with r, "00:01:02:12".
func ParseTime(s string, r FrameRate) (time.Duration, error) {
	return model.ParseTime(s, r)
}

// FrameRateFromFloat returns the shortest decimal of fps as a fraction, so 23.976 gives 2997/125.
func FrameRateFromFloat(fps float64) (FrameRate, error) {
	return model.FrameRateFromFloat(fps)
}
