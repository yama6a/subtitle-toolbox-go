package stbox

import (
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The shapes of the Read, Write and Detect functions of a format package.
type (
	readFunc[O any]     func(io.Reader, model.CommonReadOptions, O) (*model.Subtitle, []model.ParseWarning, error)
	writeFunc[O, R any] func(io.Writer, *model.Subtitle, model.CommonWriteOptions, O) (R, error)
	detectFunc          func([]byte) bool
)

var (
	_ readFunc[stboxjson.ReadOptions]                          = stboxjson.Read
	_ writeFunc[stboxjson.WriteOptions, stboxjson.WriteReport] = stboxjson.Write
	_ readFunc[webvtt.ReadOptions]                             = webvtt.Read
	_ writeFunc[webvtt.WriteOptions, webvtt.WriteReport]       = webvtt.Write
	_ detectFunc                                               = webvtt.Detect
	_ readFunc[srt.ReadOptions]                                = srt.Read
	_ writeFunc[srt.WriteOptions, srt.WriteReport]             = srt.Write
	_ detectFunc                                               = srt.Detect
	_ readFunc[ass.ReadOptions]                                = ass.Read
	_ writeFunc[ass.WriteOptions, ass.WriteReport]             = ass.Write
	_ detectFunc                                               = ass.Detect
	_ readFunc[cavena890.ReadOptions]                          = cavena890.Read
	_ writeFunc[cavena890.WriteOptions, cavena890.WriteReport] = cavena890.Write
	_ readFunc[ttml.ReadOptions]                               = ttml.Read
	_ writeFunc[ttml.WriteOptions, ttml.WriteReport]           = ttml.Write
	_ readFunc[microdvd.ReadOptions]                           = microdvd.Read
	_ writeFunc[microdvd.WriteOptions, microdvd.WriteReport]   = microdvd.Write
	_ detectFunc                                               = microdvd.Detect
	_ detectFunc                                               = ttml.Detect
	_ readFunc[sami.ReadOptions]                               = sami.Read
	_ writeFunc[sami.WriteOptions, sami.WriteReport]           = sami.Write
	_ detectFunc                                               = sami.Detect
	_ readFunc[sbv.ReadOptions]                                = sbv.Read
	_ writeFunc[sbv.WriteOptions, sbv.WriteReport]             = sbv.Write
	_ detectFunc                                               = sbv.Detect
	_ readFunc[scc.ReadOptions]                                = scc.Read
	_ writeFunc[scc.WriteOptions, scc.WriteReport]             = scc.Write
	_ detectFunc                                               = scc.Detect
	_ readFunc[mpl2.ReadOptions]                               = mpl2.Read
	_ writeFunc[mpl2.WriteOptions, mpl2.WriteReport]           = mpl2.Write
	_ detectFunc                                               = mpl2.Detect
	_ readFunc[tmplayer.ReadOptions]                           = tmplayer.Read
	_ writeFunc[tmplayer.WriteOptions, tmplayer.WriteReport]   = tmplayer.Write
	_ detectFunc                                               = tmplayer.Detect
	_ readFunc[mpsub.ReadOptions]                              = mpsub.Read
	_ writeFunc[mpsub.WriteOptions, mpsub.WriteReport]         = mpsub.Write
	_ detectFunc                                               = mpsub.Detect
	_ readFunc[pac.ReadOptions]                                = pac.Read
	_ writeFunc[pac.WriteOptions, pac.WriteReport]             = pac.Write
	_ detectFunc                                               = pac.Detect
	_ readFunc[pgs.ReadOptions]                                = pgs.Read
	_ writeFunc[pgs.WriteOptions, pgs.WriteReport]             = pgs.Write
	_ detectFunc                                               = pgs.Detect
	_ readFunc[lrc.ReadOptions]                                = lrc.Read
	_ writeFunc[lrc.WriteOptions, lrc.WriteReport]             = lrc.Write
	_ detectFunc                                               = lrc.Detect
	_ readFunc[mcc.ReadOptions]                                = mcc.Read
	_ writeFunc[mcc.WriteOptions, mcc.WriteReport]             = mcc.Write
	_ detectFunc                                               = mcc.Detect
	_ readFunc[vobsub.ReadOptions]                             = vobsub.Read
	_ readFunc[qttext.ReadOptions]                             = qttext.Read
	_ writeFunc[qttext.WriteOptions, qttext.WriteReport]       = qttext.Write
	_ detectFunc                                               = qttext.Detect
	_ writeFunc[plaintext.WriteOptions, plaintext.WriteReport] = plaintext.Write
	_ readFunc[csv.ReadOptions]                                = csv.Read
	_ writeFunc[csv.WriteOptions, csv.WriteReport]             = csv.Write
	_ readFunc[itt.ReadOptions]                                = itt.Read
	_ writeFunc[itt.WriteOptions, itt.WriteReport]             = itt.Write
	_ readFunc[subviewer.ReadOptions]                          = subviewer.Read
	_ writeFunc[subviewer.WriteOptions, subviewer.WriteReport] = subviewer.Write
	_ detectFunc                                               = subviewer.Detect
	_ readFunc[ebustl.ReadOptions]                             = ebustl.Read
	_ writeFunc[ebustl.WriteOptions, ebustl.WriteReport]       = ebustl.Write
	_ detectFunc                                               = ebustl.Detect
	_ readFunc[sprucestl.ReadOptions]                          = sprucestl.Read
	_ writeFunc[sprucestl.WriteOptions, sprucestl.WriteReport] = sprucestl.Write
	_ detectFunc                                               = sprucestl.Detect
	_ readFunc[dcp.ReadOptions]                                = dcp.Read
	_ writeFunc[dcp.WriteOptions, dcp.WriteReport]             = dcp.Write
	_ detectFunc                                               = dcp.Detect
)

func TestFormatsFollowTheRowOrder(t *testing.T) {
	t.Parallel()

	want := []Format{
		ASS, Cavena890, CSV, FFMetadataChapters, HTMLTranscript, ITT, JSON, AssemblyAI, AWSTranscribe, Deepgram, GoogleSpeech, LRC, MCC, MicroDVD, MPSub, PAC, PGS, SAMI, SBV, SCC, SubRip, EBUSTL,
		SpruceSTL, SubViewer, TTML, DCP, TSV, PlainText, MPL2, TMPlayer, QuickTimeText, VobSub, WebVTT, Whisper, YouTubeTimedText,
		OGMChapters, PodcastChapters, PodcastTranscript, YouTubeChapters,
	}
	assert.Equal(t, want, Formats())
}

func TestEveryFormatConstantHasARow(t *testing.T) {
	t.Parallel()

	consts := formatConsts(t)
	values := slices.Sorted(maps.Values(consts))
	assert.Equal(t, values, slices.Sorted(slices.Values(Formats())))
}

func TestFactsMatchTheFunctions(t *testing.T) {
	t.Parallel()

	for _, f := range Formats() {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			_, _, err := readFormat(f, strings.NewReader(""), ReadOptions{})
			assert.Equal(t, f.CanRead(), !errors.Is(err, errNoReader), "CanRead")
			_, err = writeFormat(io.Discard, &Subtitle{}, f, WriteOptions{})
			assert.Equal(t, f.CanWrite(), !errors.Is(err, errNoWriter), "CanWrite")
			if f.CanRead() {
				assert.Equal(t, detected(f), f.IsAutoDetected(), "IsAutoDetected")
			}
		})
	}
}

func detected(f Format) bool {
	return slices.ContainsFunc(jsonChecks, func(c jsonCheck) bool { return c.format == f }) ||
		slices.ContainsFunc(rawSignatures, func(s signature) bool { return s.format == f }) ||
		slices.ContainsFunc(signatures, func(s signature) bool { return s.format == f })
}

func TestPlainTextIsWriteOnly(t *testing.T) {
	t.Parallel()

	assert.True(t, PlainText.CanWrite())
	assert.False(t, PlainText.CanRead())
	assert.False(t, slices.ContainsFunc(signatures, func(s signature) bool { return s.format == PlainText }))
}

func TestDetectionListsFollowTheOrder(t *testing.T) {
	t.Parallel()

	jsonOrder := []Format{JSON, PodcastTranscript, Whisper, YouTubeTimedText}
	signatureOrder := []Format{
		WebVTT, TTML, DCP, SAMI, ASS, MPSub, MicroDVD, QuickTimeText, SubRip, SBV, SubViewer, LRC, EBUSTL, PAC, SpruceSTL, SCC, MCC, YouTubeTimedText, MPL2, TMPlayer, HTMLTranscript,
	}
	jsonFormats := make([]Format, 0, len(jsonChecks))
	signatureFormats := make([]Format, 0, len(signatures))
	for _, c := range jsonChecks {
		jsonFormats = append(jsonFormats, c.format)
	}
	for _, s := range signatures {
		signatureFormats = append(signatureFormats, s.format)
	}
	assert.True(t, isSubsequence(jsonFormats, jsonOrder), "JSON checks %v do not follow %v", jsonFormats, jsonOrder)
	assert.True(t, isSubsequence(signatureFormats, signatureOrder),
		"signatures %v do not follow %v", signatureFormats, signatureOrder)
	for _, f := range slices.Concat(jsonFormats, signatureFormats) {
		assert.True(t, f.IsAutoDetected(), "%s is in a detection list but not auto-detected", f)
		assert.True(t, f.CanRead(), "%s is in a detection list but cannot be read", f)
	}
}

func TestIsSubsequence(t *testing.T) {
	t.Parallel()

	order := []Format{WebVTT, TTML, SubRip}
	tests := []struct {
		name string
		list []Format
		want bool
	}{
		{"empty", nil, true},
		{"all", []Format{WebVTT, TTML, SubRip}, true},
		{"gap", []Format{WebVTT, SubRip}, true},
		{"wrong order", []Format{SubRip, WebVTT}, false},
		{"twice", []Format{WebVTT, WebVTT}, false},
		{"not in the order", []Format{ASS}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, isSubsequence(tt.list, order))
		})
	}
}

func TestFormatFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path   string
		want   Format
		wantOK bool
	}{
		{"movie.sub", MicroDVD, true},
		{"movie.SUB", MicroDVD, true},
		{"notes.txt", PlainText, true},
		{"clip.xml", TTML, true},
		{"cues.json", JSON, true},
		{"old.ssa", ASS, true},
		{"show.dfxp", TTML, true},
		{"show.smi", SAMI, true},
		{"show.sami", SAMI, true},
		{"show.vtt", WebVTT, true},
		{"movie.en.srt", SubRip, true},
		{"dir.d/movie.srt", SubRip, true},
		{"subtitles/movie.Srt", SubRip, true},
		{"movie", "", false},
		{"dir.srt/movie", "", false},
		{"movie.", "", false},
		{"movie.mkv", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			got, ok := FormatFromPath(tt.path)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatsReturnsACopy(t *testing.T) {
	t.Parallel()

	Formats()[0] = "changed"
	assert.Equal(t, ASS, Formats()[0])
}

func isSubsequence(list, order []Format) bool {
	i := 0
	for _, f := range list {
		j := slices.Index(order[i:], f)
		if j < 0 {
			return false
		}
		i += j + 1
	}
	return true
}
