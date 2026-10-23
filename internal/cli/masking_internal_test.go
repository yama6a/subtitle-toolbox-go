package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/profanity"
)

func maskDir(t *testing.T) string {
	t.Helper()

	return fixtureDir(t, map[string]string{
		"keys.srt":  "profanity/keys.srt",
		"radio.vtt": "profanity/radio.vtt",
		"words.txt": "profanity/words.txt",
	})
}

func applyProfanity(opts profanity.Options, ranges *[]profanity.MuteRange) func(s *stbox.Subtitle) error {
	return func(s *stbox.Subtitle) error {
		report, err := profanity.Apply(s, opts)
		if ranges != nil {
			*ranges = report.MuteRanges
		}
		return err
	}
}

// Ported from testMaskWords of tests/Cli/BinaryEditTest.php of subtitle-toolbox-php.
func TestConvertMaskWords(t *testing.T) {
	t.Parallel()

	dir := maskDir(t)
	words := []string{"damn*", "hell"}
	tests := []struct {
		name   string
		mask   []string
		opts   profanity.Options
		golden string
	}{
		{"stars", nil, profanity.Options{Words: words}, "keys_stars.srt"},
		{"stars by name", []string{"--mask", "STARS"}, profanity.Options{Words: words}, "keys_stars.srt"},
		{"first letter", []string{"--mask", "first-letter"}, profanity.Options{Words: words, Mask: profanity.MaskFirstLetter}, "keys_first_letter.srt"},
		{"remove", []string{"--mask", "remove"}, profanity.Options{Words: words, Mask: profanity.MaskRemove}, "keys_removed.srt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, append([]string{"convert", "keys.srt", "--to", "srt", "-o", "-", "--mask-words", "words.txt"}, tt.mask...)...)
			assert.Equal(t, result{Stdout: string(testfiles.Read(t, "profanity/"+tt.golden))}, got)
			assert.Equal(t, libraryOutput(t, "profanity/keys.srt", stbox.SubRip, stbox.WriteOptions{}, applyProfanity(tt.opts, nil)), got.Stdout)
		})
	}

	assert.Equal(t, result{Code: exitFile, Stderr: "Error: missing.txt: The file does not exist.\n"},
		call{dir: dir}.run(t, "convert", "keys.srt", "--to", "srt", "--mask-words", "missing.txt"))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
	assert.Equal(t, result{Code: exitFile, Stderr: "Error: sub: The file does not exist.\n"},
		call{dir: dir}.run(t, "convert", "keys.srt", "--to", "srt", "--mask-words", "sub", "--keep-going"),
		"a directory is no word file")
}

// Ported from testMuteRanges of tests/Cli/BinaryEditTest.php of subtitle-toolbox-php.
func TestConvertMuteFiles(t *testing.T) {
	t.Parallel()

	dir := maskDir(t)
	var ranges []profanity.MuteRange
	opts := profanity.Options{Words: []string{"damn*", "hell"}, Mask: profanity.MaskNone, Padding: 100 * time.Millisecond}
	want := libraryOutput(t, "profanity/radio.vtt", stbox.WebVTT, stbox.WriteOptions{}, applyProfanity(opts, &ranges))

	got := call{dir: dir}.run(t, "convert", "radio.vtt", "--to", "vtt", "-o", "out.vtt", "--mask-words", "words.txt",
		"--mask", "none", "--mute-edl", "radio.edl", "--mute-filter", "radio.af", "--mute-padding", "00:00:00,1")
	assert.Equal(t, result{Stdout: "radio.vtt -> out.vtt\nradio.vtt -> radio.edl\nradio.vtt -> radio.af\n"}, got)
	assert.Equal(t, want, readFile(t, dir, "out.vtt"))
	assert.Equal(t, profanity.EDL(ranges), readFile(t, dir, "radio.edl"))
	assert.Equal(t, "1.500 2.100 1\n6.200 6.800 1\n7.900 9.100 1\n", readFile(t, dir, "radio.edl"))
	assert.Equal(t, string(testfiles.Read(t, "profanity/radio.af")), readFile(t, dir, "radio.af"))
}

// Ported from testMuteRangesHoldTheTimesAfterTheTimingEdits of tests/Cli/BinaryEditTest.php of
// subtitle-toolbox-php.
func TestConvertMuteFilesHoldTheTimesAfterTheTimingEdits(t *testing.T) {
	t.Parallel()

	dir := maskDir(t)
	var ranges []profanity.MuteRange
	want := libraryOutput(t, "profanity/keys.srt", stbox.SubRip, stbox.WriteOptions{}, all(
		shift(100*time.Second),
		func(s *stbox.Subtitle) error { return s.ExtendShortCues(3*time.Second, 0) },
		applyProfanity(profanity.Options{Words: []string{"damn*", "hell"}}, &ranges),
	))

	got := call{dir: dir}.run(t, "convert", "keys.srt", "--to", "srt", "-o", "out.srt", "--shift", "100",
		"--timing-min-duration", "3", "--mask-words", "words.txt", "--mute-edl", "keys.edl", "--mute-filter", "keys.af")
	assert.Equal(t, result{Stdout: "keys.srt -> out.srt\nkeys.srt -> keys.edl\nkeys.srt -> keys.af\n"}, got)
	assert.Equal(t, "103.400 105.500 1\n108.000 109.100 1\n111.500 116.200 1\n", readFile(t, dir, "keys.edl"))
	assert.Equal(t, profanity.EDL(ranges), readFile(t, dir, "keys.edl"))
	assert.Equal(t, profanity.FFmpegVolumeFilter(ranges)+"\n", readFile(t, dir, "keys.af"))
	assert.Equal(t, want, readFile(t, dir, "out.srt"))
}

func TestConvertMuteFilesWithoutMatches(t *testing.T) {
	t.Parallel()

	dir := maskDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "none.txt"), []byte("heck\n"), 0o600))
	got := call{dir: dir}.run(t, "convert", "keys.srt", "-o", "-", "--mask-words", "none.txt",
		"--mute-edl", "keys.edl", "--mute-filter", "keys.af")
	assert.Equal(t, result{
		Stdout: libraryOutput(t, "profanity/keys.srt", stbox.SubRip, stbox.WriteOptions{}, nil),
		Stderr: "keys.srt -> keys.edl\nkeys.srt -> keys.af\n",
	}, got)
	assert.Empty(t, readFile(t, dir, "keys.edl"))
	assert.Empty(t, readFile(t, dir, "keys.af"))
}

// Ported from testConvertRemovesTheSubtitleOutputWhenTheMuteFileAppears of tests/Cli/OutputRaceTest.php of
// subtitle-toolbox-php.
func TestConvertRemovesTheSubtitleOutputWhenTheMuteFileAppears(t *testing.T) {
	t.Parallel()

	dir := maskDir(t)
	race := func(path string) {
		if path == "radio.edl" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte("written by another process"), 0o600))
		}
	}
	assert.Equal(t,
		result{Code: exitFile, Stdout: "radio.vtt -> new.srt\n", Stderr: "Error: radio.edl exists. Removed the file that this run wrote.\n"},
		call{dir: dir, beforeCreate: race}.run(t, "convert", "radio.vtt", "--to", "srt", "-o", "new.srt", "--mask-words", "words.txt", "--mute-edl", "radio.edl"))
	assert.NoFileExists(t, filepath.Join(dir, "new.srt"))
	assert.Equal(t, "written by another process", readFile(t, dir, "radio.edl"))
}

func TestConvertWordFile(t *testing.T) {
	t.Parallel()

	const cue = "1\n00:00:01,000 --> 00:00:02,000\n"
	tests := []struct {
		name  string
		words string
		text  string
		want  string
	}{
		{"Cyrillic letter with the byte 0x85", "\u0445\u0440\u0435\u043d\n", "\u041a\u0430\u043a\u043e\u0439 \u0445\u0440\u0435\u043d!", "\u041a\u0430\u043a\u043e\u0439 ****!"},
		{"Latin letter with the byte 0x85", "\u00c5s\r\n", "\u00c5s \u00e5s", "** **"},
		{"line endings CR LF, LF and CR", "a\r\nb\nc\rd\r\r\ne", "a b c d e f", "* * * * * f"},
		{"repeated BOM and spaces", "\ufeff\ufeff  hell \t\n\n \n", "hell", "****"},
		{"NEL is no line break", "a\u0085b\n", "a\u0085b a b", "*** a b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "words.txt"), []byte(tt.words), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "in.srt"), []byte(cue+tt.text+"\n"), 0o600))
			assert.Equal(t, result{Stdout: cue + tt.want + "\n"},
				call{dir: dir}.run(t, "convert", "in.srt", "--no-bom", "--mask-words", "words.txt"))
		})
	}
}

// Ported from testMuteRanges and testMuteFilesNeverOverwriteAnotherFileOfTheRun of tests/Cli/BinaryEditTest.php
// and the masking rows of tests/Cli/BinaryEditTest.php and tests/Cli/BinaryGeneralTest.php of subtitle-toolbox-php.
func TestConvertMaskingUsageErrors(t *testing.T) {
	t.Parallel()

	bad := "A profanity word must be a non-empty UTF-8 string with a * only at the end, got "
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"mask beep", []string{"keys.srt", "--to", "srt", "--mask-words", "words.txt", "--mask", "beep"}, `The option --mask must be stars, first-letter, remove or none, got "beep".`},
		{"mask without mask words", []string{"keys.srt", "--to", "srt", "--mask", "stars"}, "Pass --mask-words with --mask."},
		{"mute edl without mask words", []string{"radio.vtt", "--to", "srt", "--mute-edl", "new.edl"}, "Pass --mask-words with --mute-edl."},
		{"mute filter without mask words", []string{"radio.vtt", "--mute-filter", "new.af"}, "Pass --mask-words with --mute-filter."},
		{"mute padding without mask words", []string{"radio.vtt", "--mute-padding", "1"}, "Pass --mask-words with --mute-padding."},
		{"checks in option order", []string{"radio.vtt", "--mute-padding", "1", "--mask", "x"}, "Pass --mask-words with --mask."},
		{"mute edl to standard output", []string{"radio.vtt", "--to", "srt", "-o", "-", "--mask-words", "words.txt", "--mute-edl", "-"}, "The option --mute-edl needs a file path."},
		{"mute filter to standard output", []string{"radio.vtt", "--mask-words", "words.txt", "--mute-filter", "-"}, "The option --mute-filter needs a file path."},
		{"mask before the path check", []string{"radio.vtt", "--mask-words", "words.txt", "--mute-edl", "-", "--mask", "x"}, `The option --mask must be stars, first-letter, remove or none, got "x".`},
		{"negative padding", []string{"radio.vtt", "--mask-words", "words.txt", "--mute-padding=-1"}, "The option --mute-padding must not be negative."},
		{"padding as text", []string{"radio.vtt", "--mask-words", "words.txt", "--mute-padding", "soon"}, `The option --mute-padding needs seconds or a timecode such as 00:01:02.500, got "soon".`},
		{"input checks before the word file", []string{"trip.srt", "shop.vtt", "--to", "srt", "--mask-words", "missing.txt"}, "2 input files need --output-dir DIR. One input file goes to standard output or to -o FILE."},
		{"group checks before the word file", []string{"radio.vtt", "--mask-words", "missing.txt", "--ass-style", "Fontsize=1"}, "Pass --to ass with --ass-style."},
		{"two inputs", []string{"radio.vtt", "keys.srt", "--to", "vtt", "--output-dir", "out", "--mask-words", "words.txt", "--mute-edl", "new.edl"}, "--mute-edl and --mute-filter take one input file, got 2."},
		{"two inputs with the filter", []string{"radio.vtt", "keys.srt", "--to", "vtt", "--output-dir", "out", "--mask-words", "words.txt", "--mute-filter", "new.af"}, "--mute-edl and --mute-filter take one input file, got 2."},
		{"edl exists", []string{"radio.vtt", "--to", "srt", "-o", "new.srt", "--mask-words", "words.txt", "--mute-edl", "keys.srt"}, "The --mute-edl file keys.srt exists. The tool never overwrites a file. Remove it, or pass another output file or directory."},
		{"edl is the filter", []string{"radio.vtt", "--to", "srt", "-o", "out.srt", "--mask-words", "words.txt", "--mute-edl", "radio.mute", "--mute-filter", "radio.mute"}, "The --mute-filter file radio.mute is also the output of --mute-edl."},
		{"edl is the output", []string{"radio.vtt", "--to", "srt", "-o", "out.srt", "--mask-words", "words.txt", "--mute-edl", "out.srt"}, "The --mute-edl file out.srt is also the output of radio.vtt."},
		{"edl is the output in a directory", []string{"radio.vtt", "--to", "srt", "--output-dir", "out", "--mask-words", "words.txt", "--mute-edl", "out/../out/radio.srt"}, "The --mute-edl file out/../out/radio.srt is also the output of radio.vtt."},
		{"edl is the input", []string{"radio.vtt", "--to", "srt", "-o", "out.srt", "--mask-words", "words.txt", "--mute-edl", "radio.vtt"}, "The --mute-edl file radio.vtt is a file that the command reads. Pass another output file or directory."},
		{"filter is the word file", []string{"radio.vtt", "--to", "srt", "-o", "out.srt", "--mask-words", "words.txt", "--mute-filter", "./words.txt"}, "The --mute-filter file ./words.txt is a file that the command reads. Pass another output file or directory."},
		{"output is the word file", []string{"radio.vtt", "--to", "srt", "-o", "list", "--mask-words", "list"}, "The output list is a file that the command reads. Pass another output file or directory."},
		{"empty word file", []string{"radio.vtt", "--mask-words", "empty.txt", "--mute-edl", "new.edl"}, "The profanity word list is empty."},
		{"bad word", []string{"radio.vtt", "--mask-words", "bad.txt"}, bad + "'a*b'."},
		{"bad word after a good one", []string{"radio.vtt", "--mask-words", "late.txt"}, bad + "'*'."},
		{"invalid UTF-8", []string{"radio.vtt", "--mask-words", "latin1.txt"}, bad + "'h\xe9ll'."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := maskDir(t)
			for name, content := range map[string]string{
				"empty.txt": "\ufeff\r\n  \r\n", "bad.txt": "a*b\n", "late.txt": "hell\n*\n", "latin1.txt": "h\xe9ll\n", "list": "hell\n",
			} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
			}
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.msg), call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}
