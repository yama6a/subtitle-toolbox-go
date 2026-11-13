package stbox_test

import (
	"bytes"
	"flag"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// roundTripFolders holds the folders whose every real file is a round trip case.
func roundTripFolders() []string {
	return []string{"ass", "dcp", "lrc", "microdvd", "mpl2", "mpsub", "qttext", "sami", "sbv", "spruce-stl", "srt", "subviewer", "tmplayer", "vtt"}
}

// roundTripFile is a round trip case in a folder that roundTripFolders does not hold. Its golden is
// expected/<file>.out.<extension of the file>, as in PHP.
type roundTripFile struct {
	folder, file string
}

func roundTripFiles() []roundTripFile {
	return []roundTripFile{
		{"ttml", "bbc_ebu_tt_d.ttml"},
		{"ttml", "mantas_netflix_ticks.dfxp"},
		{"ttml", "w3c_imsc11_regions.ttml"},
		{"scc", "popon_broadcast_df.scc"},
		{"itt", "fcp_23976_styles.itt"},
		{"mcc", "captionmaker_608_708.mcc"},
		{"mcc", "captionmaker_popon_df.mcc"},
		{"mcc", "v1_23976_timecode.mcc"},
		{"html", "hhmmss_empty_paragraphs.html"},
		{"html", "spec_example_shape.html"},
		{"google-speech", "bus_v1_long_running_operation.json"},
		{"google-speech", "restaurant_v2_diarization.json"},
		{"podcast", "podcast_transcript_convert_from_html.json"},
		{"podcast", "podcast_transcript_convert_from_srt.json"},
		{"podcast", "spec_word_segments.json"},
		{"whisper", "faster_whisper_ctranslate2_segments.json"},
		{"whisper", "faster_whisper_ctranslate2_words.json"},
		{"whisper", "openai_api_verbose_json_french.json"},
		{"whisper", "openai_api_verbose_json_words.json"},
		{"whisper", "openai_whisper_german.json"},
		{"whisper", "openai_whisper_word_timestamps.json"},
		{"whisper", "whisper_cpp_oj.json"},
		{"whisper", "whisper_cpp_ojf.json"},
		{"whisper", "whisper_cpp_ojf_split_utf8.json"},
		{"whisper", "whisperx_diarize.json"},
		{"youtube", "auto.en.json3"},
		{"youtube", "manual.en.json3"},
		{"youtube", "blank-lines.en.srv3"},
	}
}

// conversion is a write golden case that reads a real file and writes it in another format. A zero class reads
// with the read options of the read golden table.
type conversion struct {
	folder, file string
	to           stbox.Format
	class        string
}

func conversions() []conversion {
	return []conversion{
		{"ass", "own_aegisub.ass", stbox.SubRip, ""},
		{"ass", "own_aegisub.ass", stbox.WebVTT, ""},
		{"ass", "own_signs_crlf.ass", stbox.SubRip, ""},
		{"ass", "own_signs_crlf.ass", stbox.WebVTT, ""},
		{"cavena890", "harbour_english.890", stbox.ASS, ""},
		{"cavena890", "harbour_english.890", stbox.SubRip, ""},
		{"cavena890", "harbour_english.890", stbox.WebVTT, ""},
		{"csv", "mantas_done_shape.csv", stbox.SubRip, ""},
		{"csv", "mantas_done_shape.csv", stbox.WebVTT, ""},
		{"csv", "mantas_done_shape.csv", stbox.ASS, ""},
		{"csv", "own_openai_whisper.tsv", stbox.SubRip, ""},
		{"csv", "own_whisper_cpp.csv", stbox.SubRip, ""},
		{"dcp", "monal_interop_shape.xml", stbox.SubRip, ""},
		{"dcp", "monal_interop_shape.xml", stbox.WebVTT, ""},
		{"dcp", "monal_interop_shape.xml", stbox.ASS, ""},
		{"dcp", "subtitle_edit_smpte2014_shape.xml", stbox.SubRip, ""},
		{"dcp", "subtitle_edit_smpte2014_shape.xml", stbox.WebVTT, ""},
		{"dcp", "subtitle_edit_smpte2014_shape.xml", stbox.ASS, ""},
		{"itt", "fcp_23976_styles.itt", stbox.SubRip, ""},
		{"itt", "fcp_23976_styles.itt", stbox.WebVTT, ""},
		{"itt", "fcp_23976_styles.itt", stbox.ASS, ""},
		{"lrc", "handwritten-enhanced.lrc", stbox.SubRip, ""},
		{"lrc", "handwritten-enhanced.lrc", stbox.WebVTT, ""},
		{"lrc", "handwritten-enhanced.lrc", stbox.ASS, ""},
		{"mpl2", "subtitle_edit_shape.txt", stbox.SubRip, ""},
		{"mpl2", "subtitle_edit_shape.txt", stbox.WebVTT, ""},
		{"mpl2", "subtitle_edit_shape.txt", stbox.ASS, ""},
		{"mpsub", "mplayer_doc_time.sub", stbox.SubRip, ""},
		{"mpsub", "mplayer_doc_time.sub", stbox.WebVTT, ""},
		{"mpsub", "mplayer_doc_time.sub", stbox.ASS, ""},
		{"pac", "bakery_latin.pac", stbox.SubRip, ""},
		{"pac", "bakery_latin.pac", stbox.WebVTT, ""},
		{"pac", "bakery_latin.pac", stbox.ASS, ""},
		{"qttext", "quicktime_export_styles.txt", stbox.SubRip, ""},
		{"qttext", "quicktime_export_styles.txt", stbox.WebVTT, ""},
		{"qttext", "quicktime_export_styles.txt", stbox.ASS, ""},
		{"sami", "multi_language.smi", stbox.SubRip, "ENCC"},
		{"sami", "multi_language.smi", stbox.WebVTT, "ENCC"},
		{"sami", "multi_language.smi", stbox.ASS, "ENCC"},
		{"sbv", "youtube_studio_lf.sbv", stbox.SubRip, ""},
		{"sbv", "youtube_studio_lf.sbv", stbox.WebVTT, ""},
		{"sbv", "youtube_studio_lf.sbv", stbox.ASS, ""},
		{"spruce-stl", "dvdsp_full_header_ntsc.stl", stbox.SubRip, ""},
		{"spruce-stl", "dvdsp_full_header_ntsc.stl", stbox.WebVTT, ""},
		{"spruce-stl", "dvdsp_full_header_ntsc.stl", stbox.ASS, ""},
		{"srt", "own_styled.srt", stbox.Cavena890, ""},
		{"srt", "own_styled.srt", stbox.SAMI, ""},
		{"srt", "own_styled.srt", stbox.ASS, ""},
		{"srt", "own_styled.srt", stbox.TTML, ""},
		{"srt", "own_styled.srt", stbox.WebVTT, ""},
		{"srt", "own_styled.srt", stbox.PAC, ""},
		{"srt", "own_styled.srt", stbox.SCC, ""},
		{"scc", "popon_broadcast_df.scc", stbox.SubRip, ""},
		{"scc", "popon_broadcast_df.scc", stbox.WebVTT, ""},
		{"scc", "popon_broadcast_df.scc", stbox.ASS, ""},
		{"scc", "rollup_news_ndf.scc", stbox.SubRip, ""},
		{"scc", "rollup_news_ndf.scc", stbox.WebVTT, ""},
		{"scc", "rollup_news_ndf.scc", stbox.ASS, ""},
		{"mcc", "captionmaker_608_708.mcc", stbox.SubRip, ""},
		{"mcc", "captionmaker_608_708.mcc", stbox.WebVTT, ""},
		{"mcc", "captionmaker_608_708.mcc", stbox.ASS, ""},
		{"mcc", "v2_5994df_fields.mcc", stbox.SubRip, ""},
		{"mcc", "v2_5994df_fields.mcc", stbox.WebVTT, ""},
		{"mcc", "v2_5994df_fields.mcc", stbox.ASS, ""},
		{"stl", "bakery_teletext_25fps.stl", stbox.SubRip, ""},
		{"stl", "bakery_teletext_25fps.stl", stbox.TTML, ""},
		{"stl", "irt_requirement-0056-001_modified.stl", stbox.SubRip, ""},
		{"stl", "irt_requirement-0056-001_modified.stl", stbox.TTML, ""},
		{"stl", "irt_requirement-0061-001.stl", stbox.SubRip, ""},
		{"stl", "irt_requirement-0061-001.stl", stbox.TTML, ""},
		{"stl", "irt_requirement-0074-001.stl", stbox.SubRip, ""},
		{"stl", "irt_requirement-0074-001.stl", stbox.TTML, ""},
		{"stl", "irt_requirement-0076-001.stl", stbox.SubRip, ""},
		{"stl", "irt_requirement-0076-001.stl", stbox.TTML, ""},
		{"stl", "irt_requirement-0076-009.stl", stbox.SubRip, ""},
		{"stl", "irt_requirement-0076-009.stl", stbox.TTML, ""},
		{"subviewer", "subviewer2_crlf.sub", stbox.SubRip, ""},
		{"subviewer", "subviewer2_crlf.sub", stbox.WebVTT, ""},
		{"subviewer", "subviewer2_crlf.sub", stbox.ASS, ""},
		{"tmplayer", "tmplayer_multiline_bom.txt", stbox.SubRip, ""},
		{"tmplayer", "tmplayer_multiline_bom.txt", stbox.WebVTT, ""},
		{"tmplayer", "tmplayer_multiline_bom.txt", stbox.ASS, ""},
		{"vtt", "w3c_voices.vtt", stbox.ASS, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.SubRip, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.WebVTT, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.ASS, ""},
		{"vtt", "w3c_voices.vtt", stbox.SubRip, ""},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.ASS, ""},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.SubRip, ""},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.ASS, ""},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.SubRip, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.SubRip, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.WebVTT, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.ASS, ""},
		{"html", "spec_example_shape.html", stbox.SubRip, ""},
		{"html", "spec_example_shape.html", stbox.WebVTT, ""},
		{"google-speech", "restaurant_v2_diarization.json", stbox.SubRip, ""},
		{"google-speech", "restaurant_v2_diarization.json", stbox.WebVTT, ""},
		{"podcast", "spec_word_segments.json", stbox.SubRip, ""},
		{"podcast", "spec_word_segments.json", stbox.WebVTT, ""},
		{"whisper", "openai_whisper_word_timestamps.json", stbox.SubRip, ""},
		{"whisper", "openai_whisper_word_timestamps.json", stbox.WebVTT, ""},
		{"whisper", "whisperx_diarize.json", stbox.SubRip, ""},
		{"whisper", "whisperx_diarize.json", stbox.WebVTT, ""},
		{"youtube", "auto.en.json3", stbox.SubRip, ""},
		{"youtube", "auto.en.json3", stbox.WebVTT, ""},
		{"youtube", "styled.en.srv3", stbox.SubRip, ""},
		{"youtube", "styled.en.srv3", stbox.WebVTT, ""},
		{"youtube", "transcript.en.srv1", stbox.SubRip, ""},
		{"youtube", "transcript.en.srv1", stbox.WebVTT, ""},
		{"srt", "own_styled.srt", stbox.YouTubeTimedText, ""},
		{"vtt", "w3c_timestamps.vtt", stbox.YouTubeTimedText, ""},
		{"srt", "own_styled.srt", stbox.GoogleSpeech, ""},
		{"srt", "own_styled.srt", stbox.Whisper, ""},
		{"vtt", "w3c_timestamps.vtt", stbox.GoogleSpeech, ""},
		{"vtt", "w3c_timestamps.vtt", stbox.Whisper, ""},
	}
}

func folder(t *testing.T, name string) goldenFolder {
	t.Helper()

	for _, g := range goldenFolders() {
		if g.name == name {
			return g
		}
	}
	t.Fatalf("the read golden table has no folder %q", name)
	return goldenFolder{}
}

func writeAs(t *testing.T, s *stbox.Subtitle, f stbox.Format) []byte {
	t.Helper()

	var out bytes.Buffer
	_, err := stbox.Write(&out, s, f, stbox.WriteOptions{})
	require.NoError(t, err)
	return out.Bytes()
}

// TestWriteGoldens writes each case with the zero write options. A round trip matches the normalised input or its
// golden expected/<file>.out.<extension>. A conversion matches its golden expected/<file>.to.<format>.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range roundTripFolders() {
		g := folder(t, name)
		for _, file := range g.realFiles(t) {
			t.Run(g.name+"/"+file+".out", func(t *testing.T) {
				t.Parallel()

				s, _ := g.read(t, file)
				golden.RoundTrip(t, testfiles.Path(t, g.name+"/real/"+file),
					testfiles.Path(t, goldenPath(g, file, ".out."+g.format.Extensions()[0])), writeAs(t, s, g.format), *update)
			})
		}
	}
	for _, r := range roundTripFiles() {
		t.Run(r.folder+"/"+r.file+".out", func(t *testing.T) {
			t.Parallel()

			g := folder(t, r.folder)
			s, _ := g.read(t, r.file)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, r.file, ".out"+path.Ext(r.file))), writeAs(t, s, g.format), *update)
		})
	}
	for _, c := range conversions() {
		t.Run(c.folder+"/"+c.file+".to."+string(c.to), func(t *testing.T) {
			t.Parallel()

			g := folder(t, c.folder)
			opts := g.options[c.file]
			if c.class != "" {
				opts.SAMI.LanguageClass = c.class
			}
			if c.folder == "microdvd" {
				// The PHP write goldens read MicroDVD at 23.976 fps, the read goldens at 25 fps.
				opts.MicroDVD.FrameRate = stbox.FrameRate{Num: 2997, Den: 125}
			}
			s, _ := g.readWith(t, c.file, opts)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, c.file, conversionSuffix(c.to))), writeAs(t, s, c.to), *update)
		})
	}
}

// TestWriteGoldenPodcastWordSegments writes the word segments of a read with word timestamps.
func TestWriteGoldenPodcastWordSegments(t *testing.T) {
	t.Parallel()

	const file = "spec_word_segments.json"
	g := folder(t, "podcast")
	s, _ := g.read(t, file)
	var out bytes.Buffer
	opts := stbox.WriteOptions{Transcript: transcript.WriteOptions{Podcast: transcript.PodcastWriteOptions{WordSegments: true}}}
	_, err := stbox.Write(&out, s, stbox.PodcastTranscript, opts)
	require.NoError(t, err)
	golden.Equal(t, testfiles.Path(t, goldenPath(g, file, ".out.wordsegments.json")), out.Bytes(), *update)
}

func TestWriteGoldenSCCNonDropFrame(t *testing.T) {
	t.Parallel()

	g := folder(t, "srt")
	s, _ := g.read(t, "own_styled.srt")
	var out bytes.Buffer
	_, err := stbox.Write(&out, s, stbox.SCC, stbox.WriteOptions{SCC: scc.WriteOptions{DropFrame: stbox.DropFrameOff}})
	require.NoError(t, err)
	golden.Equal(t, testfiles.Path(t, goldenPath(g, "own_styled.srt", ".to.ndf.scc")), out.Bytes(), *update)
}

// TestWriteGoldenMCCNewFile writes a file that holds no stored header. The test sets the header field Creation Date
// first, because a new file holds the date of the day.
func TestWriteGoldenMCCNewFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		suffix string
		opts   mcc.WriteOptions
	}{
		{".to.mcc", mcc.WriteOptions{}},
		{".to.23.976.mcc", mcc.WriteOptions{FrameRate: stbox.FrameRate23976}},
	}
	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			t.Parallel()

			g := folder(t, "srt")
			s, _ := g.read(t, "own_styled.srt")
			require.NoError(t, s.SetFormatData("mcc", model.NewData(
				model.Field{Key: "header", Value: model.NewData(model.Field{Key: "Creation Date", Value: "Monday, October 12, 2026"})},
			)))
			var out bytes.Buffer
			report, err := stbox.Write(&out, s, stbox.MCC, stbox.WriteOptions{MCC: tt.opts})
			require.NoError(t, err)
			require.NotNil(t, report.MCC)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, "own_styled.srt", tt.suffix)), out.Bytes(), *update)
		})
	}
}

// conversionSuffix returns .to.<format>, plus .<extension> when the first extension of f differs from its name,
// for example .to.sami.smi.
func conversionSuffix(f stbox.Format) string {
	suffix := ".to." + string(f)
	if ext := f.Extensions()[0]; ext != string(f) {
		suffix += "." + ext
	}
	return suffix
}

// Ports testSpacesAroundWordTimestampsSeparateTheWords of tests/Parsers/LyricsParserTest.php of subtitle-toolbox-php.
func TestLRCWordTimesWriteAsWordsInSubRip(t *testing.T) {
	t.Parallel()

	s, _ := folder(t, "lrc").read(t, "handwritten-enhanced.lrc")
	first, ok := s.Cue(0)
	require.True(t, ok)
	assert.Equal(t, []string{"<00:00:03.000>Slow <00:00:03.550>river <00:00:04.400>runs"}, first.TaggedLines())
	assert.Contains(t, string(writeAs(t, s, stbox.SubRip)), "\nSlow river runs\n")
}

// TestWriteEBUSTLGoldens converts own_styled.srt at 25 and at 30 fps. The file data sets CD and RD, because a new file
// holds today's date there.
func TestWriteEBUSTLGoldens(t *testing.T) {
	t.Parallel()

	for _, rate := range []stbox.FrameRate{{Num: 25, Den: 1}, {Num: 30, Den: 1}} {
		t.Run(rate.String(), func(t *testing.T) {
			t.Parallel()

			g := folder(t, "srt")
			s, _ := g.read(t, "own_styled.srt")
			gsi := model.NewData(model.Field{Key: "CD", Value: "261012"}, model.Field{Key: "RD", Value: "261012"})
			require.NoError(t, s.SetFormatData("stl", model.NewData(model.Field{Key: "gsi", Value: gsi})))
			var out bytes.Buffer
			_, err := stbox.Write(&out, s, stbox.EBUSTL, stbox.WriteOptions{EBUSTL: ebustl.WriteOptions{FrameRate: rate}})
			require.NoError(t, err)
			golden.Equal(t, testfiles.Path(t, "srt/real/expected/own_styled.srt.to."+rate.String()+".stl"), out.Bytes(), *update)
		})
	}
}
