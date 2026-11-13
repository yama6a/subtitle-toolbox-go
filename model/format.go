package model

import "slices"

// Format is the name of a subtitle format. Its value is the format name of the CLI.
type Format string

// The formats of the registry, in registry order.
const (
	ASS       Format = "ass"
	Cavena890 Format = "cavena890"
	CSV       Format = "csv"
	// HTMLTranscript is the HTML transcript of a podcast: a cite, a time and a p per paragraph. Detection finds it last.
	HTMLTranscript Format = "html"
	ITT            Format = "itt"
	JSON           Format = "json"
	AWSTranscribe  Format = "aws-transcribe"
	LRC            Format = "lrc"
	MCC            Format = "mcc"
	MicroDVD       Format = "microdvd"
	MPSub          Format = "mpsub"
	PAC            Format = "pac"
	PGS            Format = "pgs"
	SAMI           Format = "sami"
	SBV            Format = "sbv"
	SCC            Format = "scc"
	SubRip         Format = "srt"
	EBUSTL         Format = "stl"
	SpruceSTL      Format = "spruce-stl"
	SubViewer      Format = "subviewer"
	TTML           Format = "ttml"
	DCP            Format = "dcp"
	TSV            Format = "tsv"
	PlainText      Format = "txt"
	MPL2           Format = "mpl2"
	TMPlayer       Format = "tmplayer"
	QuickTimeText  Format = "qttext"
	VobSub         Format = "vobsub"
	WebVTT         Format = "vtt"
	Whisper        Format = "whisper"
	// PodcastTranscript is the JSON transcript of a podcast feed. Detection finds it before Whisper JSON.
	PodcastTranscript Format = "podcast-transcript"
	// YouTubeChapters is a list of lines such as "0:00 Intro". Detection never finds it.
	YouTubeChapters Format = "youtube-chapters"
	// FFMetadataChapters is the ;FFMETADATA1 text of FFmpeg. Detection never finds it.
	FFMetadataChapters Format = "ffmeta-chapters"
	// OGMChapters is the simple chapter format of mkvmerge. Detection never finds it.
	OGMChapters Format = "ogm-chapters"
	// PodcastChapters is the JSON chapters file of Podcasting 2.0. Detection never finds it.
	PodcastChapters Format = "podcast-chapters"
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
	{format: Cavena890, extensions: []string{"890"}, canRead: true, canWrite: true, binary: true},
	{format: CSV, extensions: []string{"csv"}, canRead: true, canWrite: true},
	{format: FFMetadataChapters, extensions: []string{"ffmeta"}, canRead: true, canWrite: true},
	{format: HTMLTranscript, extensions: []string{"html", "htm"}, canRead: true, canWrite: true, autoDetected: true},
	{format: ITT, extensions: []string{"itt"}, canRead: true, canWrite: true},
	{format: JSON, extensions: []string{"json"}, canRead: true, canWrite: true, autoDetected: true},
	{format: AWSTranscribe, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: LRC, extensions: []string{"lrc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MCC, extensions: []string{"mcc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MicroDVD, extensions: []string{"sub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: MPSub, extensions: []string{"mpsub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: PAC, extensions: []string{"pac", "fpc"}, canRead: true, canWrite: true, autoDetected: true, binary: true},
	{format: PGS, extensions: []string{"sup"}, canRead: true, canWrite: true, autoDetected: true, binary: true},
	{format: SAMI, extensions: []string{"smi", "sami"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SBV, extensions: []string{"sbv"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SCC, extensions: []string{"scc"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SubRip, extensions: []string{"srt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: EBUSTL, extensions: []string{"stl"}, canRead: true, canWrite: true, autoDetected: true, binary: true},
	{format: SpruceSTL, extensions: []string{"stl"}, canRead: true, canWrite: true, autoDetected: true},
	{format: SubViewer, extensions: []string{"sub"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TTML, extensions: []string{"ttml", "dfxp", "xml"}, canRead: true, canWrite: true, autoDetected: true},
	{format: DCP, extensions: []string{"xml"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TSV, extensions: []string{"tsv"}, canRead: true, canWrite: true},
	{format: PlainText, extensions: []string{"txt"}, canWrite: true, autoDetected: true},
	{format: MPL2, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: TMPlayer, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: QuickTimeText, extensions: []string{"txt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: VobSub, extensions: []string{"idx"}, canRead: true, binary: true},
	{format: WebVTT, extensions: []string{"vtt"}, canRead: true, canWrite: true, autoDetected: true},
	{format: Whisper, extensions: []string{"json"}, canRead: true, canWrite: true, autoDetected: true},
	{format: OGMChapters, extensions: []string{"txt"}, canRead: true, canWrite: true},
	{format: PodcastChapters, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: PodcastTranscript, extensions: []string{"json"}, canRead: true, canWrite: true, autoDetected: true},
	{format: YouTubeChapters, extensions: []string{"txt"}, canRead: true, canWrite: true},
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
