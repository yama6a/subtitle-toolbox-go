package conformance_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// goldenFolder is a row of the read golden table of https://github.com/yama6a/subtitle-toolbox-go/issues/9.
type goldenFolder struct {
	name   string
	format stbox.Format
	files  int
	// options holds the read options of single files by file name. Every other file reads with the zero options.
	options map[string]stbox.ReadOptions
	// transcript reads every file with transcriptGolden.
	transcript bool
	// defaults gives each file a second golden <file>.default.json, read with the zero options.
	defaults bool
}

// microDVDAt25 is the read option of the MicroDVD read goldens.
var microDVDAt25 = stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}

// dubbingScript is the CSV layout of csv/real/dubbing_script.csv in the read golden table of #9.
var dubbingScript = csv.ReadOptions{
	Columns:   csv.Columns{Start: csv.ColumnNamed("Start TC"), Speaker: csv.ColumnNamed("Character")},
	FrameRate: stbox.FrameRate{Num: 25, Den: 1},
}

// transcriptGolden holds the read options of the transcript read goldens.
var transcriptGolden = stbox.ReadOptions{
	Transcript: transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true},
	YouTube:    youtube.ReadOptions{WordTimestamps: true},
}

func goldenFolders() []goldenFolder {
	return []goldenFolder{
		{name: "ass", format: stbox.ASS, files: 7},
		{name: "assemblyai", format: stbox.AssemblyAI, files: 2, transcript: true, defaults: true},
		{name: "aws-transcribe", format: stbox.AWSTranscribe, files: 2, transcript: true, defaults: true},
		{name: "google-speech", format: stbox.GoogleSpeech, files: 2, transcript: true, defaults: true},
		{name: "cavena890", format: stbox.Cavena890, files: 7},
		{name: "chapters/ffmetadata", format: stbox.FFMetadataChapters, files: 3},
		{name: "chapters/ogm", format: stbox.OGMChapters, files: 2},
		{name: "chapters/podcast", format: stbox.PodcastChapters, files: 3},
		{name: "chapters/youtube", format: stbox.YouTubeChapters, files: 2},
		{name: "csv", format: stbox.CSV, files: 7, options: map[string]stbox.ReadOptions{
			"dubbing_script.csv": {CSV: dubbingScript},
			"headerless_frame_times.csv": {CSV: csv.ReadOptions{
				Columns:   csv.Columns{Start: csv.ColumnAt(0), End: csv.ColumnAt(1), Text: csv.ColumnAt(2), NoHeader: true},
				FrameRate: stbox.FrameRate{Num: 25, Den: 1},
			}},
			"excel_de_semicolon.csv": {CSV: csv.ReadOptions{
				Columns: csv.Columns{End: csv.ColumnNamed("Ende"), Speaker: csv.ColumnNamed("Sprecher")},
			}},
		}},
		{name: "dcp", format: stbox.DCP, files: 4},
		{name: "deepgram", format: stbox.Deepgram, files: 3, transcript: true, defaults: true},
		{name: "html", format: stbox.HTMLTranscript, files: 2, transcript: true},
		{name: "itt", format: stbox.ITT, files: 3},
		{name: "json", format: stbox.JSON, files: 6},
		{name: "lrc", format: stbox.LRC, files: 9},
		{name: "mcc", format: stbox.MCC, files: 6},
		{name: "microdvd", format: stbox.MicroDVD, files: 4, options: map[string]stbox.ReadOptions{
			"own_max_frames.sub":           microDVDAt25,
			"sub_microdvd.sub":             microDVDAt25,
			"sub_microdvd_with_styles.sub": microDVDAt25,
			"subsrt_sample.sub":            microDVDAt25,
		}},
		{name: "mpl2", format: stbox.MPL2, files: 2},
		{name: "mpsub", format: stbox.MPSub, files: 2},
		{name: "pac", format: stbox.PAC, files: 13},
		{name: "qttext", format: stbox.QuickTimeText, files: 2},
		{
			name: "sami", format: stbox.SAMI, files: 11,
			options: map[string]stbox.ReadOptions{"own_korean_cp949.smi": {Encoding: "CP949"}},
		},
		{name: "sbv", format: stbox.SBV, files: 5},
		{name: "scc", format: stbox.SCC, files: 9},
		{name: "spruce-stl", format: stbox.SpruceSTL, files: 3},
		{name: "srt", format: stbox.SubRip, files: 19},
		{name: "stl", format: stbox.EBUSTL, files: 17},
		{name: "subviewer", format: stbox.SubViewer, files: 6},
		{name: "tmplayer", format: stbox.TMPlayer, files: 4},
		{name: "ttml", format: stbox.TTML, files: 29},
		{name: "vtt", format: stbox.WebVTT, files: 30},
		{name: "podcast", format: stbox.PodcastTranscript, files: 3, transcript: true},
		{name: "whisper", format: stbox.Whisper, files: 10, transcript: true, defaults: true},
		{name: "youtube", format: stbox.YouTubeTimedText, files: 9, transcript: true},
	}
}

// realFiles returns the file names under <folder>/real, without SOURCES.md and the expected folder.
func (g goldenFolder) realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, g.name+"/real"))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	require.Len(t, names, g.files)
	return names
}

func (g goldenFolder) read(t *testing.T, file string) (*stbox.Subtitle, stbox.ReadInfo) {
	t.Helper()

	if g.transcript {
		return g.readWith(t, file, transcriptGolden)
	}
	return g.readWith(t, file, g.options[file])
}

func (g goldenFolder) readWith(t *testing.T, file string, opts stbox.ReadOptions) (*stbox.Subtitle, stbox.ReadInfo) {
	t.Helper()

	opts.Format = g.format
	s, info, err := parseBytes(testfiles.Read(t, g.name+"/real/"+file), opts)
	require.NoError(t, err)
	return s, info
}

// TestReadGoldens reads each real file and compares the warnings and the library JSON of the result with the read
// golden expected/<file>.json. With -update it rewrites a golden file whose content differs. Review each diff by hand.
func TestReadGoldens(t *testing.T) {
	t.Parallel()

	for _, g := range goldenFolders() {
		for _, file := range g.realFiles(t) {
			t.Run(g.name+"/"+file, func(t *testing.T) {
				t.Parallel()

				s, info := g.read(t, file)
				compareGolden(t, s, info, g.name+"/real/expected/"+file+".json")
			})
			if g.defaults {
				t.Run(g.name+"/"+file+".default", func(t *testing.T) {
					t.Parallel()

					s, info := g.readWith(t, file, stbox.ReadOptions{})
					compareGolden(t, s, info, g.name+"/real/expected/"+file+".default.json")
				})
			}
		}
	}
}

func compareGolden(t *testing.T, s *stbox.Subtitle, info stbox.ReadInfo, golden string) {
	t.Helper()

	var library bytes.Buffer
	_, err := stbox.Write(&library, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: 1}})
	require.NoError(t, err)
	warnings, err := json.Marshal(goldenWarnings(info.Warnings))
	require.NoError(t, err)
	var got bytes.Buffer
	got.WriteString(`{"warnings":`)
	got.Write(warnings)
	got.WriteString(`,"subtitle":`)
	got.Write(library.Bytes())
	got.WriteString("}")

	file := testfiles.Path(t, golden)
	if *update && canonicalJSONOf(t, testfiles.Read(t, golden)) != canonicalJSONOf(t, got.Bytes()) {
		var indented bytes.Buffer
		require.NoError(t, json.Indent(&indented, got.Bytes(), "", "    "))
		indented.WriteString("\n")
		require.NoError(t, os.WriteFile(file, indented.Bytes(), 0o600))
	}
	assert.Equal(t, canonicalJSONOf(t, testfiles.Read(t, golden)), canonicalJSONOf(t, got.Bytes()))
}

func canonicalJSONOf(t *testing.T, b []byte) string {
	t.Helper()

	var v any
	require.NoError(t, json.Unmarshal(b, &v))
	return canonicalJSON(t, v)
}

func TestEveryReadGoldenHasARealFile(t *testing.T) {
	t.Parallel()

	for _, g := range goldenFolders() {
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()

			paths, err := filepath.Glob(testfiles.Path(t, g.name+"/real/expected/*.json"))
			require.NoError(t, err)
			var got []string
			for _, p := range paths {
				name := filepath.Base(p)
				if !strings.Contains(name, ".out.") && !strings.Contains(name, ".to.") {
					got = append(got, name)
				}
			}
			files := g.realFiles(t)
			var want []string
			for _, file := range files {
				want = append(want, file+".json")
				if g.defaults {
					want = append(want, file+".default.json")
				}
			}
			assert.ElementsMatch(t, want, got)
		})
	}
}

// goldenWarnings returns warnings in the JSON shape of the read goldens, with null for NoLine and NoBlock.
func goldenWarnings(warnings []stbox.ParseWarning) []any {
	out := make([]any, 0, len(warnings))
	for _, w := range warnings {
		var line, block any
		if w.Line != stbox.NoLine {
			line = w.Line
		}
		if w.Block != stbox.NoBlock {
			block = w.Block
		}
		out = append(out, map[string]any{
			"message":    w.Message,
			"lineNumber": line,
			"blockIndex": block,
			"block":      append([]string{}, w.Lines...),
			"action":     string(w.Action),
		})
	}
	return out
}

// canonicalJSON encodes v with sorted object keys and one line per value, after a JSON round trip that turns every
// number into a float64. So key order and number spelling such as 4.0 against 4 do not count.
func canonicalJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(b, &decoded))
	b, err = json.MarshalIndent(decoded, "", "    ")
	require.NoError(t, err)
	return string(b)
}

func goldenPath(g goldenFolder, file, suffix string) string {
	return path.Join(g.name, "real", "expected", file+suffix)
}

// Ports testRealFilesGiveTheCuesOf1x of tests/ReadOptionsTest.php of subtitle-toolbox-php.
// Each golden holds the expected [start, end, lines] rows of the file with the options.
func TestReadOptionGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		golden string
		file   string
		opts   stbox.ReadOptions
	}{
		{
			golden: "french_windows_1252",
			file:   "encoding/french-windows-1252.srt",
			opts:   stbox.ReadOptions{Format: stbox.SubRip, Encoding: "Windows-1252"},
		},
		{
			golden: "sami_multi_language_frcc",
			file:   "sami/real/multi_language.smi",
			opts: stbox.ReadOptions{
				Format: stbox.SAMI, LastCueDuration: new(10 * time.Second), SAMI: sami.ReadOptions{LanguageClass: "FRCC"},
			},
		},
		{
			golden: "microdvd_subsrt_sample",
			file:   "microdvd/real/subsrt_sample.sub",
			opts:   stbox.ReadOptions{Format: stbox.MicroDVD, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 2997, Den: 125}}},
		},
		{
			golden: "csv_excel_de_semicolon",
			file:   "csv/real/excel_de_semicolon.csv",
			opts: stbox.ReadOptions{
				Format: stbox.CSV, LastCueDuration: new(10 * time.Second), CSV: csv.ReadOptions{Delimiter: csv.Semicolon},
			},
		},
		{
			golden: "scc_rollup_news_ndf",
			file:   "scc/real/rollup_news_ndf.scc",
			opts:   stbox.ReadOptions{Format: stbox.SCC, LastCueDuration: new(4 * time.Second), SCC: scc.ReadOptions{Channel: 1}},
		},
		{
			golden: "whisper_word_timestamps",
			file:   "whisper/real/openai_whisper_word_timestamps.json",
			opts:   stbox.ReadOptions{Format: stbox.Whisper, Transcript: transcript.ReadOptions{WordTimestamps: true}},
		},
		{
			golden: "whisperx_speaker_voices",
			file:   "whisper/real/whisperx_diarize.json",
			opts:   stbox.ReadOptions{Format: stbox.Whisper, Transcript: transcript.ReadOptions{SpeakerVoices: true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			path := testfiles.Path(t, tt.file)
			data, err := os.ReadFile(testfiles.Path(t, "read-options/"+tt.golden+".json"))
			require.NoError(t, err)
			var want any
			require.NoError(t, json.Unmarshal(data, &want))

			s, _, err := stbox.Load(path, tt.opts)
			require.NoError(t, err)
			var rows []any
			for _, c := range s.All() {
				tagged := c.TaggedLines()
				lines := make([]any, len(tagged))
				for i, line := range tagged {
					lines[i] = line
				}
				rows = append(rows, []any{c.Start.Seconds(), c.End.Seconds(), lines})
			}
			assert.Equal(t, want, any(rows))
		})
	}
}

// Ports testALastCueWithoutEndLastsFiveSecondsByDefault and testALastCueDurationOf0KeepsTheLastCue of
// tests/ReadOptionsTest.php of subtitle-toolbox-php. Each reader that uses the default adds its row.
func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		opts  stbox.ReadOptions
		start time.Duration
	}{
		{"LRC", "lrc/real/handwritten-core.lrc", stbox.ReadOptions{Format: stbox.LRC}, 32800 * time.Millisecond},
		{"QuickTime text", "qttext/last_sample_with_text.txt", stbox.ReadOptions{Format: stbox.QuickTimeText}, 12500 * time.Millisecond},
		{"TMPlayer", "tmplayer/real/tmplayer_crlf.txt", stbox.ReadOptions{Format: stbox.TMPlayer}, 3600 * time.Second},
		{"SAMI", "sami/real/mantas_smi_formatted.smi", stbox.ReadOptions{Format: stbox.SAMI}, 17350 * time.Millisecond},
		{"CSV", "csv/real/dubbing_script.csv", stbox.ReadOptions{Format: stbox.CSV, CSV: dubbingScript}, 36008200 * time.Millisecond},
		{"SCC", "read-options/last_caption_without_erase.scc", stbox.ReadOptions{Format: stbox.SCC}, 3504601 * time.Millisecond},
		{"SubViewer 1", "subviewer/real/subviewer1_delay.sub", stbox.ReadOptions{Format: stbox.SubViewer}, 17 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := testfiles.Path(t, tt.file)
			for _, c := range []struct {
				last *time.Duration
				want time.Duration
			}{{nil, 5 * time.Second}, {new(7 * time.Second), 7 * time.Second}, {new(time.Duration(0)), 0}} {
				opts := tt.opts
				opts.LastCueDuration = c.last
				s, _, err := stbox.Load(path, opts)
				require.NoError(t, err)
				last, _ := s.Cue(s.Len() - 1)
				assert.Equal(t, [2]time.Duration{tt.start, tt.start + c.want}, [2]time.Duration{last.Start, last.End})
			}
		})
	}
}
