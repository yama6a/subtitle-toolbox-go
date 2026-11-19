package stbox

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

type jsonCheck struct {
	format Format
	match  func(keys map[string]json.RawMessage) bool
}

type signature struct {
	format Format
	match  func(content []byte) bool
}

//nolint:gochecknoglobals // ordered detection lists
var (
	jsonChecks = []jsonCheck{
		{format: JSON, match: stboxjson.DetectJSON},
		{format: PodcastTranscript, match: transcript.DetectPodcastJSON},
		{format: Whisper, match: transcript.DetectWhisperJSON},
		{format: YouTubeTimedText, match: youtube.DetectJSON},
	}
	rawSignatures = []signature{
		{format: PGS, match: pgs.Detect},
	}
	signatures = []signature{
		{format: WebVTT, match: webvtt.Detect},
		{format: TTML, match: ttml.Detect},
		{format: DCP, match: dcp.Detect},
		{format: SAMI, match: sami.Detect},
		{format: ASS, match: ass.Detect},
		{format: MPSub, match: mpsub.Detect},
		{format: MicroDVD, match: microdvd.Detect},
		{format: QuickTimeText, match: qttext.Detect},
		{format: SubRip, match: srt.Detect},
		{format: SBV, match: sbv.Detect},
		{format: SubViewer, match: subviewer.Detect},
		{format: LRC, match: lrc.Detect},
		{format: EBUSTL, match: ebustl.Detect},
		{format: PAC, match: pac.Detect},
		{format: SpruceSTL, match: sprucestl.Detect},
		{format: SCC, match: scc.Detect},
		{format: MCC, match: mcc.Detect},
		{format: YouTubeTimedText, match: youtube.Detect},
		{format: MPL2, match: mpl2.Detect},
		{format: TMPlayer, match: tmplayer.Detect},
		{format: HTMLTranscript, match: transcript.DetectHTML},
	}
)

// Formats returns every format of the facts table, in facts table order.
func Formats() []Format {
	return model.Formats()
}

// FormatFromPath returns the format of the file extension of path, compared without case.
// When formats share an extension, the first row of the facts table owns it: ".sub" is MicroDVD,
// ".txt" is plain text, ".xml" is TTML and ".json" is the library JSON.
func FormatFromPath(path string) (Format, bool) {
	formats := formatsOfExtension(extension(path))
	if len(formats) == 0 {
		return "", false
	}
	return formats[0], true
}

// formatsOfExtension returns the formats that list ext, in facts table order.
func formatsOfExtension(ext string) []Format {
	if ext == "" {
		return nil
	}
	var formats []Format
	for _, f := range Formats() {
		if slices.Contains(f.Extensions(), ext) {
			formats = append(formats, f)
		}
	}
	return formats
}

// extension returns the file extension of path without the dot, in lower case.
func extension(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

// errNoReader and errNoWriter are the results of readFormat and writeFormat for a format without a reader or
// a writer. Read and Write check CanRead and CanWrite first, so callers never see them.
var (
	errNoReader = errors.New("stbox: no reader")
	errNoWriter = errors.New("stbox: no writer")
)

// readFormat reads r with the reader of f.
//
//nolint:cyclop,gocyclo,funlen // one case per format
func readFormat(f Format, r io.Reader, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	common := opts.common()
	switch f {
	case ASS:
		return ass.Read(r, common, ass.ReadOptions{})
	case Cavena890:
		return cavena890.Read(r, common, cavena890.ReadOptions{})
	case CSV:
		return csv.Read(r, common, opts.CSV)
	case FFMetadataChapters:
		return chapters.ReadFFMetadata(r, common, opts.Chapters)
	case HTMLTranscript:
		return transcript.ReadHTML(r, common, opts.Transcript)
	case ITT:
		return itt.Read(r, common, itt.ReadOptions{})
	case JSON:
		return stboxjson.Read(r, common, stboxjson.ReadOptions{})
	case AssemblyAI:
		return transcript.ReadAssemblyAI(r, common, opts.Transcript)
	case AWSTranscribe:
		return transcript.ReadAWSTranscribe(r, common, opts.Transcript)
	case Deepgram:
		return transcript.ReadDeepgram(r, common, opts.Transcript)
	case GoogleSpeech:
		return transcript.ReadGoogleSpeech(r, common, opts.Transcript)
	case LRC:
		return lrc.Read(r, common, lrc.ReadOptions{})
	case MCC:
		return mcc.Read(r, common, opts.MCC)
	case MicroDVD:
		return microdvd.Read(r, common, opts.MicroDVD)
	case MPSub:
		return mpsub.Read(r, common, mpsub.ReadOptions{})
	case PAC:
		return readPAC(r, common, opts)
	case PGS:
		return pgs.Read(r, common, pgs.ReadOptions{})
	case SAMI:
		return sami.Read(r, common, opts.SAMI)
	case SBV:
		return sbv.Read(r, common, sbv.ReadOptions{})
	case SCC:
		return scc.Read(r, common, opts.SCC)
	case SubRip:
		return srt.Read(r, common, srt.ReadOptions{})
	case EBUSTL:
		return ebustl.Read(r, common, opts.EBUSTL)
	case SpruceSTL:
		return sprucestl.Read(r, common, opts.SpruceSTL)
	case SubViewer:
		return subviewer.Read(r, common, subviewer.ReadOptions{})
	case TTML:
		return ttml.Read(r, common, ttml.ReadOptions{})
	case DCP:
		return dcp.Read(r, common, dcp.ReadOptions{})
	case TSV:
		return readTSV(r, common, opts)
	case PlainText:
		return nil, nil, errNoReader
	case MPL2:
		return mpl2.Read(r, common, mpl2.ReadOptions{})
	case TMPlayer:
		return tmplayer.Read(r, common, tmplayer.ReadOptions{})
	case QuickTimeText:
		return qttext.Read(r, common, qttext.ReadOptions{})
	case VobSub:
		return vobsub.Read(r, common, opts.VobSub)
	case WebVTT:
		return webvtt.Read(r, common, webvtt.ReadOptions{})
	case Whisper:
		return transcript.ReadWhisper(r, common, opts.Transcript)
	case YouTubeTimedText:
		return youtube.Read(r, common, opts.YouTube)
	case OGMChapters:
		return chapters.ReadOGM(r, common, opts.Chapters)
	case PodcastChapters:
		return chapters.ReadPodcast(r, common, opts.Chapters)
	case PodcastTranscript:
		return transcript.ReadPodcast(r, common, opts.Transcript)
	case YouTubeChapters:
		return chapters.ReadYouTube(r, common, opts.Chapters)
	}
	return nil, nil, errNoReader
}

// writeFormat writes s with the writer of f.
//
//nolint:cyclop,gocyclo,funlen // one case per format
func writeFormat(w io.Writer, s *Subtitle, f Format, opts WriteOptions) (WriteReport, error) {
	common := opts.common()
	switch f {
	case ASS:
		_, err := ass.Write(w, s, common, opts.ASS)
		return WriteReport{}, err
	case Cavena890:
		_, err := cavena890.Write(w, s, common, cavena890.WriteOptions{})
		return WriteReport{}, err
	case CSV:
		return writeCSV(w, s, common, opts)
	case FFMetadataChapters:
		_, err := chapters.WriteFFMetadata(w, s, common, chapters.WriteOptions{})
		return WriteReport{}, err
	case HTMLTranscript:
		_, err := transcript.WriteHTML(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case ITT:
		_, err := itt.Write(w, s, common, opts.ITT)
		return WriteReport{}, err
	case JSON:
		_, err := stboxjson.Write(w, s, common, opts.JSON)
		return WriteReport{}, err
	case AssemblyAI:
		_, err := transcript.WriteAssemblyAI(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case AWSTranscribe:
		_, err := transcript.WriteAWSTranscribe(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case Deepgram:
		_, err := transcript.WriteDeepgram(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case GoogleSpeech:
		_, err := transcript.WriteGoogleSpeech(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case LRC:
		_, err := lrc.Write(w, s, common, lrc.WriteOptions{})
		return WriteReport{}, err
	case MCC:
		report, err := mcc.Write(w, s, common, opts.MCC)
		if err != nil {
			return WriteReport{}, err
		}
		return WriteReport{MCC: &report}, nil
	case MicroDVD:
		_, err := microdvd.Write(w, s, common, opts.MicroDVD)
		return WriteReport{}, err
	case MPSub:
		_, err := mpsub.Write(w, s, common, opts.MPSub)
		return WriteReport{}, err
	case PAC:
		_, err := pac.Write(w, s, common, pac.WriteOptions{})
		return WriteReport{}, err
	case PGS:
		_, err := pgs.Write(w, s, common, pgs.WriteOptions{})
		return WriteReport{}, err
	case SAMI:
		_, err := sami.Write(w, s, common, sami.WriteOptions{})
		return WriteReport{}, err
	case SBV:
		_, err := sbv.Write(w, s, common, sbv.WriteOptions{})
		return WriteReport{}, err
	case SCC:
		report, err := scc.Write(w, s, common, opts.SCC)
		if err != nil {
			return WriteReport{}, err
		}
		return WriteReport{SCC: &report}, nil
	case SubRip:
		_, err := srt.Write(w, s, common, srt.WriteOptions{})
		return WriteReport{}, err
	case EBUSTL:
		report, err := ebustl.Write(w, s, common, opts.EBUSTL)
		if err != nil {
			return WriteReport{}, err
		}
		return WriteReport{EBUSTL: &report}, nil
	case SpruceSTL:
		_, err := sprucestl.Write(w, s, common, opts.SpruceSTL)
		return WriteReport{}, err
	case SubViewer:
		_, err := subviewer.Write(w, s, common, opts.SubViewer)
		return WriteReport{}, err
	case TTML:
		_, err := ttml.Write(w, s, common, ttml.WriteOptions{})
		return WriteReport{}, err
	case DCP:
		_, err := dcp.Write(w, s, common, dcp.WriteOptions{})
		return WriteReport{}, err
	case TSV:
		return writeTSV(w, s, common, opts)
	case PlainText:
		_, err := plaintext.Write(w, s, common, opts.PlainText)
		return WriteReport{}, err
	case MPL2:
		_, err := mpl2.Write(w, s, common, mpl2.WriteOptions{})
		return WriteReport{}, err
	case TMPlayer:
		_, err := tmplayer.Write(w, s, common, tmplayer.WriteOptions{})
		return WriteReport{}, err
	case QuickTimeText:
		_, err := qttext.Write(w, s, common, qttext.WriteOptions{})
		return WriteReport{}, err
	case VobSub:
		return WriteReport{}, errNoWriter
	case WebVTT:
		report, err := webvtt.Write(w, s, common, webvtt.WriteOptions{})
		if err != nil {
			return WriteReport{}, err
		}
		return WriteReport{WebVTT: &report}, nil
	case Whisper:
		_, err := transcript.WriteWhisper(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case YouTubeTimedText:
		_, err := youtube.Write(w, s, common, youtube.WriteOptions{})
		return WriteReport{}, err
	case OGMChapters:
		_, err := chapters.WriteOGM(w, s, common, chapters.WriteOptions{})
		return WriteReport{}, err
	case PodcastChapters:
		_, err := chapters.WritePodcast(w, s, common, chapters.WriteOptions{})
		return WriteReport{}, err
	case PodcastTranscript:
		_, err := transcript.WritePodcast(w, s, common, opts.Transcript)
		return WriteReport{}, err
	case YouTubeChapters:
		_, err := chapters.WriteYouTube(w, s, common, chapters.WriteOptions{})
		return WriteReport{}, err
	}
	return WriteReport{}, errNoWriter
}

func readPAC(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	pacOpts := opts.PAC
	if pacOpts.CodePage == pac.CodePageAuto && opts.Encoding != "" {
		codePage, ok := pac.CodePageFromEncoding(opts.Encoding)
		if !ok {
			return nil, nil, &InvalidArgumentError{
				Option: "ReadOptions.Encoding",
				Msg:    "PAC files have no code page for the encoding " + strconv.Quote(opts.Encoding),
			}
		}
		pacOpts.CodePage = codePage
	}
	return pac.Read(r, common, pacOpts)
}

// readTSV reads with the CSV reader, which detects the delimiter, and names TSV in its errors.
func readTSV(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	s, warnings, err := csv.Read(r, common, opts.CSV)
	var perr *ParseError
	if errors.As(err, &perr) && perr.Format == CSV {
		withFormat := *perr
		withFormat.Format = TSV
		return nil, nil, &withFormat
	}
	return s, warnings, err
}

// writeCSV writes a comma for a subtitle whose stored delimiter is a tab, unless the options set a delimiter.
func writeCSV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if d, _ := s.FormatData(string(CSV)); opts.CSV.Delimiter == csv.DelimiterDefault && isTab(d) {
		opts.CSV.Delimiter = csv.Comma
	}
	_, err := csv.Write(w, s, common, opts.CSV)
	return WriteReport{}, err
}

func isTab(d *model.Data) bool {
	delimiter, _ := d.String("delimiter")
	return delimiter == string(csv.Tab)
}

// writeTSV writes a tab unless the options set a delimiter.
func writeTSV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if opts.CSV.Delimiter == csv.DelimiterDefault {
		opts.CSV.Delimiter = csv.Tab
	}
	s, err := writing.SkipImageCues(s, model.TSV, common)
	if err != nil {
		return WriteReport{}, err
	}
	_, err = csv.Write(w, s, common, opts.CSV)
	return WriteReport{}, err
}
