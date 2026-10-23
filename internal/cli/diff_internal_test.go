package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// diffDir returns a directory with the CLI fixtures and the two versions of the diff fixtures as v1.srt and v2.srt.
func diffDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	copyFile(t, testfiles.Path(t, "diff/own_original.srt"), filepath.Join(dir, "v1.srt"))
	copyFile(t, testfiles.Path(t, "diff/own_edited.srt"), filepath.Join(dir, "v2.srt"))
	return dir
}

func TestDiffUsageErrors(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "old"), 0o755))
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "old", "trip.srt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "old", "shop.vtt"))
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"no file", []string{"diff"}, "Pass two files, the old one and the new one."},
		{"one file", []string{"diff", "v1.srt"}, "Pass two files, the old one and the new one."},
		{"three files", []string{"diff", "v1.srt", "v2.srt", "trip.srt"}, "Pass two files, the old one and the new one."},
		{
			"standard input as the new file",
			[]string{"diff", "v1.srt", "-"},
			"Standard input works only for the old file. Pass the new file by name.",
		},
		{
			"standard input for both",
			[]string{"diff", "-", "-"},
			"Standard input works only for the old file. Pass the new file by name.",
		},
		{
			"negative time tolerance",
			[]string{"diff", "v1.srt", "v2.srt", "--time-tolerance", "-1"},
			"The option --time-tolerance must not be negative.",
		},
		{
			"time tolerance that is no time",
			[]string{"diff", "v1.srt", "v2.srt", "--time-tolerance", "soon"},
			`The option --time-tolerance needs seconds or a timecode such as 00:01:02.500, got "soon".`,
		},
		{
			"unknown new format",
			[]string{"diff", "trip.srt", "shop.vtt", "--new-from", "docx"},
			`Unknown format "docx". Run "stbox formats" for the list.`,
		},
		{
			"unknown format",
			[]string{"diff", "trip.srt", "shop.vtt", "--from", "docx"},
			`Unknown format "docx". Run "stbox formats" for the list.`,
		},
		{"no keep-going", []string{"diff", "trip.srt", "trip.srt", "--keep-going"}, "Unknown option --keep-going."},
		{
			"a directory with 2 files as the old file",
			[]string{"diff", "old", "trip.srt"},
			"The diff command takes one old file, got 2.",
		},
		{
			"a glob with 2 files as the old file",
			[]string{"diff", "old/*", "trip.srt"},
			"The diff command takes one old file, got 2.",
		},
		{"an unknown encoding", []string{"diff", "trip.srt", "shop.vtt", "--encoding", "CP1125"}, `The encoding "CP1125" is unknown.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, tt.words...)
			assert.Equal(t, exitUsage, got.Code)
			assert.Empty(t, got.Stdout)
			assert.Contains(t, got.Stderr, "Error: "+tt.msg)
			assert.Contains(t, got.Stderr, "Run \"stbox help diff\" for the usage.\n")
		})
	}
}

func TestDiffPrintsTheReportAndExitsWith1(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	got := call{dir: dir}.run(t, "diff", "v1.srt", "v2.srt")

	assert.Equal(t, result{Code: exitResult, Stdout: string(testfiles.Read(t, "diff/own_report.txt"))}, got)
}

func TestDiffOfEqualFilesPrintsNothingAndExitsWith0(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "trip.vtt"), []byte(convertAs(t, "trip.srt", stbox.WebVTT)), 0o600))

	assert.Equal(t, result{}, call{dir: dir}.run(t, "diff", "trip.srt", "trip.vtt"))
	assert.Equal(t, result{}, call{dir: dir}.run(t, "diff", "trip.srt", "trip.srt"))
	assert.Equal(t, result{}, call{dir: dir}.run(t, "diff", "trip.srt", "trip.vtt", "--text-only"))
}

func TestDiffJSONReports(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)
	tests := []struct {
		name   string
		option []string
		golden string
	}{
		{"no option", nil, "own_report.json"},
		{"time tolerance", []string{"--time-tolerance", "00:00:00.5"}, "own_report_time_tolerance.json"},
		{"ignore formatting", []string{"--ignore-formatting"}, "own_report_ignore_formatting.json"},
		{"ignore whitespace", []string{"--ignore-whitespace"}, "own_report_ignore_whitespace.json"},
		{"text only", []string{"--text-only"}, "own_report_text_only.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			words := append([]string{"diff", "v1.srt", "v2.srt", "--json"}, tt.option...)
			got := call{dir: dir}.run(t, words...)

			assert.Equal(t, result{Code: exitResult, Stdout: string(testfiles.Read(t, "diff/"+tt.golden))}, got)
		})
	}
}

func TestDiffJSONOfEqualFiles(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	got := call{dir: dir}.run(t, "diff", "trip.srt", "trip.srt", "--json")

	assert.Equal(t, result{Stdout: `[
    {
        "oldFile": "trip.srt",
        "newFile": "trip.srt",
        "equal": true,
        "differences": [],
        "oldWarnings": [],
        "newWarnings": []
    }
]
`}, got)
}

func TestDiffJSONNamesStandardInputAsDash(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	got := call{dir: dir, stdin: readFile(t, dir, "v1.srt")}.run(t, "diff", "-", "v2.srt", "--json", "--time-tolerance", "00:00:00.5")

	want := string(testfiles.Read(t, "diff/own_report_time_tolerance.json"))
	assert.Equal(t, result{Code: exitResult, Stdout: strings.Replace(want, `"v1.srt"`, `"-"`, 1)}, got)
}

func TestDiffTextFromStandardInput(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	got := call{dir: dir, stdin: readFile(t, dir, "v1.srt")}.run(t, "diff", "-", "v2.srt")

	assert.Equal(t, result{Code: exitResult, Stdout: string(testfiles.Read(t, "diff/own_report.txt"))}, got)
}

func TestDiffFileFailures(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)
	tests := []struct {
		name   string
		words  []string
		stdout string
		stderr string
	}{
		{"missing new file", []string{"diff", "v1.srt", "missing.srt"}, "", "missing.srt: The file does not exist.\n"},
		{"missing old file", []string{"diff", "missing.srt", "v2.srt"}, "", "missing.srt: The file does not exist.\n"},
		{"new file that is a directory", []string{"diff", "v1.srt", "."}, "", ".: The file does not exist.\n"},
		{"broken new file", []string{"diff", "trip.srt", "broken.srt"}, "", brokenMessage + "\n" + lenientHint + "\n"},
		{"broken old file", []string{"diff", "broken.srt", "trip.srt"}, "", brokenMessage + "\n" + lenientHint + "\n"},
		{
			"new file of no format",
			[]string{"diff", "trip.srt", "notes.txt"},
			"",
			"notes.txt: Format detection found no subtitle format. Pass --new-from FORMAT.\n",
		},
		{
			"old file of no format",
			[]string{"diff", "notes.txt", "trip.srt"},
			"",
			"notes.txt: Format detection found no subtitle format. Pass --from FORMAT.\n",
		},
		{
			"new file in the wrong format",
			[]string{"diff", "trip.srt", "shop.vtt", "--new-from", "srt"},
			"",
			"shop.vtt: line 1: Block #0 has no cue number on its first line. The line is \"WEBVTT\".\n" + lenientHint + "\n",
		},
		{
			"broken new file in JSON",
			[]string{"diff", "trip.srt", "broken.srt", "--json"},
			"[]\n",
			brokenMessage + "\n" + lenientHint + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, result{Code: exitFile, Stdout: tt.stdout, Stderr: tt.stderr}, call{dir: dir}.run(t, tt.words...))
		})
	}
}

func TestDiffFailureWinsOverTheResult(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	got := call{dir: dir}.run(t, "diff", "v1.srt", "v2.srt", "--from", "vtt")

	assert.Equal(t, exitFile, got.Code)
	assert.Empty(t, got.Stdout)
}

func TestDiffReadOptionsApplyToBothFiles(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	t.Run("lenient", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "diff", "broken.srt", "broken.srt", "--lenient")
		assert.Equal(t, result{Stderr: brokenMessage + " (skipped)\n" + brokenMessage + " (skipped)\n"}, got)
	})
	t.Run("lenient JSON lists the warnings of both files", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "diff", "broken.srt", "trip.srt", "--lenient", "--json")
		assert.Equal(t, exitResult, got.Code)
		assert.Equal(t, brokenMessage+" (skipped)\n", got.Stderr)
		assert.Contains(t, got.Stdout, `        "oldWarnings": [
            {
                "lineNumber": 6,
                "blockIndex": 1,
`)
		assert.Contains(t, got.Stdout, "        \"newWarnings\": []\n")
	})
	t.Run("encoding", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "diff", "latin1.srt", "latin1.srt", "--encoding", "Windows-1252")
		assert.Equal(t, result{}, got)
	})
	t.Run("a detected encoding is a warning with a hint for each file", func(t *testing.T) {
		t.Parallel()

		warning := "latin1.srt: The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong. (repaired)\n"
		got := call{dir: dir}.run(t, "diff", "latin1.srt", "latin1.srt", "--lenient")
		assert.Equal(t, result{Stderr: warning + encodingHint + "\n" + warning + encodingHint + "\n"}, got)
	})
	t.Run("from", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "diff", "trip.srt", "trip.srt", "--from", "srt")
		assert.Equal(t, result{}, got)
	})
}

func TestDiffNewFromReplacesFromForTheNewFile(t *testing.T) {
	t.Parallel()

	dir := diffDir(t)

	withBoth := call{dir: dir}.run(t, "diff", "trip.srt", "shop.vtt", "--from", "srt", "--new-from", "vtt")
	assert.Equal(t, exitResult, withBoth.Code)
	assert.Empty(t, withBoth.Stderr)
	assert.Contains(t, withBoth.Stdout, "added: new cue 1\n")

	autoDetected := call{dir: dir}.run(t, "diff", "trip.srt", "shop.vtt")
	assert.Equal(t, withBoth, autoDetected)

	fromOnly := call{dir: dir}.run(t, "diff", "trip.srt", "shop.vtt", "--from", "srt")
	assert.Equal(t, exitFile, fromOnly.Code)
	assert.Contains(t, fromOnly.Stderr, "shop.vtt: line 1:")
}

func TestDiffHelp(t *testing.T) {
	t.Parallel()

	want := `Usage: stbox diff <old> <new> [options]

List the added, removed and changed cues between two subtitle files.

The diff pairs cues by time and text, not by cue number. So one added cue does
not shift the rest. Cue numbers start at 1. The exit code is 1 when the files
differ, as with the Unix diff command. The files can have different formats.
--from and --encoding apply to both files. --new-from replaces --from for the
new file.

Options:
  --time-tolerance SECONDS  A larger difference of a start or end time is a
                            timing change. Default: 0.001.
  --ignore-formatting       Compare the text without tags.
  --ignore-whitespace       Compare the text without spaces, tabs and line
                            breaks.
  --text-only               Report no timing changes.
  --json                    Print the differences as JSON: a list with one
                            object for the pair of files.
  --from FORMAT             Input format. Default: the format detected from the
                            content. When detection finds no format, the file
                            extension sets it.
  --encoding NAME           Encoding of input that is not UTF-8, for example
                            Windows-1252. It replaces code page detection. A
                            BOM, valid UTF-8 and UTF-16 input win over it.
  --lenient                 Skip or repair broken cues and print a warning for
                            each.
  --input-fps RATE          Frame rate of a MicroDVD input without a {1}{1}<fps>
                            first line, and of CSV or TSV times in hh:mm:ss:ff.
  --fps RATE                Same as --input-fps.
  --new-from FORMAT         Format of the new file. Default: as for --from.
  -h, --help                Show this help.
`
	got := call{}.run(t, "diff", "--help")
	assert.Equal(t, result{Stdout: want}, got)
	assert.Equal(t, got, call{}.run(t, "help", "diff"))
}
