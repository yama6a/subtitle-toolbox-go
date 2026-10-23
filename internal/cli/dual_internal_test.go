package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// dualDir returns a directory with the CLI fixtures and the dual fixtures as en.srt and de.srt.
func dualDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	copyFile(t, testfiles.Path(t, "dual/station_en.srt"), filepath.Join(dir, "en.srt"))
	copyFile(t, testfiles.Path(t, "dual/station_de.srt"), filepath.Join(dir, "de.srt"))
	return dir
}

func dualGolden(t *testing.T, name string) string {
	t.Helper()

	return string(testfiles.Read(t, "dual/"+name))
}

// Adapted from BinaryDiffDualHlsTest::usageErrors and testDualTakesThePrimaryAndTheSecondaryFileByName.
func TestDualUsageErrors(t *testing.T) {
	t.Parallel()

	dir := dualDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "taken.srt"), []byte("old"), 0o600))
	pair := []string{"--primary", "en.srt", "--secondary", "de.srt"}
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"two positionals", []string{"dual", "en.srt", "de.srt"}, "dual takes no file arguments. Pass --primary FILE and --secondary FILE."},
		{
			"a positional beside --secondary",
			[]string{"dual", "en.srt", "--secondary", "de.srt"},
			"dual takes no file arguments. Pass --primary FILE and --secondary FILE.",
		},
		{"no option", []string{"dual"}, "Pass --primary FILE and --secondary FILE."},
		{"no secondary", []string{"dual", "--primary", "en.srt"}, "Pass --primary FILE and --secondary FILE."},
		{"no primary", []string{"dual", "--secondary", "de.srt"}, "Pass --primary FILE and --secondary FILE."},
		{"a glob with 2 files", []string{"dual", "--primary", "??.srt", "--secondary", "de.srt"}, "--primary takes one file, got 2."},
		{
			"standard input as the secondary file",
			[]string{"dual", "--primary", "en.srt", "--secondary", "-"},
			"Standard input works only for --primary. Pass the secondary file by name.",
		},
		{"an unknown mode", append([]string{"dual", "--mode", "side"}, pair...), `The option --mode must be stack or top-bottom, got "side".`},
		{
			"a secondary alignment of 0",
			append([]string{"dual", "--secondary-alignment", "0"}, pair...),
			`The option --secondary-alignment needs a whole number from 1 to 9, got "0".`,
		},
		{
			"a secondary alignment of 10",
			append([]string{"dual", "--secondary-alignment", "10"}, pair...),
			`The option --secondary-alignment needs a whole number from 1 to 9, got "10".`,
		},
		{
			"an empty secondary style",
			append([]string{"dual", "--secondary-style", ""}, pair...),
			`The secondary style  must be a core markup tag such as i or font color="#ffff00".`,
		},
		{
			"a secondary style that is no core tag",
			append([]string{"dual", "--secondary-style", "em"}, pair...),
			`The secondary style em must be a core markup tag such as i or font color="#ffff00".`,
		},
		{
			"a negative snap tolerance",
			append([]string{"dual", "--snap-tolerance", "-1"}, pair...),
			"The option --snap-tolerance must not be negative.",
		},
		{
			"a snap tolerance that is no time",
			append([]string{"dual", "--snap-tolerance", "soon"}, pair...),
			`The option --snap-tolerance needs seconds or a timecode such as 00:01:02.500, got "soon".`,
		},
		{"--from", append([]string{"dual", "--from", "srt"}, pair...), "Unknown option --from."},
		{"--new-from", append([]string{"dual", "--new-from", "srt"}, pair...), "Unknown option --new-from."},
		{"--keep-going", append([]string{"dual", "--keep-going"}, pair...), "Unknown option --keep-going."},
		{
			"an unknown primary format",
			append([]string{"dual", "--primary-from", "docx"}, pair...),
			`Unknown format "docx". Run "stbox formats" for the list.`,
		},
		{
			"an unknown secondary format",
			append([]string{"dual", "--secondary-from", "docx"}, pair...),
			`Unknown format "docx". Run "stbox formats" for the list.`,
		},
		{
			"the secondary file as the output",
			append([]string{"dual", "-o", "de.srt"}, pair...),
			"The output de.srt is a file that the command reads. Pass another output file or directory.",
		},
		{
			"the primary file as the output",
			append([]string{"dual", "-o", "en.srt"}, pair...),
			"The output en.srt is a file that the command reads. Pass another output file or directory.",
		},
		{
			"the directory of both files as the output",
			append([]string{"dual", "--output-dir", "."}, pair...),
			"The output ./en.srt is a file that the command reads. Pass another output file or directory.",
		},
		{
			"an output that exists",
			append([]string{"dual", "-o", "taken.srt"}, pair...),
			"The output taken.srt exists. The tool never overwrites a file. Remove it, or pass another output file or directory.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := snapshot(t, dir)
			assert.Equal(t, usageResult("dual", tt.msg), call{dir: dir}.run(t, tt.words...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestDualWritesTheStackToStandardOutput(t *testing.T) {
	t.Parallel()

	dir := dualDir(t)
	want := result{Stdout: dualGolden(t, "station_stack.srt")}

	assert.Equal(t, want, call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-style", "i"))
	assert.Equal(t, want, call{dir: dir, stdin: readFile(t, dir, "en.srt")}.run(t, "dual", "--primary", "-", "--secondary", "de.srt", "--secondary-style", "i"))
	assert.Equal(t, want, call{dir: dir}.run(t, "dual", "--secondary-style", "i", "--mode", "STACK", "--secondary", "de.srt", "--primary", "en.srt"))
}

func TestDualWritesToFiles(t *testing.T) {
	t.Parallel()

	t.Run("an output directory takes the base name of the primary file", func(t *testing.T) {
		t.Parallel()

		dir := dualDir(t)

		got := call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-style", "i", "--output-dir", "out")

		assert.Equal(t, result{Stdout: "en.srt -> out/en.srt\n"}, got)
		assert.Equal(t, dualGolden(t, "station_stack.srt"), readFile(t, dir, "out/en.srt"))
	})
	t.Run("--to sets the format", func(t *testing.T) {
		t.Parallel()

		dir := dualDir(t)

		got := call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-style", "i", "--to", "vtt", "-o", "both.vtt")

		assert.Equal(t, result{Stdout: "en.srt -> both.vtt\n"}, got)
		assert.Equal(t, dualGolden(t, "station_stack.vtt"), readFile(t, dir, "both.vtt"))
	})
	t.Run("top-bottom to ASS", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dualDir(t)}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--mode", "top-bottom",
			"--secondary-style", `font color="#ffff00"`, "--to", "ass")

		assert.Equal(t, result{Stdout: dualGolden(t, "station_top_bottom.ass")}, got)
	})
}

func TestDualSnapToleranceAndSecondaryAlignment(t *testing.T) {
	t.Parallel()

	got := call{dir: dualDir(t)}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--mode", "top-bottom",
		"--snap-tolerance", "00:00.5", "--secondary-alignment", "7")

	assert.Equal(t, result{Stdout: dualGolden(t, "station_top_bottom_aligned.srt")}, got)
}

func TestDualReadsEachFileWithItsOwnFormat(t *testing.T) {
	t.Parallel()

	dir := dualDir(t)
	enVTT := call{dir: dir}.run(t, "convert", "en.srt", "--to", "vtt")
	require.Equal(t, 0, enVTT.Code)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "en.txt"), []byte(enVTT.Stdout), 0o600))
	stack := dualGolden(t, "station_stack.srt")

	t.Run("a primary of another format gives that format", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "dual", "--primary", "en.txt", "--secondary", "de.srt", "--secondary-style", "i")

		assert.Equal(t, 0, got.Code, got.Stderr)
		assert.True(t, strings.HasPrefix(got.Stdout, "\ufeffWEBVTT\n"), got.Stdout)
	})
	t.Run("the secondary file does not inherit --primary-from", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "dual", "--primary", "frames.sub", "--primary-from", "microdvd", "--input-fps", "25", "--secondary", "en.srt")

		assert.Equal(t, 0, got.Code, got.Stderr)
		assert.Contains(t, got.Stdout, "Hello from the frames.|The train to Hamburg")
	})
	t.Run("--secondary-from sets the format of the secondary file", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-from", "sbv")

		assert.Equal(t, exitFile, got.Code)
		assert.Empty(t, got.Stdout)
		assert.True(t, strings.HasPrefix(got.Stderr, "de.srt: "), got.Stderr)
	})
	t.Run("--secondary-from srt reads the secondary file as SubRip", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-from", "srt", "--secondary-style", "i")

		assert.Equal(t, result{Stdout: stack}, got)
	})
}

func TestDualFailures(t *testing.T) {
	t.Parallel()

	broken := `broken.srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:03,000 => 00:00:04,000".`
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			"a missing secondary file",
			[]string{"dual", "--primary", "en.srt", "--secondary", "missing.srt"},
			result{Code: exitFile, Stderr: "missing.srt: The file does not exist.\n"},
		},
		{
			"a missing primary file",
			[]string{"dual", "--primary", "missing.srt", "--secondary", "de.srt"},
			result{Code: exitFile, Stderr: "missing.srt: The file does not exist.\n"},
		},
		{
			"a broken secondary file",
			[]string{"dual", "--primary", "en.srt", "--secondary", "broken.srt"},
			result{Code: exitFile, Stderr: broken + "\n" + lenientHint + "\n"},
		},
		{
			"a secondary file that is no subtitle",
			[]string{"dual", "--primary", "en.srt", "--secondary", "notes.txt"},
			result{Code: exitFile, Stderr: "notes.txt: " + unknownFormatAdvice("secondary-from") + "\n"},
		},
		{
			"a primary file that is no subtitle",
			[]string{"dual", "--primary", "notes.txt", "--secondary", "de.srt"},
			result{Code: exitFile, Stderr: "notes.txt: Format detection found no subtitle format. Pass --primary-from FORMAT.\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := dualDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, tt.want, call{dir: dir}.run(t, tt.words...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}

	t.Run("lenient prints the warnings with the path of the secondary file", func(t *testing.T) {
		t.Parallel()

		got := call{dir: dualDir(t)}.run(t, "dual", "--primary", "en.srt", "--secondary", "broken.srt", "--lenient")

		assert.Equal(t, 0, got.Code)
		assert.Equal(t, broken+" (skipped)\n", got.Stderr)
	})
	t.Run("a secondary failure is the failure of the file", func(t *testing.T) {
		t.Parallel()

		dir := dualDir(t)
		got := call{dir: dir}.run(t, "dual", "--primary", "en.srt", "--secondary", "missing.srt", "--output-dir", "out")

		assert.Equal(t, exitFile, got.Code)
		assert.NoDirExists(t, filepath.Join(dir, "out"))
	})
}

func TestDualHelp(t *testing.T) {
	t.Parallel()

	got := call{}.run(t, "help", "dual")

	assert.Equal(t, 0, got.Code)
	assert.True(t, strings.HasPrefix(got.Stdout, "Usage: stbox dual --primary FILE --secondary FILE [options]\n\n"+
		"Merge two subtitles in two languages into one file that shows both.\n"), got.Stdout)
	assert.Regexp(t, `(?s)--secondary-alignment 1-9 .*Default: 8\..*--snap-tolerance SECONDS .*Default: 0\.25\.`, got.Stdout)
	assert.NotContains(t, got.Stdout, "  --from ")
	assert.NotContains(t, got.Stdout, "--keep-going")
	var names []string
	for _, line := range strings.Split(got.Stdout, "\n") {
		if name, ok := strings.CutPrefix(line, "  --primary"); ok || strings.HasPrefix(line, "  --secondary") {
			names = append(names, strings.Fields(strings.TrimPrefix(line+name, "  "))[0])
		}
	}
	assert.Equal(t, []string{"--primary", "--secondary", "--secondary-style", "--secondary-alignment", "--primary-from", "--secondary-from"}, names)
}

func TestDualAppliesTheOutputAndInputOptions(t *testing.T) {
	t.Parallel()

	dir := dualDir(t)
	pair := []string{"dual", "--primary", "en.srt", "--secondary", "de.srt", "--secondary-style", "i"}
	stack := dualGolden(t, "station_stack.srt")
	bom := "\ufeff"
	run := func(extra ...string) result { return call{dir: dir}.run(t, append(slices.Clone(pair), extra...)...) }

	assert.Equal(t, result{Stdout: strings.ReplaceAll(stack, "\n", "\r\n")}, run("--line-ending", "crlf"))
	assert.Equal(t, result{Stdout: strings.TrimPrefix(stack, bom)}, run("--no-bom"))
	assert.Equal(t, result{Stdout: stack}, run("--bom"))
	assert.Equal(t, result{Stdout: stack}, run("--encoding", "UTF-8"))

	frames := run("--to", "microdvd", "--output-fps", "25")
	assert.Equal(t, 0, frames.Code, frames.Stderr)
	assert.True(t, strings.HasPrefix(frames.Stdout, "{25}{100}The train to Hamburg leaves from platform 4.|{y:i}"), frames.Stdout)
	assert.Equal(t, frames, run("--to", "microdvd", "--fps", "25"))
}
