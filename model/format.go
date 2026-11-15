package model

import "slices"

// Format is the name of a subtitle format. Its value is the format name of the CLI.
type Format string

// The formats. Formats returns them in facts table order.
const (
	ASS       Format = "ass"
	Cavena890 Format = "cavena890"
	CSV       Format = "csv"
	// HTMLTranscript is the HTML transcript of a podcast: a cite, a time and a p per paragraph.
	HTMLTranscript Format = "html"
	ITT            Format = "itt"
	JSON           Format = "json"
	AssemblyAI     Format = "assemblyai"
	AWSTranscribe  Format = "aws-transcribe"
	Deepgram       Format = "deepgram"
	GoogleSpeech   Format = "google-speech"
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
	// YouTubeTimedText is the timed text that YouTube serves: json3, srv3, srv2 and srv1.
	YouTubeTimedText Format = "youtube"
	// PodcastTranscript is the JSON transcript of a podcast feed.
	PodcastTranscript Format = "podcast-transcript"
	// YouTubeChapters is a list of lines such as "0:00 Intro".
	YouTubeChapters Format = "youtube-chapters"
	// FFMetadataChapters is the ;FFMETADATA1 text of FFmpeg.
	FFMetadataChapters Format = "ffmeta-chapters"
	// OGMChapters is the simple chapter format of mkvmerge.
	OGMChapters Format = "ogm-chapters"
	// PodcastChapters is the JSON chapters file of Podcasting 2.0.
	PodcastChapters Format = "podcast-chapters"
)

type formatFact struct {
	format     Format
	extensions []string
	canRead    bool
	canWrite   bool
	binary     bool
}

//nolint:gochecknoglobals // lookup table of the format facts
var formatFacts = []formatFact{
	{format: ASS, extensions: []string{"ass", "ssa"}, canRead: true, canWrite: true},
	{format: Cavena890, extensions: []string{"890"}, canRead: true, canWrite: true, binary: true},
	{format: CSV, extensions: []string{"csv"}, canRead: true, canWrite: true},
	{format: FFMetadataChapters, extensions: []string{"ffmeta"}, canRead: true, canWrite: true},
	{format: HTMLTranscript, extensions: []string{"html", "htm"}, canRead: true, canWrite: true},
	{format: ITT, extensions: []string{"itt"}, canRead: true, canWrite: true},
	{format: JSON, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: AssemblyAI, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: AWSTranscribe, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: Deepgram, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: GoogleSpeech, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: LRC, extensions: []string{"lrc"}, canRead: true, canWrite: true},
	{format: MCC, extensions: []string{"mcc"}, canRead: true, canWrite: true},
	{format: MicroDVD, extensions: []string{"sub"}, canRead: true, canWrite: true},
	{format: MPSub, extensions: []string{"mpsub"}, canRead: true, canWrite: true},
	{format: PAC, extensions: []string{"pac", "fpc"}, canRead: true, canWrite: true, binary: true},
	{format: PGS, extensions: []string{"sup"}, canRead: true, canWrite: true, binary: true},
	{format: SAMI, extensions: []string{"smi", "sami"}, canRead: true, canWrite: true},
	{format: SBV, extensions: []string{"sbv"}, canRead: true, canWrite: true},
	{format: SCC, extensions: []string{"scc"}, canRead: true, canWrite: true},
	{format: SubRip, extensions: []string{"srt"}, canRead: true, canWrite: true},
	{format: EBUSTL, extensions: []string{"stl"}, canRead: true, canWrite: true, binary: true},
	{format: SpruceSTL, extensions: []string{"stl"}, canRead: true, canWrite: true},
	{format: SubViewer, extensions: []string{"sub"}, canRead: true, canWrite: true},
	{format: TTML, extensions: []string{"ttml", "dfxp", "xml"}, canRead: true, canWrite: true},
	{format: DCP, extensions: []string{"xml"}, canRead: true, canWrite: true},
	{format: TSV, extensions: []string{"tsv"}, canRead: true, canWrite: true},
	{format: PlainText, extensions: []string{"txt"}, canWrite: true},
	{format: MPL2, extensions: []string{"txt"}, canRead: true, canWrite: true},
	{format: TMPlayer, extensions: []string{"txt"}, canRead: true, canWrite: true},
	{format: QuickTimeText, extensions: []string{"txt"}, canRead: true, canWrite: true},
	{format: VobSub, extensions: []string{"idx"}, canRead: true, binary: true},
	{format: WebVTT, extensions: []string{"vtt"}, canRead: true, canWrite: true},
	{format: Whisper, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: YouTubeTimedText, extensions: []string{"srv3", "json3", "srv1", "ytt"}, canRead: true, canWrite: true},
	{format: OGMChapters, extensions: []string{"txt"}, canRead: true, canWrite: true},
	{format: PodcastChapters, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: PodcastTranscript, extensions: []string{"json"}, canRead: true, canWrite: true},
	{format: YouTubeChapters, extensions: []string{"txt"}, canRead: true, canWrite: true},
}

// Formats returns every format of the facts table, in facts table order. The first row that lists an extension owns it.
func Formats() []Format {
	formats := make([]Format, len(formatFacts))
	for i, r := range formatFacts {
		formats[i] = r.format
	}
	return formats
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

// IsBinary reports whether f is a binary format. Its reader gets the raw bytes of the input with no decoding,
// and ReadInfo.SourceEncoding is "". Its writer ignores LineEnding and BOM.
func (f Format) IsBinary() bool {
	return f.fact().binary
}
