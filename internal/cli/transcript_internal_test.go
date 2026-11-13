package cli

import (
	"bytes"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
	"github.com/yama6a/subtitle-toolbox-go/v4/speakers"
)

// libraryCall is the library call that a CLI call of TestConvertTranscriptOptions must equal.
type libraryCall struct {
	read  stbox.ReadOptions
	write stbox.WriteOptions
	edit  func(s *stbox.Subtitle) error
}

func (l libraryCall) output(t *testing.T, path string, f stbox.Format) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, path), l.read)
	require.NoError(t, err)
	if l.edit != nil {
		require.NoError(t, l.edit(s))
	}
	l.write.BOM = stbox.BOMRemove
	var buf bytes.Buffer
	_, err = stbox.Write(&buf, s, f, l.write)
	require.NoError(t, err)
	return buf.String()
}

func withWords() stbox.ReadOptions {
	return stbox.ReadOptions{Transcript: transcript.ReadOptions{WordTimestamps: true}}
}

func byWords(opts resegment.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error { _, err := resegment.ByWords(s, opts); return err }
}

func splitLong(opts resegment.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error { _, err := resegment.SplitLong(s, opts); return err }
}

func highlight(opts karaoke.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error { _, err := karaoke.Apply(s, opts); return err }
}

func labels(opts speakers.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error { _, err := speakers.Apply(s, opts); return err }
}

func TestConvertTranscriptOptions(t *testing.T) {
	t.Parallel()

	const (
		long   = "resegmenting/own_whisper_long_segments.json"
		song   = "whisper/real/openai_whisper_word_timestamps.json"
		lyrics = "lrc/real/handwritten-enhanced.lrc"
		voices = "speakers/voices.vtt"
		sdhSRT = "speakers/sdh_labels.srt"
	)
	gap := 300 * time.Millisecond
	narrow := resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: 30, MaxLinesPerCue: 1}, MaxWordGap: &gap}
	narrowSplit := resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: 30, MaxLinesPerCue: 1}}
	yellow := `font color="#ffff00"`
	tests := []struct {
		name string
		file string
		to   stbox.Format
		args []string
		lib  libraryCall
	}{
		{"words in WebVTT", long, stbox.WebVTT, []string{"--word-timestamps"}, libraryCall{read: withWords()}},
		{"words with --from", long, stbox.WebVTT, []string{"--word-timestamps", "--from", "whisper"}, libraryCall{read: withWords()}},
		{"no words without the flag", long, stbox.WebVTT, nil, libraryCall{}},
		{"flag on a format without words", "cli/trip.srt", stbox.WebVTT, []string{"--word-timestamps"}, libraryCall{}},
		{"resegment", long, stbox.SubRip, []string{"--structure-resegment"}, libraryCall{read: withWords(), edit: byWords(resegment.Options{})}},
		{
			"resegment with limits and gap", long, stbox.SubRip,
			[]string{"--structure-resegment", "--structure-max-cpl", "30", "--structure-max-lines", "1", "--structure-max-word-gap", "00:00:00.3"},
			libraryCall{read: withWords(), edit: byWords(narrow)},
		},
		{
			"split long with words", long, stbox.WebVTT,
			[]string{"--structure-split-long", "--word-timestamps"},
			libraryCall{read: withWords(), edit: splitLong(resegment.Options{})},
		},
		{"split long without words", long, stbox.WebVTT, []string{"--structure-split-long"}, libraryCall{edit: splitLong(resegment.Options{})}},
		{
			"split long with limits", long, stbox.WebVTT,
			[]string{"--structure-split-long", "--structure-max-cpl", "30", "--structure-max-lines", "1"},
			libraryCall{edit: splitLong(narrowSplit)},
		},
		{"karaoke", song, stbox.SubRip, []string{"--karaoke"}, libraryCall{read: withWords(), edit: highlight(karaoke.Options{})}},
		{
			"karaoke style", lyrics, stbox.SubRip,
			[]string{"--karaoke", "--karaoke-style", yellow},
			libraryCall{read: withWords(), edit: highlight(karaoke.Options{Style: yellow})},
		},
		{
			"ASS karaoke tag", song, stbox.ASS,
			[]string{"--ass-karaoke-tag", "kf"},
			libraryCall{read: withWords(), write: stbox.WriteOptions{ASS: ass.WriteOptions{KaraokeTag: ass.KaraokeFill}}},
		},
		{"speakers prefix", voices, stbox.SubRip, []string{"--speakers", "prefix"}, libraryCall{edit: labels(speakers.Options{Style: speakers.StylePrefix})}},
		{
			"speakers dashes", voices, stbox.SubRip,
			[]string{"--speakers", "dashes"},
			libraryCall{edit: labels(speakers.Options{Style: speakers.StyleDialogueDashes})},
		},
		{"speakers colors", voices, stbox.SubRip, []string{"--speakers", "colors"}, libraryCall{edit: labels(speakers.Options{Style: speakers.StyleColors})}},
		{"speakers from prefix", sdhSRT, stbox.WebVTT, []string{"--speakers", "from-prefix"}, libraryCall{edit: labels(speakers.Options{FromLabels: true})}},
		{"speakers run before strip-tags", voices, stbox.SubRip, []string{"--strip-tags", "--speakers", "prefix"}, libraryCall{edit: all(
			labels(speakers.Options{Style: speakers.StylePrefix}),
			func(s *stbox.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil },
		)}},
		{"speakers run before case", sdhSRT, stbox.WebVTT, []string{"--case", "lower", "--speakers", "from-prefix", "--language", "en"}, libraryCall{edit: all(
			labels(speakers.Options{FromLabels: true}),
			func(s *stbox.Subtitle) error { return s.ChangeCase(model.CaseLower, "en") },
		)}},
	}
	dir := newDir(t)
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := "in" + strconv.Itoa(i) + filepath.Ext(tt.file)
			copyFile(t, testfiles.Path(t, tt.file), filepath.Join(dir, input))
			args := append([]string{"convert", input, "--to", string(tt.to), "-o", "-", "--no-bom"}, tt.args...)
			assert.Equal(t, result{Stdout: tt.lib.output(t, tt.file, tt.to)}, call{dir: dir}.run(t, args...))
		})
	}
}

func TestConvertTranscriptUsageErrors(t *testing.T) {
	t.Parallel()

	song := []string{"convert", "trip.srt", "--to", "ass", "-o", "-"}
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"speakers mode", []string{"--speakers", "names"}, `The option --speakers must be prefix, dashes, colors or from-prefix, got "names".`},
		{"case before speakers", []string{"--speakers", "names", "--case", "title"}, `The option --case must be upper, lower or sentence, got "title".`},
		{"speakers before rtl", []string{"--rtl", "left", "--speakers", "names"}, `The option --speakers must be prefix, dashes, colors or from-prefix, got "names".`},
		{"karaoke tag value", []string{"--ass-karaoke-tag", "x"}, `The option --ass-karaoke-tag must be k, kf or ko, got "x".`},
		{"karaoke with ASS tag", []string{"--karaoke", "--ass-karaoke-tag", "k"}, "Pass only one of --karaoke and --ass-karaoke-tag."},
		{"style without karaoke", []string{"--karaoke-style", "b"}, "Pass --karaoke with --karaoke-style."},
		{
			"style em",
			[]string{"--karaoke", "--karaoke-style", "em"},
			`The style em must be one of the core markup tags b, i, u, s or font, for example u or font color="#ffff00".`,
		},
		{
			"style empty",
			[]string{"--karaoke", "--karaoke-style", ""},
			`The style  must be one of the core markup tags b, i, u, s or font, for example u or font color="#ffff00".`,
		},
		{"gap without resegment", []string{"--structure-max-word-gap", "1"}, "Pass --structure-resegment with --structure-max-word-gap."},
		{"gap value", []string{"--structure-resegment", "--structure-max-word-gap", "0"}, "The option --structure-max-word-gap must be greater than 0."},
		{
			"gap rule before limits rule",
			[]string{"--structure-max-word-gap", "1", "--structure-max-cpl", "30"},
			"Pass --structure-resegment with --structure-max-word-gap.",
		},
		{
			"limits with split long",
			[]string{"--structure-max-cpl", "x", "--structure-split-long"},
			`The option --structure-max-cpl needs a whole number of 1 or more, got "x".`,
		},
		{"karaoke mode", []string{"--karaoke", "--karaoke-mode", "cumulative"}, "Unknown option --karaoke-mode."},
		{"karaoke words", []string{"--karaoke", "--karaoke-words", "2"}, "Unknown option --karaoke-words."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.msg), call{dir: dir}.run(t, append(song, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestConvertSCCSplitAdvice(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"bakery.stl": "stl/real/bakery_teletext_25fps.stl"})
	run := func(extra ...string) result {
		return call{dir: dir}.run(t, append([]string{
			"convert", "bakery.stl", "--from", "stl", "--to", "scc", "-o", "-",
			"--structure-wrap", "--structure-max-cpl", "32", "--structure-max-lines", "4",
		}, extra...)...)
	}
	failed := run()
	assert.Equal(t, exitFile, failed.Code)
	assert.Contains(t, failed.Stderr, ". Pass --structure-split-long --structure-wrap --structure-max-cpl 32 --structure-max-lines 4.\n")

	fixed := run("--structure-split-long")
	assert.Equal(t, 0, fixed.Code)
	assert.Empty(t, fixed.Stderr)
	assert.Contains(t, fixed.Stdout, "Scenarist_SCC V1.0")
}

func TestNeedsFromAdvice(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, testfiles.Path(t, "cli/notes.txt"), dir+"/notes.txt")
	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"convert", []string{"convert", "notes.txt", "-o", "-"}, "notes.txt: Format detection found no subtitle format. Pass --from FORMAT. " +
			"Chapters and cloud speech-to-text JSON always need it, for example --from deepgram.\n"},
		{"sync reference", []string{"sync", "notes.txt", "--reference", "notes.txt", "-o", "-"}, "notes.txt: Format detection found no subtitle format. " +
			"Write it to a subtitle file with convert --from FORMAT first. " +
			"Chapters and cloud speech-to-text JSON always need --from, for example --from deepgram.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, tt.words...)
			assert.Equal(t, exitFile, got.Code)
			assert.Contains(t, got.Stderr, tt.want)
		})
	}
}

func TestWordTimestampsOptionIsAcceptedByEveryReadingCommand(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"trip.srt": "cli/trip.srt", "shop.vtt": "cli/shop.vtt"})
	tests := []struct {
		name  string
		words []string
		want  int
	}{
		{"info", []string{"info", "trip.srt", "--word-timestamps"}, 0},
		{"validate", []string{"validate", "trip.srt", "--preset", "bbc", "--word-timestamps"}, exitResult},
		{"sync", []string{"sync", "trip.srt", "--reference", "shop.vtt", "-o", "-", "--word-timestamps"}, 0},
		{"diff", []string{"diff", "trip.srt", "trip.srt", "--word-timestamps"}, 0},
		{"dual", []string{"dual", "--primary", "trip.srt", "--secondary", "shop.vtt", "-o", "-", "--word-timestamps"}, 0},
		{"hls", []string{"hls", "trip.srt", "--output-dir", "hls", "--word-timestamps"}, 0},
		{"translate", []string{"translate", "trip.srt", "--word-timestamps"}, exitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, tt.words...)
			assert.Equal(t, tt.want, got.Code)
			assert.NotContains(t, got.Stderr, "Unknown option")
		})
	}
}
