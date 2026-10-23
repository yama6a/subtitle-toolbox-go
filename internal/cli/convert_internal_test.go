package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/fixing"
	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/hearingimpaired"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/profanity"
)

// fixtureDir returns a new directory with the CLI fixtures and copies of the testdata files in files, which maps a
// file name in the directory to a path below testdata.
func fixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := newDir(t)
	for name, path := range files {
		copyFile(t, testfiles.Path(t, path), filepath.Join(dir, name))
	}
	return dir
}

// libraryOutput reads the file at path below testdata, runs edit on it and writes it as the CLI must.
func libraryOutput(t *testing.T, path string, f stbox.Format, opts stbox.WriteOptions, edit func(s *stbox.Subtitle) error) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, path), stbox.ReadOptions{})
	require.NoError(t, err)
	if edit != nil {
		require.NoError(t, edit(s))
	}
	var buf bytes.Buffer
	require.NoError(t, stbox.Write(&buf, s, f, opts))
	return buf.String()
}

// cueTexts describes each cue by its times and lines, for a comparison with a file that holds the expected cues.
func cueTexts(t *testing.T, content string, f stbox.Format) []string {
	t.Helper()

	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{Format: f})
	require.NoError(t, err)
	out := make([]string, 0, s.Len())
	for _, c := range s.Cues() {
		out = append(out, c.Start.String()+" "+c.End.String()+"\n"+c.Text())
	}
	return out
}

func assertCuesOf(t *testing.T, path, got string, f stbox.Format) {
	t.Helper()

	assert.Equal(t, cueTexts(t, string(testfiles.Read(t, path)), f), cueTexts(t, got, f))
}

func all(edits ...func(s *stbox.Subtitle) error) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error {
		for _, e := range edits {
			if err := e(s); err != nil {
				return err
			}
		}
		return nil
	}
}

func sdh(opts hearingimpaired.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error {
		_, err := hearingimpaired.Apply(s, opts)
		return err
	}
}

func fix(opts fixing.Options) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error {
		_, err := fixing.Apply(s, opts)
		return err
	}
}

func shift(d time.Duration) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error { return s.Shift(d) }
}

func TestConvertRunsTheGroupsInAFixedOrder(t *testing.T) {
	t.Parallel()

	want := libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, all(
		fix(fixing.Options{Language: "en"}),
		sdh(hearingimpaired.Options{KeepParentheses: true}),
		func(s *stbox.Subtitle) error {
			return s.ReplaceText("Too late", "[late] too late", model.ReplaceOptions{})
		},
		func(s *stbox.Subtitle) error { return s.ChangeCase(model.CaseUpper, "en") },
		func(s *stbox.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil },
		func(s *stbox.Subtitle) error { return s.WrapLines(20, 2) },
		func(s *stbox.Subtitle) error { return s.RemoveDuplicateCues(0) },
		shift(-500*time.Millisecond),
		func(s *stbox.Subtitle) error { return s.Scale(1.001) },
		func(s *stbox.Subtitle) error { return s.FixOverlaps(100 * time.Millisecond) },
		func(s *stbox.Subtitle) error { return s.ExtendShortCues(time.Second, 100*time.Millisecond) },
		applyProfanity(profanity.Options{Words: []string{"tickets"}, Mask: profanity.MaskFirstLetter}, nil),
	))
	words := []string{
		"--mask-words", "words.lst", "--mask", "first-letter", "--timing-min-duration", "1", "--timing-fix-overlaps", "--timing-min-gap", "0.1", "--scale", "1.001",
		"--shift", "-0.5", "--structure-merge-duplicates", "--structure-wrap", "--structure-max-cpl", "20",
		"--case", "upper", "--strip-tags", "--replace", "Too late=[late] too late", "--sdh", "--sdh-keep-parentheses",
		"--language", "en", "--errors-fix",
	}
	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "words.lst"), []byte("tickets\n"), 0o600))
	got := call{dir: dir}.run(t, append([]string{"convert", "trip.srt", "--to", "srt", "-o", "-"}, words...)...)
	assert.Equal(t, result{Stdout: want}, got)
	assert.Contains(t, got.Stdout, "\n(SIGHS) [LATE]\nTOO LATE.\n")
	assert.Contains(t, got.Stdout, "\nWE NEED TWO T****** FOR\n")

	reversed := []string{"convert", "trip.srt"}
	for i := len(words) - 1; i >= 0; i-- {
		if strings.HasPrefix(words[i], "--") {
			reversed = append(reversed, words[i])
			continue
		}
		reversed = append(reversed, words[i-1], words[i])
		i--
	}
	assert.Equal(t, got, call{dir: dir}.run(t, reversed...))
}

func TestConvertRunsTextAndTimingEditsInOneCall(t *testing.T) {
	t.Parallel()

	want := libraryOutput(t, "cli/trip.srt", stbox.WebVTT, stbox.WriteOptions{},
		all(sdh(hearingimpaired.Options{}), shift(-500*time.Millisecond), func(s *stbox.Subtitle) error { return s.FixOverlaps(0) }))
	assert.Equal(t, result{Stdout: want},
		runCLI(t, "convert", "trip.srt", "--sdh", "--timing-fix-overlaps", "--shift", "-0.5", "--to", "vtt", "-o", "-"))
	assert.Equal(t, "\ufeffWEBVTT\n\n1\n00:00:00.500 --> 00:00:02.000\n<i>The train leaves at noon.</i>\n\n"+
		"2\n00:00:02.000 --> 00:00:04.500\nWe need two tickets for the long ride to the coast.\n\n"+
		"3\n00:00:05.500 --> 00:00:05.900\nToo late.\n", want)
}

func TestConvertKeepsTheInputFormatWithoutTo(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, shift(2*time.Second))},
		call{dir: dir}.run(t, "convert", "trip.srt", "--shift", "2"))
	assert.Equal(t, result{Stdout: "shop.vtt -> out/shop.vtt\n"}, call{dir: dir}.run(t, "convert", "shop.vtt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "shop.vtt", stbox.WebVTT), readFile(t, dir, "out/shop.vtt"))
}

// Ported from tests/Cli/ConvertGoldensTest.php of subtitle-toolbox-php, the 1.0 rows.
func TestConvertWritesTheFormatOfTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		to   stbox.Format
	}{
		{"srt/real/own_styled.srt", stbox.WebVTT},
		{"vtt/real/own_ytdlp_rolling.vtt", stbox.SubRip},
		{"ass/real/own_aegisub.ass", stbox.SubRip},
		{"ttml/real/bbc_ebu_tt_d.ttml", stbox.SubRip},
		{"sami/real/multi_language.smi", stbox.SAMI},
		{"lrc/real/handwritten-enhanced.lrc", stbox.SubRip},
		{"mpl2/real/subtitle_edit_shape.txt", stbox.SubRip},
		{"tmplayer/real/tmplayer_multiline_bom.txt", stbox.WebVTT},
		{"srt/real/own_styled.srt", stbox.MPL2},
		{"srt/real/own_styled.srt", stbox.TMPlayer},
		{"mpsub/real/mplayer_doc_time.sub", stbox.SubRip},
		{"mpsub/real/mplayer_doc_frames.sub", stbox.WebVTT},
		{"srt/real/own_styled.srt", stbox.MPSub},
		{"qttext/real/quicktime_export_styles.txt", stbox.SubRip},
		{"qttext/real/subtitle_edit_shape.txt", stbox.WebVTT},
		{"srt/real/own_styled.srt", stbox.QuickTimeText},
		{"sbv/real/youtube_studio_lf.sbv", stbox.PlainText},
		{"subviewer/real/subviewer2_crlf.sub", stbox.SubRip},
		{"subviewer/real/subviewer1_delay.sub", stbox.WebVTT},
		{"srt/real/own_styled.srt", stbox.SubViewer},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			name := filepath.Base(tt.path)
			dir := fixtureDir(t, map[string]string{name: tt.path})
			want := libraryOutput(t, tt.path, tt.to, stbox.WriteOptions{}, nil)
			assert.Equal(t, result{Stdout: want}, call{dir: dir}.run(t, "convert", name, "--to", string(tt.to)))
			assert.Equal(t, result{Stdout: name + " -> out." + string(tt.to) + "\n"},
				call{dir: dir}.run(t, "convert", name, "--to", string(tt.to), "-o", "out."+string(tt.to)))
			assert.Equal(t, want, readFile(t, dir, "out."+string(tt.to)))
		})
	}
}

func TestConvertShift(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"ferry.vtt": "editing/own_ferry_drift.vtt", "harbour.vtt": "editing/harbour_tour.vtt"})
	earlier := libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, shift(-2500*time.Millisecond))
	threeOn := libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error {
		from := 3 * time.Second
		return s.ShiftRange(time.Second, &from, nil)
	})
	allShifted := string(testfiles.Read(t, "editing/own_ferry_drift_shifted_all.vtt"))
	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"seconds", []string{"ferry.vtt", "--shift", "-1.5"}, allShifted},
		{"seconds after =", []string{"ferry.vtt", "--shift=-1.5"}, allShifted},
		{"time code", []string{"trip.srt", "--shift", "-00:00:02,500"}, earlier},
		{"time code after =", []string{"trip.srt", "--shift=-00:00:02,500"}, earlier},
		{"shift after", []string{"trip.srt", "--shift", "1", "--shift-after", "00:03"}, threeOn},
		{
			"shift range",
			[]string{"ferry.vtt", "--shift", "2", "--shift-after", "6", "--shift-before", "00:10:00"},
			string(testfiles.Read(t, "editing/own_ferry_drift_shifted.vtt")),
		},
		{
			"shift range in seconds",
			[]string{"ferry.vtt", "--to", "vtt", "--shift", "2", "--shift-after", "6", "--shift-before", "600"},
			string(testfiles.Read(t, "editing/own_ferry_drift_shifted.vtt")),
		},
		{
			"moved cue sorts before earlier cues",
			[]string{"harbour.vtt", "--shift", "-12.5", "--shift-after", "17.5"},
			string(testfiles.Read(t, "editing/harbour_tour_shifted_back.vtt")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...)
			require.Equal(t, 0, got.Code, got.Stderr)
			f := stbox.SubRip
			if strings.HasSuffix(tt.words[0], ".vtt") {
				f = stbox.WebVTT
			}
			assert.Equal(t, cueTexts(t, tt.want, f), cueTexts(t, got.Stdout, f))
		})
	}

	shop := call{dir: dir}.run(t, "convert", "shop.vtt", "--shift", "5", "--shift-after", "12")
	assert.Equal(t, 0, shop.Code)
	assert.Contains(t, shop.Stdout, "\n00:00:10.000 --> 00:00:12.000\n")
	assert.Contains(t, shop.Stdout, "\n00:00:18.000 --> 00:00:20.500\n")
}

func TestConvertTimeLimitIsAUsageError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, usageResult("convert", `The option --shift needs a time below 100000 hours, got "1e300".`),
		runCLI(t, "convert", "missing.srt", "--shift", "1e300"))
	for _, v := range []string{"100000:00:00", "-360000000"} {
		got := runCLI(t, "convert", "missing.srt", "--shift", v)
		assert.Equal(t, exitUsage, got.Code)
		assert.True(t, strings.HasPrefix(got.Stderr, "Error: The option --shift needs "), got.Stderr)
	}
	assert.Equal(t, exitUsage, runCLI(t, "convert", "trip.srt", "--timing-min-duration", "1e300").Code)
}

func TestConvertRetimeBeyondTheTimeLimitFailsTheFile(t *testing.T) {
	t.Parallel()

	s, _, err := stbox.Load(testfiles.Path(t, "cli/trip.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	want := result{Code: exitFile, Stderr: "trip.srt: " + message(s.Scale(1e300)) + "\n"}
	assert.Equal(t, want, runCLI(t, "convert", "trip.srt", "--scale", "1e300"))
	assert.Equal(t, want, runCLI(t, "convert", "trip.srt", "--from-fps", "1", "--to-fps", "1e-300"))
}

func TestConvertSync(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"ferry.vtt": "editing/own_ferry_drift.vtt"})
	firstAndLast := libraryOutput(t, "editing/own_ferry_drift.vtt", stbox.WebVTT, stbox.WriteOptions{}, func(s *stbox.Subtitle) error {
		return s.SyncByPoints([]model.SyncPoint{{Old: 5 * time.Second, New: 12500 * time.Millisecond}, {Old: 1500 * time.Second, New: 6065 * time.Second}})
	})
	shifted := libraryOutput(t, "editing/own_ferry_drift.vtt", stbox.WebVTT, stbox.WriteOptions{}, shift(2*time.Second))
	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"first and last", []string{"--sync", "first=00:00:12.5", "--sync", "last=01:41:05"}, firstAndLast},
		{"cue numbers", []string{"--sync=#1=12.5", "--sync", "#5=6065"}, firstAndLast},
		{"one point", []string{"--sync", "10=12"}, shifted},
		{"one point with to", []string{"--to", "vtt", "--sync", "10=12"}, shifted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, result{Stdout: tt.want}, call{dir: dir}.run(t, append([]string{"convert", "ferry.vtt"}, tt.words...)...))
		})
	}

	got := call{dir: dir}.run(t, "convert", "ferry.vtt", "--sync", "#2=00:12", "--sync", "00:10:00=610", "--sync", "1200=00:20:05,000")
	require.Equal(t, 0, got.Code, got.Stderr)
	assertCuesOf(t, "editing/own_ferry_drift_synced.vtt", got.Stdout, stbox.WebVTT)
}

func TestConvertSyncPointsThatDoNotFitTheFileFailTheFile(t *testing.T) {
	t.Parallel()

	assert.Equal(t, result{Code: exitFile, Stderr: `trip.srt: The --sync point "#4=12" names cue 4, but the subtitle has 3 cues.` + "\n"},
		runCLI(t, "convert", "trip.srt", "--sync", "#4=12"))
	assert.Equal(t, result{Code: exitFile, Stderr: `trip.srt: The --sync points must increase in both times, got "10=12" before "last=20".` + "\n"},
		runCLI(t, "convert", "trip.srt", "--sync", "10=12", "--sync", "last=20"))

	var empty bytes.Buffer
	require.NoError(t, stbox.Write(&empty, &stbox.Subtitle{}, stbox.JSON, stbox.WriteOptions{}))
	assert.Equal(t, result{Code: exitFile, Stderr: "stdin: The --sync points need a subtitle with cues.\n"},
		call{stdin: empty.String()}.run(t, "convert", "-", "--sync", "first=1"))
}

func TestConvertScaleAndFrameRate(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"ferry.vtt": "editing/own_ferry_drift.vtt", "de.srt": "sync/own_target_de_25fps.srt"})

	got := call{dir: dir}.run(t, "convert", "ferry.vtt", "--scale", "1.001")
	require.Equal(t, 0, got.Code, got.Stderr)
	assertCuesOf(t, "editing/own_ferry_drift_scaled.vtt", got.Stdout, stbox.WebVTT)

	got = call{dir: dir}.run(t, "convert", "de.srt", "--from-fps", "25", "--to-fps", "23.976")
	require.Equal(t, 0, got.Code, got.Stderr)
	assertCuesOf(t, "sync/own_target_de_23976fps.srt", got.Stdout, stbox.SubRip)

	frameRate := func(s *stbox.Subtitle) error { return s.ConvertFrameRate(25, 23.976) }
	assert.Equal(t, result{Stdout: libraryOutput(t, "cli/trip.srt", stbox.WebVTT, stbox.WriteOptions{}, frameRate)},
		call{dir: dir}.run(t, "convert", "trip.srt", "--from-fps", "25", "--to-fps", "23.976", "--to", "vtt", "-o", "-"))

	scale := func(s *stbox.Subtitle) error { return s.Scale(1.001) }
	inOrder := libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, all(shift(-1500*time.Millisecond), scale, frameRate))
	reversed := libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, all(frameRate, scale, shift(-1500*time.Millisecond)))
	assert.NotEqual(t, reversed, inOrder)
	assert.Equal(t, result{Stdout: inOrder},
		call{dir: dir}.run(t, "convert", "trip.srt", "--from-fps", "25", "--to-fps", "23.976", "--scale", "1.001", "--shift", "-1.5"))
}

func TestConvertTiming(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{
		"asr.srt": "fixes/own_asr_tight_timing.srt", "long.srt": "fixes/own_long_cues.srt",
		"overlaps.srt": "fixes/own_overlaps_and_short_cues.srt",
	})
	lead := func(in, out, gap time.Duration) func(s *stbox.Subtitle) error {
		return func(s *stbox.Subtitle) error { return s.AddLeadInOut(in, out, gap) }
	}
	asr := func(edit func(s *stbox.Subtitle) error) string {
		return libraryOutput(t, "fixes/own_asr_tight_timing.srt", stbox.SubRip, stbox.WriteOptions{}, edit)
	}
	tests := []struct {
		name  string
		words []string
		want  string
		cues  string
	}{
		{
			"lead-in and lead-out",
			[]string{"asr.srt", "--timing-lead-in", "0.2", "--timing-lead-out", "00:00:00.300", "--timing-min-gap", "0.083"},
			asr(lead(200*time.Millisecond, 300*time.Millisecond, 83*time.Millisecond)), "fixes/own_asr_tight_timing_lead.srt",
		},
		{"lead-out alone", []string{"asr.srt", "--timing-lead-out", "0.5"}, asr(lead(0, 500*time.Millisecond, 0)), ""},
		{"lead-in with a gap", []string{"asr.srt", "--timing-lead-in", "0.5", "--timing-min-gap", "0.1"}, asr(lead(500*time.Millisecond, 0, 100*time.Millisecond)), ""},
		{
			"max duration",
			[]string{"long.srt", "--timing-max-duration", "7"},
			libraryOutput(t, "fixes/own_long_cues.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error { return s.LimitLongCues(7 * time.Second) }),
			"fixes/own_long_cues_limited.srt",
		},
		{
			"max duration as a time code",
			[]string{"long.srt", "--timing-max-duration", "00:00:07,000"},
			libraryOutput(t, "fixes/own_long_cues.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error { return s.LimitLongCues(7 * time.Second) }),
			"fixes/own_long_cues_limited.srt",
		},
		{
			"max duration runs last",
			[]string{"asr.srt", "--timing-max-duration", "2.5", "--timing-lead-out", "1", "--timing-min-duration", "3"},
			asr(all(func(s *stbox.Subtitle) error { return s.ExtendShortCues(3*time.Second, 0) }, lead(0, time.Second, 0),
				func(s *stbox.Subtitle) error { return s.LimitLongCues(2500 * time.Millisecond) })), "",
		},
		{
			// Ported from the "timing fixes" row of tests/Cli/ApplicationConvertTest.php of subtitle-toolbox-php.
			"timing fixes",
			[]string{"overlaps.srt", "--timing-fix-overlaps", "--timing-min-duration", "0.833", "--timing-min-gap", "0.083", "--structure-wrap", "--structure-max-cpl", "42"},
			libraryOutput(t, "fixes/own_overlaps_and_short_cues.srt", stbox.SubRip, stbox.WriteOptions{}, all(
				func(s *stbox.Subtitle) error { return s.WrapLines(42, 2) },
				func(s *stbox.Subtitle) error { return s.FixOverlaps(83 * time.Millisecond) },
				func(s *stbox.Subtitle) error { return s.ExtendShortCues(833*time.Millisecond, 83*time.Millisecond) },
			)),
			"fixes/own_overlaps_and_short_cues_fixed.srt",
		},
		{
			"min duration as a time code",
			[]string{"trip.srt", "--to", "srt", "--timing-min-duration", "00:00:01.2"},
			libraryOutput(t, "cli/trip.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error { return s.ExtendShortCues(1200*time.Millisecond, 0) }), "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...)
			assert.Equal(t, result{Stdout: tt.want}, got)
			if tt.cues != "" {
				assertCuesOf(t, tt.cues, got.Stdout, stbox.SubRip)
			}
		})
	}

	wrapped := call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--timing-fix-overlaps", "--timing-min-gap", "0.1",
		"--timing-min-duration", "1", "--structure-wrap", "--structure-max-cpl", "30")
	assert.Equal(t, 0, wrapped.Code)
	assert.Contains(t, wrapped.Stdout, "00:00:01,000 --> 00:00:02,400\n")
	assert.Contains(t, wrapped.Stdout, "[BELL RINGS] ANNA: We need two tickets\nfor the long ride to the coast.\n")
	assert.Contains(t, wrapped.Stdout, "00:00:06,000 --> 00:00:07,000\n")
}

func TestConvertStructure(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{
		"speech.srt": "short-cues/own_speech_to_text.srt", "duplicates.srt": "editing/own_duplicate_cues.srt",
		"turns.srt": "fixes/own_dialogue_turns_wrapped.srt",
	})
	merged := call{dir: dir}.run(t, "convert", "speech.srt", "--to", "srt", "-o", "-", "--structure-merge-short")
	assert.Equal(t, result{Stdout: libraryOutput(t, "short-cues/own_speech_to_text.srt", stbox.SubRip, stbox.WriteOptions{},
		func(s *stbox.Subtitle) error { return s.MergeShortCues(model.MergeShortCuesOptions{}) })}, merged)
	assertCuesOf(t, "short-cues/own_speech_to_text_merged.srt", merged.Stdout, stbox.SubRip)

	narrow := libraryOutput(t, "short-cues/own_speech_to_text.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error {
		return s.MergeShortCues(model.MergeShortCuesOptions{Limits: model.CueLimits{MaxCharactersPerLine: 20, MaxLinesPerCue: 3}})
	})
	assert.Equal(t, result{Stdout: narrow}, call{dir: dir}.run(t, "convert", "speech.srt", "--to", "srt", "-o", "-",
		"--structure-merge-short", "--structure-max-cpl", "20", "--structure-max-lines", "3"))

	sameTime := libraryOutput(t, "editing/own_duplicate_cues.srt", stbox.SubRip, stbox.WriteOptions{},
		func(s *stbox.Subtitle) error { return s.MergeSameTimeCues(0) })
	assert.NotEqual(t, libraryOutput(t, "editing/own_duplicate_cues.srt", stbox.SubRip, stbox.WriteOptions{}, nil), sameTime)
	assert.Equal(t, result{Stdout: sameTime}, call{dir: dir}.run(t, "convert", "duplicates.srt", "--structure-merge-same-time"))

	// The help lists unwrap, merge short, wrap, merge duplicates and merge same time. The run order differs.
	runOrder := libraryOutput(t, "fixes/own_dialogue_turns_wrapped.srt", stbox.SubRip, stbox.WriteOptions{}, all(
		func(s *stbox.Subtitle) error { s.UnwrapLines(); return nil },
		func(s *stbox.Subtitle) error { return s.MergeSameTimeCues(0) },
		func(s *stbox.Subtitle) error {
			return s.MergeShortCues(model.MergeShortCuesOptions{Limits: model.CueLimits{MaxCharactersPerLine: 30}})
		},
		func(s *stbox.Subtitle) error { return s.WrapLines(30, 2) },
		func(s *stbox.Subtitle) error { return s.RemoveDuplicateCues(0) },
	))
	assert.Equal(t, result{Stdout: runOrder}, call{dir: dir}.run(t, "convert", "turns.srt", "--structure-merge-duplicates",
		"--structure-wrap", "--structure-merge-short", "--structure-merge-same-time", "--structure-unwrap", "--structure-max-cpl", "30"))
	unwrapped := libraryOutput(t, "fixes/own_dialogue_turns_wrapped.srt", stbox.SubRip, stbox.WriteOptions{},
		func(s *stbox.Subtitle) error { s.UnwrapLines(); return nil })
	assert.Equal(t, result{Stdout: unwrapped}, call{dir: dir}.run(t, "convert", "turns.srt", "--structure-unwrap"))
}

func TestConvertTextEdits(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{
		"caps.vtt": "transforms/own_cea608_caps.vtt", "multi.srt": "transforms/own_multilingual_caps.srt",
		"rtl.srt": "transforms/own_rtl.srt", "fixed.srt": "transforms/own_rtl_fixed.srt",
	})
	caps, multi, rtl := "transforms/own_cea608_caps.vtt", "transforms/own_multilingual_caps.srt", "transforms/own_rtl.srt"
	changeCase := func(mode model.CaseMode, language string) func(s *stbox.Subtitle) error {
		return func(s *stbox.Subtitle) error { return s.ChangeCase(mode, language) }
	}
	strip := func(s *stbox.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil }
	regex := func(from, to string) func(s *stbox.Subtitle) error {
		return func(s *stbox.Subtitle) error { return s.ReplaceText(from, to, model.ReplaceOptions{Regex: true}) }
	}
	tests := []struct {
		name  string
		words []string
		input string
		edit  func(s *stbox.Subtitle) error
		opts  stbox.WriteOptions
		cues  string
	}{
		{"sentence case", []string{"caps.vtt", "--case", "sentence"}, caps, changeCase(model.CaseSentence, ""), stbox.WriteOptions{}, "transforms/own_cea608_caps_sentence.vtt"},
		{
			"regex replace and strip tags",
			[]string{"caps.vtt", "--replace-regex", "--replace", `/\[[^\]]*\]/=`, "--replace", `/\.{4,}/=...`, "--strip-tags"},
			caps, all(regex(`\[[^\]]*\]`, ""), regex(`\.{4,}`, "..."), strip),
			stbox.WriteOptions{},
			"transforms/own_cea608_caps_cleaned.vtt",
		},
		{"strip tags", []string{"caps.vtt", "--strip-tags"}, caps, strip, stbox.WriteOptions{}, "transforms/own_cea608_caps_stripped.vtt"},
		{"upper case", []string{"multi.srt", "--case", "upper"}, multi, changeCase(model.CaseUpper, ""), stbox.WriteOptions{}, "transforms/own_multilingual_caps_upper.srt"},
		{"lower case in Turkish", []string{"multi.srt", "--case", "lower", "--language", "tr"}, multi, changeCase(model.CaseLower, "tr"), stbox.WriteOptions{}, "transforms/own_multilingual_caps_lower_tr.srt"},
		{
			"replace in any case",
			[]string{"multi.srt", "--replace", "uhr=Uhr", "--replace-ignore-case"},
			multi,
			func(s *stbox.Subtitle) error {
				return s.ReplaceText("uhr", "Uhr", model.ReplaceOptions{IgnoreCase: true})
			},
			stbox.WriteOptions{},
			"transforms/own_multilingual_caps_replaced.srt",
		},
		{
			"regex flag i and back reference",
			[]string{"multi.srt", "--replace-regex", "--replace", `#(u)hr#i=\1HR`},
			multi,
			func(s *stbox.Subtitle) error {
				return s.ReplaceText("(u)hr", "${1}HR", model.ReplaceOptions{Regex: true, IgnoreCase: true})
			},
			stbox.WriteOptions{},
			"",
		},
		{"fix right-to-left", []string{"rtl.srt", "--rtl", "fix"}, rtl, func(s *stbox.Subtitle) error { s.FixRightToLeft(); return nil }, stbox.WriteOptions{}, "transforms/own_rtl_fixed.srt"},
		{
			"clean right-to-left",
			[]string{"fixed.srt", "--no-bom", "--rtl", "clean"},
			"transforms/own_rtl_fixed.srt",
			func(s *stbox.Subtitle) error { s.RemoveBidiControls(); return nil },
			stbox.WriteOptions{BOM: stbox.BOMRemove},
			rtl,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := stbox.SubRip
			if strings.HasSuffix(tt.input, ".vtt") {
				f = stbox.WebVTT
			}
			got := call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...)
			assert.Equal(t, result{Stdout: libraryOutput(t, tt.input, f, tt.opts, tt.edit)}, got)
			if tt.cues != "" {
				assertCuesOf(t, tt.cues, got.Stdout, f)
			}
		})
	}
	assert.NotEqual(t, readFile(t, dir, "multi.srt"), call{dir: dir}.run(t, "convert", "multi.srt", "--replace-regex", "--replace", `#(u)hr#i=\1HR`).Stdout)

	// preg_replace of PHP 8.5 writes this line for the same FROM and TO.
	php := `(sighs) Too [$_|\1|$1|$$||late|late|$&|\x].`
	got := call{dir: dir}.run(t, "convert", "trip.srt", "--replace-regex", "--replace", `/(late)/=[$_|\\1|\$1|$$|\11|$1|${1}|$&|\x]`)
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "\n"+php+"\n")

	// PHP numbers named and unnamed groups in pattern order and writes "[sigh|s]" here.
	got = call{dir: dir}.run(t, "convert", "trip.srt", "--replace-regex", "--replace", `/(?<n>sigh)(s)/=[\1|\2]`, "--replace", `/T\_?oo/=Too`)
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "\n([sigh|s]) Too late.\n")
}

// Ported from testStripSdh of tests/Cli/BinaryEditTest.php, testConvertSdhKeepOption of
// tests/Cli/BinaryOptionCoverageTest.php and the "SDH" row of tests/Cli/ApplicationConvertTest.php of
// subtitle-toolbox-php.
func TestConvertSDH(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"sdh.srt": "hearing-impaired/own_sdh.srt"})
	braces := []hearingimpaired.BracketPair{{Open: "{", Close: "}"}, {Open: "\u00ab", Close: "\u00bb"}}
	tests := []struct {
		name   string
		words  []string
		opts   hearingimpaired.Options
		write  stbox.WriteOptions
		golden string
	}{
		{
			"defaults with CR LF and BOM",
			[]string{"--line-ending", "crlf", "--bom"},
			hearingimpaired.Options{},
			stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMAdd},
			"own_sdh_removed.srt",
		},
		{"keep square brackets", []string{"--sdh-keep-square-brackets"}, hearingimpaired.Options{KeepSquareBrackets: true}, stbox.WriteOptions{}, "own_sdh_removed_keep_square_brackets.srt"},
		{"keep music lines", []string{"--sdh-keep-music-lines"}, hearingimpaired.Options{KeepMusicLines: true}, stbox.WriteOptions{}, "own_sdh_removed_keep_music_lines.srt"},
		{"any case labels", []string{"--sdh-any-case-labels"}, hearingimpaired.Options{AnyCaseLabels: true}, stbox.WriteOptions{}, "own_sdh_removed_any_case_labels.srt"},
		{"keep parentheses", []string{"--sdh-keep-parentheses"}, hearingimpaired.Options{KeepParentheses: true}, stbox.WriteOptions{}, "own_sdh_removed_keep_parentheses.srt"},
		{"keep speaker labels", []string{"--sdh-keep-speaker-labels"}, hearingimpaired.Options{KeepSpeakerLabels: true}, stbox.WriteOptions{}, "own_sdh_removed_keep_speaker_labels.srt"},
		{
			"all options",
			[]string{"--sdh-any-case-labels", "--sdh-brackets", "{}", "--sdh-lyrics", "--sdh-brackets", "\u00ab\u00bb"},
			hearingimpaired.Options{AnyCaseLabels: true, CustomBrackets: braces, Lyrics: true},
			stbox.WriteOptions{},
			"own_sdh_removed_all_options.srt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, append([]string{"convert", "sdh.srt", "--to", "srt", "--sdh"}, tt.words...)...)
			assert.Equal(t, result{Stdout: libraryOutput(t, "hearing-impaired/own_sdh.srt", stbox.SubRip, tt.write, sdh(tt.opts))}, got)
			assert.Equal(t, string(testfiles.Read(t, "hearing-impaired/"+tt.golden)), got.Stdout)
		})
	}

	got := runCLI(t, "convert", "trip.srt", "--sdh", "--to", "vtt", "-o", "-")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "\nWe need two tickets for the long ride to the coast.\n")
	assert.Contains(t, got.Stdout, "\nToo late.\n")
	assert.NotContains(t, got.Stdout, "BELL")
}

// Ported from testForcedOnly of tests/Cli/BinaryEditTest.php of subtitle-toolbox-php, with the TTML fixture.
func TestConvertForcedOnlyOfTTML(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"harbour.ttml": "forced/forced_inheritance.ttml"})
	forced := libraryOutput(t, "forced/forced_inheritance.ttml", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error {
		*s = *s.ForcedCuesOnly()
		return nil
	})
	assert.Equal(t, result{Stdout: forced}, call{dir: dir}.run(t, "convert", "harbour.ttml", "--to", "srt", "-o", "-", "--forced-only"))
	assert.Equal(t, 3, strings.Count(forced, " --> "))
	all := call{dir: dir}.run(t, "convert", "harbour.ttml", "--to", "srt", "-o", "-")
	assert.Equal(t, libraryOutput(t, "forced/forced_inheritance.ttml", stbox.SubRip, stbox.WriteOptions{}, nil), all.Stdout)
	assert.Equal(t, 5, strings.Count(all.Stdout, " --> "))
}

func TestConvertForcedOnly(t *testing.T) {
	t.Parallel()

	s, _, err := stbox.Load(testfiles.Path(t, "cli/trip.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	c, _ := s.Cue(1)
	c.Forced = true
	require.NoError(t, s.SetCue(1, c))
	for s.Len() > 2 {
		require.NoError(t, s.RemoveCue(2))
	}
	var in bytes.Buffer
	require.NoError(t, stbox.Write(&in, s, stbox.JSON, stbox.WriteOptions{}))
	cl := call{stdin: in.String()}

	var both, forced bytes.Buffer
	require.NoError(t, stbox.Write(&both, s, stbox.SubRip, stbox.WriteOptions{}))
	require.NoError(t, stbox.Write(&forced, s.ForcedCuesOnly(), stbox.SubRip, stbox.WriteOptions{}))
	assert.Equal(t, result{Stdout: forced.String()}, cl.run(t, "convert", "-", "--from", "json", "--to", "srt", "-o", "-", "--forced-only"))
	assert.Equal(t, 1, strings.Count(forced.String(), " --> "))
	assert.Equal(t, result{Stdout: both.String()}, cl.run(t, "convert", "-", "--from", "json", "--to", "srt", "-o", "-"))
	assert.Equal(t, 2, strings.Count(both.String(), " --> "))
}

func TestConvertReadAndWriteOptions(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	latin := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "Windows-1252", "--to", "vtt", "-o", "-")
	assert.Equal(t, 0, latin.Code)
	assert.True(t, strings.HasPrefix(latin.Stdout, "\ufeffWEBVTT\n"), latin.Stdout)
	assert.Contains(t, latin.Stdout, "\nCafé au lait.\n")

	s, _, err := stbox.Load(filepath.Join(dir, "latin1.srt"), stbox.ReadOptions{Encoding: "UTF-8"})
	require.NoError(t, err)
	writeErr := stbox.Write(&bytes.Buffer{}, s, stbox.JSON, stbox.WriteOptions{})
	_, ok := errors.AsType[*stbox.UnwritableError](writeErr)
	require.True(t, ok, writeErr)
	unwritable := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "UTF-8", "--to", "json", "-o", "-")
	assert.Equal(t, exitFile, unwritable.Code)
	assert.Empty(t, unwritable.Stdout)
	assert.True(t, strings.HasSuffix(unwritable.Stderr, "latin1.srt: "+message(writeErr)+"\n"), unwritable.Stderr)

	ttmlErr := stbox.Write(&bytes.Buffer{}, s, stbox.TTML, stbox.WriteOptions{})
	uerr, ok := errors.AsType[*stbox.UnwritableError](ttmlErr)
	require.True(t, ok, ttmlErr)
	require.ErrorIs(t, uerr, stbox.ErrInvalidUTF8)
	assert.Equal(t, stbox.TTML, uerr.Format)
	ttml := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "UTF-8", "--to", "ttml", "-o", "-")
	assert.Equal(t, exitFile, ttml.Code)
	assert.Empty(t, ttml.Stdout)
	assert.True(t, strings.HasSuffix(ttml.Stderr, "latin1.srt: "+message(ttmlErr)+"\n"), ttml.Stderr)

	bom := call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "json", "-o", "-", "--bom")
	assert.Equal(t, result{Stdout: libraryOutput(t, "cli/shop.vtt", stbox.JSON, stbox.WriteOptions{BOM: stbox.BOMAdd}, nil)}, bom)
	assert.True(t, strings.HasPrefix(bom.Stdout, "\ufeff{"))

	assert.Equal(t, result{Stdout: "trip.srt -> trip.bak\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.bak"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "trip.bak"))
}

func TestConvertPrintsTheDroppedColorsOfWebVTT(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"ferry.srt": "srt/real/own_font_colors.srt"})
	s, _, err := stbox.Load(filepath.Join(dir, "ferry.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	var want bytes.Buffer
	report, err := webvtt.WriteWithReport(&want, s, model.CommonWriteOptions{LineEnding: stbox.CRLF}, webvtt.WriteOptions{})
	require.NoError(t, err)
	require.Len(t, report.DroppedColors, 1)

	line := `ferry.srt: Cue #3 at 4 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.` + "\n"
	assert.Equal(t, result{Stdout: want.String(), Stderr: line},
		call{dir: dir}.run(t, "convert", "ferry.srt", "--to", "vtt", "-o", "-", "--line-ending", "crlf"))
	assert.Equal(t, result{Stdout: libraryOutput(t, "srt/real/own_font_colors.srt", stbox.WebVTT, stbox.WriteOptions{}, nil), Stderr: line},
		call{dir: dir}.run(t, "convert", "ferry.srt", "--to", "vtt", "-o", "-"))
	assert.Equal(t, result{Stdout: "ferry.srt -> out/ferry.vtt\n", Stderr: line},
		call{dir: dir}.run(t, "convert", "ferry.srt", "--to", "vtt", "--output-dir", "out"))
}

func TestConvertUsageErrors(t *testing.T) {
	t.Parallel()

	trip := []string{"convert", "trip.srt", "--to", "srt", "-o", "-"}
	timeMsg := "needs seconds or a timecode such as 00:01:02.500, got"
	wrapLimits := "Pass --structure-wrap or --structure-merge-short with"
	minGap := "Pass --timing-fix-overlaps, --timing-min-duration, --timing-lead-in or --timing-lead-out with --timing-min-gap."
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"sdh lyrics without sdh", []string{"--sdh-lyrics"}, "Pass --sdh with --sdh-lyrics."},
		{"sdh brackets without sdh", []string{"--sdh-brackets", "{}"}, "Pass --sdh with --sdh-brackets."},
		{"sdh keep square brackets without sdh", []string{"--sdh-keep-square-brackets"}, "Pass --sdh with --sdh-keep-square-brackets."},
		{"sdh keep parentheses without sdh", []string{"--sdh-keep-parentheses"}, "Pass --sdh with --sdh-keep-parentheses."},
		{"sdh keep speaker labels without sdh", []string{"--sdh-keep-speaker-labels"}, "Pass --sdh with --sdh-keep-speaker-labels."},
		{"sdh keep music lines without sdh", []string{"--sdh-keep-music-lines"}, "Pass --sdh with --sdh-keep-music-lines."},
		{"sdh any case labels without sdh", []string{"--sdh-any-case-labels"}, "Pass --sdh with --sdh-any-case-labels."},
		{"sdh checks in option order", []string{"--sdh-brackets", "{}", "--sdh-lyrics"}, "Pass --sdh with --sdh-lyrics."},
		{"sdh brackets of one character", []string{"--sdh", "--sdh-brackets", "{"}, `The option --sdh-brackets needs two characters, an opening and a closing one, got "{".`},
		{"sdh brackets of three characters", []string{"--sdh", "--sdh-brackets", "{}}"}, `The option --sdh-brackets needs two characters, an opening and a closing one, got "{}}".`},
		{"sdh brackets empty", []string{"--sdh", "--sdh-brackets="}, `The option --sdh-brackets needs two characters, an opening and a closing one, got "".`},
		{"sdh brackets of invalid UTF-8", []string{"--sdh", "--sdh-brackets", "{\xff"}, "The option --sdh-brackets needs two characters, an opening and a closing one, got \"{\xff\"."},
		{"sdh before replace", []string{"--replace", "x", "--sdh-lyrics"}, "Pass --sdh with --sdh-lyrics."},
		{"replace without =", []string{"--replace", "colour"}, `The option --replace needs FROM=TO, got "colour".`},
		{"replace with empty FROM", []string{"--replace", "=x"}, `The option --replace needs FROM=TO, got "=x".`},
		{"replace regex invalid", []string{"--replace-regex", "--replace", "/(/=x"}, `The option --replace has the invalid regular expression "/(/".`},
		{"replace regex bad flag", []string{"--replace-regex", "--replace", "/a/U=x"}, `The option --replace has the invalid regular expression "/a/U". The flag U is not supported.`},
		{"replace checks each pair in order", []string{"--replace-regex", "--replace", "a=b", "--replace", "x"}, `The option --replace has the invalid regular expression "a".`},
		{"replace shape before pattern", []string{"--replace-regex", "--replace", "/a/=b", "--replace", "x"}, `The option --replace needs FROM=TO, got "x".`},
		{"replace regex alone", []string{"--replace-regex"}, "Pass --replace with --replace-regex."},
		{"replace regex before ignore case", []string{"--replace-ignore-case", "--replace-regex"}, "Pass --replace with --replace-regex."},
		{"replace ignore case alone", []string{"--replace-ignore-case"}, "Pass --replace with --replace-ignore-case."},
		{"case title", []string{"--case", "title"}, `The option --case must be upper, lower or sentence, got "title".`},
		{"case before rtl", []string{"--rtl", "left", "--case", "title"}, `The option --case must be upper, lower or sentence, got "title".`},
		{"rtl left", []string{"--rtl", "left"}, `The option --rtl must be fix or clean, got "left".`},
		{"language alone", []string{"--language", "tr"}, "Pass --case or --errors-fix with --language."},
		{"language before groups", []string{"--language", "tr", "--replace-regex"}, "Pass --case or --errors-fix with --language."},
		{"output options before language", []string{"--language", "tr", "--line-ending", "cr"}, `The option --line-ending must be lf or crlf, got "cr".`},
		{"regex", []string{"--regex", "--replace", "a=b"}, "Unknown option --regex."},
		{"case language", []string{"--case-language", "tr"}, "Unknown option --case-language."},
		{"karaoke tag", []string{"--karaoke-tag", "k"}, "Unknown option --karaoke-tag."},
		{"max cpl 0", []string{"--structure-wrap", "--structure-max-cpl", "0"}, `The option --structure-max-cpl needs a whole number of 1 or more, got "0".`},
		{"max cpl 0 with merge short", []string{"--structure-merge-short", "--structure-max-cpl", "0"}, `The option --structure-max-cpl needs a whole number of 1 or more, got "0".`},
		{"max lines x", []string{"--structure-wrap", "--structure-max-lines", "x"}, `The option --structure-max-lines needs a whole number of 1 or more, got "x".`},
		{"max cpl without wrap", []string{"--structure-max-cpl", "30"}, wrapLimits + " --structure-max-cpl."},
		{"max cpl with unwrap", []string{"--structure-max-cpl", "30", "--structure-unwrap"}, wrapLimits + " --structure-max-cpl."},
		{"max lines without wrap", []string{"--structure-max-lines", "1", "--timing-fix-overlaps"}, wrapLimits + " --structure-max-lines."},
		{"max cpl rule before max lines rule", []string{"--structure-max-lines", "1", "--structure-max-cpl", "1"}, wrapLimits + " --structure-max-cpl."},
		{"min gap without fix", []string{"--timing-min-gap", "-1"}, minGap},
		{"min gap with only wrap", []string{"--timing-min-gap", "0.1", "--structure-wrap"}, minGap},
		{"min gap as frames", []string{"--timing-fix-overlaps", "--timing-min-gap", "00:00:01:12"}, `The option --timing-min-gap ` + timeMsg + ` "00:00:01:12".`},
		{"lead-in negative", []string{"--timing-lead-in=-0.2"}, "The option --timing-lead-in must not be negative."},
		{"lead-out soon", []string{"--timing-lead-out", "soon"}, `The option --timing-lead-out ` + timeMsg + ` "soon".`},
		{"min duration negative", []string{"--timing-min-duration", "-00:00:01"}, "The option --timing-min-duration must be greater than 0."},
		{"max duration 0", []string{"--timing-max-duration", "0"}, "The option --timing-max-duration must be greater than 0."},
		{"shift soon", []string{"--shift", "soon"}, `The option --shift ` + timeMsg + ` "soon".`},
		{"shift five parts", []string{"--shift", "1:2:3:4:5"}, `The option --shift ` + timeMsg + ` "1:2:3:4:5".`},
		{"shift after alone", []string{"--shift-after", "1"}, "Pass --shift with --shift-after."},
		{"shift before without shift", []string{"--scale", "2", "--shift-before", "600"}, "Pass --shift with --shift-before."},
		{"shift before not after shift after", []string{"--shift", "2", "--shift-after", "600", "--shift-before", "600"}, "The option --shift-before must be after --shift-after."},
		{"scale 0", []string{"--scale", "0"}, "The option --scale must be greater than 0."},
		{"scale nan", []string{"--scale", "NaN"}, `The option --scale needs a number, got "NaN".`},
		{"from fps alone", []string{"--from-fps", "25"}, "Pass --from-fps and --to-fps together."},
		{"to fps alone", []string{"--to-fps", "25"}, "Pass --from-fps and --to-fps together."},
		{"to fps 0", []string{"--from-fps", "25", "--to-fps", "0"}, "The option --to-fps must be greater than 0."},
		{"sync with shift", []string{"--sync", "10=12", "--shift", "2"}, "Pass --sync without --shift and --scale."},
		{"sync with scale", []string{"--sync", "10=12", "--scale", "2"}, "Pass --sync without --shift and --scale."},
		{"sync conflict before the values", []string{"--sync", "x", "--scale", "0"}, "Pass --sync without --shift and --scale."},
		{"sync without =", []string{"--sync", "10"}, `The option --sync needs OLD=NEW, got "10".`},
		{"sync bad old", []string{"--sync", "soon=12"}, `The option --sync ` + timeMsg + ` "soon".`},
		{"sync bad new", []string{"--sync", "first=1:2:3:4:5"}, `The option --sync ` + timeMsg + ` "1:2:3:4:5".`},
		{"sync cue 0", []string{"--sync", "#0=12"}, `The option --sync needs a cue number from 1 after #, got "#0=12".`},
		{"sync cue with 10 digits", []string{"--sync", "#1234567890=12"}, `The option --sync needs a cue number from 1 after #, got "#1234567890=12".`},
		{"sync same old", []string{"--sync", "10=12", "--sync", "10=15"}, `The --sync points must increase in both times, got "10=12" before "10=15".`},
		{"sync old reverse", []string{"--sync", "600=610", "--sync", "10=12"}, `The --sync points must increase in both times, got "600=610" before "10=12".`},
		{"sync new reverse", []string{"--sync", "10=20", "--sync", "first=5", "--sync", "30=15"}, `The --sync points must increase in both times, got "10=20" before "30=15".`},
		{"sync points before the fps pair", []string{"--sync", "x", "--from-fps", "25"}, `The option --sync needs OLD=NEW, got "x".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.msg), call{dir: dir}.run(t, append(trip, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestNeedsOneOf(t *testing.T) {
	t.Parallel()

	opts := []option{flagOption("a", ""), flagOption("b", ""), flagOption("c", ""), flagOption("x", "")}
	a, err := parseArgs([]string{"--x"}, opts)
	require.NoError(t, err)
	for i, want := range []string{"Pass --a with --x.", "Pass --a or --b with --x.", "Pass --a, --b or --c with --x."} {
		assert.Equal(t, usage(want), needsOneOf(a, []string{"a", "b", "c"}[:i+1], "x"), strconv.Itoa(i))
	}
	assert.NoError(t, needsOneOf(a, []string{"a", "x"}, "x"))
	assert.Equal(t, usage("Pass --a with --x."), needs(a, "a", "b", "x"))
}

// Ports the ASS options of tests/Cli/BinaryEditTest.php and tests/Cli/BinaryGeneralTest.php of subtitle-toolbox-php.
func TestConvertASSOptions(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{
		"words.vtt": "vtt/real/w3c_timestamps.vtt", "aegisub.ass": "ass/real/own_aegisub.ass", "song.lrc": "lrc/real/handwritten-enhanced.lrc",
	})
	fill := libraryOutput(t, "vtt/real/w3c_timestamps.vtt", stbox.ASS, stbox.WriteOptions{ASS: ass.WriteOptions{KaraokeTag: ass.KaraokeFill}}, nil)
	assert.Contains(t, fill, `\kf`)
	assert.Equal(t, result{Stdout: fill}, call{dir: dir}.run(t, "convert", "words.vtt", "--to", "ass", "-o", "-", "--ass-karaoke-tag", "kf"))
	assert.Equal(t, result{Stdout: fill}, call{dir: dir}.run(t, "convert", "words.vtt", "--to", "ssa", "-o", "-", "--ass-karaoke-tag", "KF"))
	lyrics := libraryOutput(t, "lrc/real/handwritten-enhanced.lrc", stbox.ASS, stbox.WriteOptions{ASS: ass.WriteOptions{KaraokeTag: ass.KaraokeFill}}, nil)
	assert.Contains(t, lyrics, `{\kf55}Slow `)
	assert.Equal(t, result{Stdout: lyrics}, call{dir: dir}.run(t, "convert", "song.lrc", "--to", "ass", "-o", "-", "--ass-karaoke-tag", "kf"))

	styled := call{dir: dir}.run(t, "convert", "trip.srt", "--to", "ass", "-o", "-", "--ass-style", "Fontname=Roboto,Fontsize=48")
	assert.Equal(t, 0, styled.Code)
	assert.Contains(t, styled.Stdout, "Style: Default,Roboto,48,")

	bogus := `The option --ass-style is not valid. The ASS style field "Bogus" does not exist. Use one of: Fontname, Fontsize, ` +
		"PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, " +
		"Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding."
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"unknown karaoke tag", []string{"trip.srt", "--to", "ass", "-o", "-", "--ass-karaoke-tag", "kx"}, `The option --ass-karaoke-tag must be k, kf or ko, got "kx".`},
		{"unknown style field", []string{"trip.srt", "--to", "ass", "-o", "-", "--ass-style", "Bogus=1"}, bogus},
		{"empty style", []string{"trip.srt", "--to", "ass", "-o", "-", "--ass-style", ""}, `The option --ass-style is not valid. The ASS style "" must hold Field=Value pairs such as Fontsize=48, got "".`},
		{"the choice comes before the style", []string{"trip.srt", "--to", "srt", "--ass-karaoke-tag", "x", "--ass-style", "Bogus=1"}, `The option --ass-karaoke-tag must be k, kf or ko, got "x".`},
		{"the style comes before --to", []string{"trip.srt", "--to", "srt", "--ass-style", "Bogus=1"}, bogus},
		{"style with --to srt", []string{"trip.srt", "--to", "srt", "-o", "-", "--ass-style", "Fontsize=48"}, "Pass --to ass with --ass-style."},
		{"karaoke tag with --output-dir", []string{"trip.srt", "--to", "srt", "--ass-karaoke-tag", "kf", "--output-dir", "out"}, "Pass --to ass with --ass-karaoke-tag."},
		{"karaoke tag without --to", []string{"trip.srt", "-o", "trip2.srt", "--ass-karaoke-tag", "kf"}, "Pass --to ass with --ass-karaoke-tag."},
		{"an ASS input without --to", []string{"aegisub.ass", "-o", "-", "--ass-style", "Fontsize=48"}, "Pass --to ass with --ass-style."},
		{"both options", []string{"trip.srt", "--to", "srt", "-o", "-", "--ass-karaoke-tag", "kf", "--ass-style", "Fontsize=48"}, "Pass --to ass with --ass-karaoke-tag and --ass-style."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.msg), call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

// Ports the "same time" row of tests/Cli/ApplicationConvertTest.php of subtitle-toolbox-php.
func TestConvertMergeSameTimeASSEvents(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"speakers.ass": "editing/own_same_time_speakers.ass"})
	want := libraryOutput(t, "editing/own_same_time_speakers.ass", stbox.SubRip, stbox.WriteOptions{BOM: stbox.BOMRemove},
		func(s *stbox.Subtitle) error { return s.MergeSameTimeCues(0) })
	got := call{dir: dir}.run(t, "convert", "speakers.ass", "--to", "srt", "-o", "-", "--structure-merge-same-time", "--no-bom")
	assert.Equal(t, result{Stdout: want}, got)
	assertCuesOf(t, "editing/own_same_time_speakers_merged.srt", got.Stdout, stbox.SubRip)
}
