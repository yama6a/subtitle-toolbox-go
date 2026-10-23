package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

const (
	existsMessage = "exists. The tool never overwrites a file. Remove it, or pass another output file or directory."
	readsMessage  = "is a file that the command reads. Pass another output file or directory."
)

func realDir(t *testing.T, dir string) string {
	t.Helper()

	realPath, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return realPath
}

func TestConvertWritesTheFormatOfToIntoTheOutputFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "trip.srt -> trip.vtt\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.vtt"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "trip.vtt"))
	assert.Equal(t, result{Stdout: "trip.srt -> new/deep/trip.json\n"},
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", ".JSON", "-o", "new/deep/trip.json"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.JSON), readFile(t, dir, "new/deep/trip.json"))
}

func TestConvertWritesMPL2IntoATxtFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "trip.srt -> trip.txt\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "mpl2", "-o", "trip.txt"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.MPL2), readFile(t, dir, "trip.txt"))
	assert.Equal(t, result{Stdout: "trip.srt -> film.txt\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "tmplayer", "-o", "film.txt"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.TMPlayer), readFile(t, dir, "film.txt"))
	assert.Equal(t, usageResult("convert", "The extension of --output trip.vtt names the format vtt, not the --to format mpl2."),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "mpl2", "-o", "trip.vtt"))
}

func TestConvertWritesMPSubIntoAnMPSubFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "trip.srt -> trip.mpsub\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "mpsub", "-o", "trip.mpsub"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.MPSub), readFile(t, dir, "trip.mpsub"))
	assert.Equal(t, result{Stdout: "trip.srt -> out/trip.mpsub\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "mpsub", "--output-dir", "out"))
	assert.Equal(t, usageResult("convert", "The extension of --output trip.vtt names the format vtt, not the --to format mpsub."),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "mpsub", "-o", "trip.vtt"))
}

func TestConvertWithoutToKeepsTheInputFormat(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: convertAs(t, "trip.srt", stbox.SubRip)}, call{dir: dir}.run(t, "convert", "trip.srt"))
	assert.Equal(t, result{Stdout: "shop.vtt -> out/shop.vtt\n"}, call{dir: dir}.run(t, "convert", "shop.vtt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "shop.vtt", stbox.WebVTT), readFile(t, dir, "out/shop.vtt"))
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "in", "trip.txt"))
	assert.Equal(t, result{Stdout: "in/trip.txt -> out/trip.srt\n"}, call{dir: dir}.run(t, "convert", "in/trip.txt", "--output-dir", "out"))
}

func TestTheOutputExtensionNeverPicksTheFormat(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	before := snapshot(t, dir)
	assert.Equal(t, usageResult("convert", "The extension of --output trip.vtt names the format vtt, not the --to format srt."),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "trip.vtt"))
	assert.Equal(t, usageResult("convert", "The extension of --output trip.ass names the format ass, not the --to format vtt."),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.ass"))
	assert.Equal(t, before, snapshot(t, dir))
	assert.Equal(t, 0, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.bak").Code)
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "trip.bak"))
	assert.Equal(t, 0, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.VTT").Code)
}

func TestWithoutToTheExtensionOfAnotherWritableFormatFailsBeforeAnyFileIsRead(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	before := snapshot(t, dir)
	msg := "The output out.vtt has the extension of the format vtt, not of the input format srt. " +
		"Pass --to vtt to write that format, or use the extension of the input format."
	assert.Equal(t, usageResult("convert", msg), call{dir: dir}.run(t, "convert", "trip.srt", "-o", "out.vtt"))
	assert.Equal(t, usageResult("convert", msg), call{dir: dir}.run(t, "convert", "broken.srt", "-o", "out.vtt"))
	assert.Equal(t, before, snapshot(t, dir))
	assert.Equal(t, result{Code: exitFile, Stderr: "stdin: " + msg + "\n"},
		call{dir: dir, stdin: readFile(t, dir, "trip.srt")}.run(t, "convert", "-", "-o", "out.vtt"))
	assert.Equal(t, before, snapshot(t, dir))

	for _, name := range []string{"kept.srt", "kept.bak", "kept.mpsub"} {
		assert.Equal(t, 0, call{dir: dir}.run(t, "convert", "trip.srt", "-o", name).Code, name)
		assert.FileExists(t, filepath.Join(dir, name))
	}
}

func TestConvertNeverOverwritesAFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "trip.vtt"), []byte("old"), 0o600))
	assert.Equal(t, usageResult("convert", "The output trip.vtt "+existsMessage),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "trip.vtt"))
	assert.Equal(t, "old", readFile(t, dir, "trip.vtt"))
	assert.Equal(t, usageResult("convert", "The output trip.srt "+readsMessage),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "srt", "-o", "trip.srt"))
	require.NoError(t, os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "dangling.vtt")))
	assert.Equal(t, usageResult("convert", "The output dangling.vtt "+existsMessage),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "dangling.vtt"))
}

func TestAnOutputThroughAMissingDirectoryIsCheckedAtThePathItNames(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.vtt"), []byte("old"), 0o600))
	assert.Equal(t, usageResult("convert", "The output new/../keep.vtt "+existsMessage),
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "new/../keep.vtt"))
	assert.Equal(t, "old", readFile(t, dir, "keep.vtt"))
	require.NoError(t, os.Remove(filepath.Join(dir, "keep.vtt")))
	assert.Equal(t, result{Stdout: "trip.srt -> new/../keep.vtt\n"},
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "new/../keep.vtt"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "keep.vtt"))
	assert.NoDirExists(t, filepath.Join(dir, "new"))
}

func TestOutputPathChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output []string
		want   string
	}{
		{"slash", []string{"-o", "newdir/"}, "The output newdir/ ends with a slash. Pass a file name, or pass --output-dir newdir/."},
		{"dot in a directory", []string{"-o", "newdir/."}, `The output newdir/. ends with ".". Pass a file name, or pass --output-dir newdir/..`},
		{"double dot in a directory", []string{"-o", "newdir/.."}, `The output newdir/.. ends with "..". Pass a file name, or pass --output-dir newdir/...`},
		{"dot", []string{"-o", "."}, `The output . ends with ".". Pass a file name, or pass --output-dir ..`},
		{"double dot", []string{"-o", ".."}, `The output .. ends with "..". Pass a file name, or pass --output-dir ...`},
		{"directory", []string{"-o", "out"}, "The output out is a directory. Pass --output-dir out."},
		{"output dir is a file", []string{"--output-dir", "trip.srt"}, "The --output-dir trip.srt is a file. Pass a directory."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			require.NoError(t, os.Mkdir(filepath.Join(dir, "out"), 0o755))
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("convert", tt.want),
				call{dir: dir}.run(t, append([]string{"convert", "trip.srt", "--to", "srt"}, tt.output...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestOneInputGoesToStandardOutputOrToTheOutputFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	before := snapshot(t, dir)
	srt := convertAs(t, "shop.vtt", stbox.SubRip)
	assert.Equal(t, result{Stdout: srt}, call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "srt"))
	assert.Equal(t, result{Stdout: srt}, call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "srt", "-o", "-"))
	assert.Equal(t, before, snapshot(t, dir))
	assert.Equal(t, result{Stdout: "shop.vtt -> out.srt\n"}, call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "srt", "-o", "out.srt"))
	assert.Equal(t, srt, readFile(t, dir, "out.srt"))
	assert.Equal(t, result{Stderr: "stdin -> new/stdin.srt\n"},
		call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "convert", "-", "--to", "srt", "-o", "new/stdin.srt"))
	assert.Equal(t, srt, readFile(t, dir, "new/stdin.srt"))
}

func TestSeveralInputsNeedAnOutputDirectory(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "season1", "trip.srt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "season1", "shop.vtt"))
	before := snapshot(t, dir)
	assert.Equal(t, usageResult("convert", "2 input files need --output-dir DIR. One input file goes to standard output or to -o FILE."),
		call{dir: dir}.run(t, "convert", "season1/shop.vtt", "season1/trip.srt", "--to", "srt"))
	assert.Equal(t, usageResult("convert", "--output takes one input file, got 2. Pass --output-dir DIR for several files."),
		call{dir: dir}.run(t, "convert", "season1/*", "--to", "srt", "-o", "both.srt"))
	assert.Equal(t, usageResult("convert", "The output season1/shop.vtt "+readsMessage),
		call{dir: dir}.run(t, "convert", "season1/trip.srt", "season1/shop.vtt", "--to", "vtt", "--output-dir", "season1"))
	assert.Equal(t, before, snapshot(t, dir))
	assert.Equal(t,
		result{Stdout: "season1/shop.vtt -> out/shop.srt\nseason1/trip.srt -> out/trip.srt\n2 files: 2 succeeded, 0 failed.\n"},
		call{dir: dir}.run(t, "convert", "season1/shop.vtt", "season1/trip.srt", "--to", "srt", "--output-dir", "out"))
}

func TestTwoInputsWithOneOutputFailBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "a", "trip.srt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "b", "trip.vtt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "b", "trip.txt"))
	assert.Equal(t, usageResult("convert", "a/trip.srt and b/trip.vtt would both write out/trip.vtt. Pass them in two runs."),
		call{dir: dir}.run(t, "convert", "a/trip.srt", "b/trip.vtt", "--to", "vtt", "--output-dir", "out"))
	assert.Equal(t, usageResult("convert", "trip.srt and b/trip.txt would both write out/trip.srt. Pass them in two runs."),
		call{dir: dir}.run(t, "convert", "trip.srt", "b/trip.txt", "--to", "srt", "--output-dir", "out"))
	assert.Equal(t, usageResult("convert", "a/trip.srt and trip.srt would both write out/trip.srt. Pass them in two runs."),
		call{dir: dir}.run(t, "convert", "a/trip.srt", "trip.srt", "--output-dir", "out"))
	assert.NoDirExists(t, filepath.Join(dir, "out"))
	assert.Equal(t, result{Stdout: "trip.srt -> out/trip.vtt\n"},
		call{dir: dir}.run(t, "convert", "trip.srt", "./trip.srt", "a/../trip.srt", "--to", "vtt", "--output-dir", "out"))
}

func TestStandardInputTakesNoOutputDir(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	want := usageResult("convert", "Standard input has no file name for --output-dir. Pass -o FILE.")
	assert.Equal(t, want, call{dir: dir}.run(t, "convert", "-", "--to", "vtt", "--output-dir", "out"))
	assert.Equal(t, want, call{dir: dir}.run(t, "convert", "trip.srt", "-", "--output-dir", "out"))
	assert.NoDirExists(t, filepath.Join(dir, "out"))
}

func TestAnOutputThatCannotBeCreatedFailsBeforeAnyWrite(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "a", "trip.srt"))
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "b", "trip.srt"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "taken.srt"), []byte("old"), 0o600))
	copyFile(t, filepath.Join(dir, "taken.srt"), filepath.Join(dir, "taken", "shop.vtt"))
	before := snapshot(t, dir)
	tests := []struct {
		want  string
		words []string
	}{
		{"2 input files need --output-dir DIR. One input file goes to standard output or to -o FILE.", []string{"trip.srt", "shop.vtt"}},
		{"--output takes one input file, got 2. Pass --output-dir DIR for several files.", []string{"trip.srt", "shop.vtt", "-o", "x.srt"}},
		{"Standard input has no file name for --output-dir. Pass -o FILE.", []string{"-", "--output-dir", "out"}},
		{"The output taken.srt " + existsMessage, []string{"trip.srt", "-o", "taken.srt"}},
		{"The output taken/shop.vtt " + existsMessage, []string{"trip.srt", "shop.vtt", "--output-dir", "taken"}},
		{"a/trip.srt and b/trip.srt would both write out/trip.srt. Pass them in two runs.", []string{"a/trip.srt", "b/trip.srt", "--output-dir", "out"}},
		{"The output shop.vtt " + readsMessage, []string{"shop.vtt", "-o", "shop.vtt"}},
		{"The output ./shop.vtt " + readsMessage, []string{"shop.vtt", "--output-dir", "."}},
		{"The output ./shop.vtt " + readsMessage, []string{"shop.vtt", "--output-dir", "./"}},
	}
	for _, tt := range tests {
		got := call{dir: dir}.run(t, append([]string{"convert"}, tt.words...)...)
		assert.Equal(t, usageResult("convert", tt.want), got, tt.want)
		assert.Equal(t, before, snapshot(t, dir), tt.want)
	}
}

func TestBatchGoesOnWithKeepGoingAndPrintsASummary(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Code:   exitFile,
			Stdout: "latin1.srt -> out/latin1.vtt\ntrip.srt -> out/trip.vtt\n3 files: 2 succeeded, 1 failed.\n",
			Stderr: brokenMessage + "\n" + lenientHint + "\n",
		},
		call{dir: dir}.run(t, "convert", "*.srt", "--to", "vtt", "--output-dir", "out", "--keep-going"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "out/trip.vtt"))
	assert.NoFileExists(t, filepath.Join(dir, "out", "broken.vtt"))
}

func TestBatchStopsAtTheFirstFailureWithoutKeepGoing(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Code:   exitFile,
			Stdout: "3 files: 0 succeeded, 1 failed, 2 skipped.\n",
			Stderr: brokenMessage + "\n" + lenientHint + "\nStopped at the first failure. Pass --keep-going to process the other files.\n",
		},
		call{dir: dir}.run(t, "convert", "broken.srt", "trip.srt", "shop.vtt", "--to", "srt", "--output-dir", "out"))
	assert.NoDirExists(t, filepath.Join(dir, "out"))
}

func TestAMissingInputOfABatchFailsThatInput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	got := call{dir: dir}.run(t, "convert", "trip.srt", "missing.srt", "--to", "srt", "--output-dir", "out", "--keep-going")
	assert.Equal(t, result{
		Code:   exitFile,
		Stdout: "trip.srt -> out/trip.srt\n2 files: 1 succeeded, 1 failed.\n",
		Stderr: "missing.srt: The file does not exist.\n",
	}, got)
}

func TestBlockedDirectory(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blocker"), []byte("a file, not a directory"), 0o600))
	assert.Equal(t,
		result{Code: exitFile, Stderr: "Error: Cannot create the directory " + filepath.Join(realDir(t, dir), "blocker", "out") + ".\n"},
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "blocker/out/trip.vtt"))
}

func TestBatchRemovesTheOutputsOfEarlierInputsWhenAnOutputAppears(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	for _, name := range []string{"a", "b", "c"} {
		copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, name+".srt"))
	}
	race := func(path string) {
		if path == "out/b.vtt" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte("written by another process"), 0o600))
		}
	}
	assert.Equal(t,
		result{Code: exitFile, Stdout: "a.srt -> out/a.vtt\n", Stderr: "Error: out/b.vtt exists. Removed the file that this run wrote.\n"},
		call{dir: dir, beforeCreate: race}.run(t, "convert", "a.srt", "b.srt", "c.srt", "--to", "vtt", "--output-dir", "out", "--keep-going"))
	entries, err := os.ReadDir(filepath.Join(dir, "out"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "b.vtt", entries[0].Name())
	assert.Equal(t, "written by another process", readFile(t, dir, "out/b.vtt"))
}

func TestInterruptKeepsTheFinishedOutputs(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stop := func(string) { cancel() }
	assert.Equal(t,
		result{
			Code:   exitInterrupt,
			Stdout: "trip.srt -> out/trip.vtt\n2 files: 1 succeeded, 0 failed, 1 skipped.\n",
			Stderr: "Stopped by a signal.\n",
		},
		call{dir: dir, ctx: ctx, beforeCreate: stop}.run(t, "convert", "trip.srt", "shop.vtt", "--to", "vtt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "out/trip.vtt"))
	assert.NoFileExists(t, filepath.Join(dir, "out", "shop.vtt"))
}

func TestOutputFilesRace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		paths []string
		racer string
		want  string
		left  []string
	}{
		{"first file", []string{"new/trip.vtt"}, "new/trip.vtt", "new/trip.vtt exists.", []string{"new", "new/trip.vtt"}},
		{
			"second file of a batch",
			[]string{"out/trip.vtt", "out/shop.vtt"},
			"out/shop.vtt",
			"out/shop.vtt exists. Removed the file that this run wrote.",
			[]string{"out", "out/shop.vtt"},
		},
		{
			"file outside the directory",
			[]string{"new/radio.srt", "new/radio.edl", "radio.af"},
			"radio.af",
			"radio.af exists. Removed the 2 files that this run wrote.",
			[]string{"radio.af"},
		},
		{
			"nested directories",
			[]string{"hls/deep/subs0.vtt", "hls/deep/subs.m3u8"},
			"hls/deep/subs.m3u8",
			"hls/deep/subs.m3u8 exists. Removed the file that this run wrote.",
			[]string{"hls", "hls/deep", "hls/deep/subs.m3u8"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			c := &console{dir: dir, beforeCreate: func(path string) {
				if path == tt.racer {
					require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte("another process"), 0o600))
				}
			}}
			files := &outputFiles{c: c}
			var err error
			for _, p := range tt.paths {
				if err = files.create(p, []byte("subtitle")); err != nil {
					break
				}
			}
			var ff *fileError
			require.ErrorAs(t, err, &ff)
			assert.Equal(t, tt.want, ff.msg)
			var left []string
			require.NoError(t, filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
				if path != dir {
					rel, _ := filepath.Rel(dir, path)
					left = append(left, filepath.ToSlash(rel))
				}
				return err
			}))
			assert.Equal(t, tt.left, left)
			assert.Equal(t, "another process", readFile(t, dir, tt.racer))
		})
	}
}

func TestRealTarget(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	root := realDir(t, dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "link")))
	tests := []struct {
		in   string
		want string
	}{
		{"trip.srt", filepath.Join(root, "trip.srt")},
		{"new.srt", filepath.Join(root, "new.srt")},
		{"new/../keep.vtt", filepath.Join(root, "keep.vtt")},
		{"new/./a/../b.vtt", filepath.Join(root, "new", "b.vtt")},
		{"link/x.vtt", filepath.Join(root, "sub", "x.vtt")},
		{"link/../x.vtt", filepath.Join(root, "x.vtt")},
		{"out/", filepath.Join(root, "out")},
		{".", root},
		{filepath.Join(dir, "a", "b"), filepath.Join(root, "a", "b")},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, (&console{dir: dir}).realTarget(tt.in))
		})
	}
}

func TestTargetNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input, outputDir string
		to               stbox.Format
		want             string
	}{
		{"a.srt", "out", stbox.WebVTT, "out/a.vtt"},
		{"movie.en.srt", "out", stbox.WebVTT, "out/movie.en.vtt"},
		{"movie.ssa", "out", stbox.ASS, "out/movie.ssa"},
		{"movie.srt", "out", stbox.ASS, "out/movie.ass"},
		{"noext", "out", stbox.SubRip, "out/noext.srt"},
		{"in/shop.VTT", "out/", stbox.WebVTT, "out/shop.VTT"},
		{"a.srt", `out\`, stbox.JSON, "out/a.json"},
		{"-", "out", stbox.WebVTT, "-"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			p := &outputPlan{outputDir: tt.outputDir}
			assert.Equal(t, tt.want, p.target(tt.input, tt.to))
		})
	}
	assert.Equal(t, "x.vtt", (&outputPlan{output: "x.vtt"}).target("a.srt", stbox.SubRip))
	assert.Equal(t, "-", (&outputPlan{}).target("a.srt", stbox.SubRip))
}

func TestUnwritableContentFailsThatFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	got := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "UTF-8", "--lenient", "--to", "json", "-o", "-")
	assert.Equal(t, exitFile, got.Code)
	assert.Empty(t, got.Stdout)
	assert.True(t, strings.HasSuffix(got.Stderr, "\nlatin1.srt: JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. "+
		"Read a file in another encoding with its source encoding. Pass --encoding with the source encoding of the file.\n"), got.Stderr)
}

// reportingConvert is a write command without edits, with an encode step that prints a report before the write.
type reportingConvert struct{ out writeOutput }

var _ fileCommand = &reportingConvert{}

func (*reportingConvert) name() string    { return "convert" }
func (*reportingConvert) summary() string { return "Write subtitle files in the format of --to." }
func (*reportingConvert) usage() []string {
	return []string{"<input> [--to FORMAT] [-o FILE] [options]", "<input>... [--to FORMAT] --output-dir DIR [options]"}
}
func (*reportingConvert) details() string { return "" }
func (*reportingConvert) options() []option {
	return slices.Concat(outputOptions("Output format. Default: the input format."), inputOptions(true, writeFPSHelp))
}

func (s *reportingConvert) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, s)
}

func (s *reportingConvert) prepare(a *args, r *runner) error {
	s.out.encode = func(in *input, out io.Writer, sub *stbox.Subtitle, f stbox.Format) error {
		r.c.err(in.label() + ": " + strconv.Itoa(sub.Len()) + " cues as " + string(f) + "\n")
		return stbox.Write(out, sub, f, s.out.opts)
	}
	if err := r.prepareRead(a); err != nil {
		return err
	}
	return s.out.prepare(a, r)
}

func (s *reportingConvert) checkInputs(r *runner, inputs []string) error {
	return s.out.checkInputs(r, inputs, inputs)
}

func (*reportingConvert) loadSideFiles(context.Context, *runner) error { return nil }

func (s *reportingConvert) process(_ context.Context, r *runner, in *input) error {
	return s.out.write(r, in, nil)
}

func (*reportingConvert) finish(*runner) error { return nil }

func TestWriteRunsTheEncodeStepOfTheCommand(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	cl := call{dir: dir, commands: []command{&reportingConvert{}}}
	assert.Equal(t, result{Stdout: "trip.srt -> out.vtt\n", Stderr: "trip.srt: 3 cues as vtt\n"},
		cl.run(t, "convert", "trip.srt", "--to", "vtt", "-o", "out.vtt"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.WebVTT), readFile(t, dir, "out.vtt"))
}
