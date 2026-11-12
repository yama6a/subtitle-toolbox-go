package model

import "slices"

// Format is the name of a subtitle format. Its value is the format name of the CLI.
type Format string

// The formats of the registry, in registry order.
const (
	ASS           Format = "ass"
	CSV           Format = "csv"
	ITT           Format = "itt"
	JSON          Format = "json"
	LRC           Format = "lrc"
	MCC           Format = "mcc"
	MicroDVD      Format = "microdvd"
	MPSub         Format = "mpsub"
	SAMI          Format = "sami"
	SBV           Format = "sbv"
	SCC           Format = "scc"
	SubRip        Format = "srt"
	EBUSTL        Format = "stl"
	SpruceSTL     Format = "spruce-stl"
	SubViewer     Format = "subviewer"
	TTML          Format = "ttml"
	TSV           Format = "tsv"
	PlainText     Format = "txt"
	MPL2          Format = "mpl2"
	TMPlayer      Format = "tmplayer"
	QuickTimeText Format = "qttext"
	WebVTT        Format = "vtt"
)

type formatFact struct {
	format       Format
	extensions   []string
	canRead      bool
	canWrite     bool
	autoDetected bool
	binary       bool
}

//nolint:gochecknoglobals // lookup table of the format facts
var formatFacts = []formatFact{
	{format: ASS, extensions: []string{"ass", "ssa"}, canRead: true, canWrite: true, autoDetected: true},
	{format: CSV, extensions: []string{"csv"}, canRead: true, canWrite: true},
	{format: ITT, extensions: []string{"itt"}, canRead: true, canWrite: true},
	{format: JSON, extensions: []string{"json"}, canRead: true, canWrite: true, autoDetected: true},
	{format: LRC, extensions: []string{"lrc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MCC, extensions: []string{"mcc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MicroDVD, extensions: []string{"sub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MPSub, extensions: []string{"mpsub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SAMI, extensions: []string{"smi", "sami"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SBV, extensions: []string{"sbv"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SCC, extensions: []string{"scc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SubRip, extensions: []string{"srt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: EBUSTL, extensions: []string{"stl"}, canRead: true, canWrite: true, autoDetected: true, binary: true},
	{format: SpruceSTL, extensions: []string{"stl"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SubViewer, extensions: []string{"sub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TTML, extensions: []string{"ttml", "dfxp", "xml"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TSV, extensions: []string{"tsv"}, canRead: true, canWrite: true},
	{format: PlainText, extensions: []string{"txt"}, canWrite: true, autoDetected: true},
	{format: MPL2, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TMPlayer, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: QuickTimeText, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: WebVTT, extensions: []string{"vtt"}, canRead: true, canWrite: true, autoDetected: true},
}

func (f Format) fact() formatFact {
	i := slices.IndexFunc(formatFacts, func(r formatFact) bool { return r.format == f })
	if i < 0 {
		return formatFact{}
	}
	return formatFacts[i]
}

// Extensions returns the file extensions of f without the dot. The first one is the extension for new files.
func (f Format) Extensions() []string {
	return slices.Clone(f.fact().extensions)
}

// CanRead reports whether the library has a reader for f.
func (f Format) CanRead() bool {
	return f.fact().canRead
}

// CanWrite reports whether the library has a writer for f.
func (f Format) CanWrite() bool {
	return f.fact().canWrite
}

// IsAutoDetected reports false for a format that loads only when the caller names it.
// True does not mean that detection finds f by its content.
func (f Format) IsAutoDetected() bool {
	return f.fact().autoDetected
}

// IsBinary reports whether f is a binary format. Its reader gets the raw bytes of the input with no decoding,
// and ReadInfo.SourceEncoding is "". Its writer ignores LineEnding and BOM.
func (f Format) IsBinary() bool {
	return f.fact().binary
}
