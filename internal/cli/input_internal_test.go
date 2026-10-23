package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

const (
	brokenMessage = `broken.srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:03,000 => 00:00:04,000".`
	bom           = "\xEF\xBB\xBF"
)

func TestUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		{"unknown option", []string{"convert", "trip.srt", "--to", "vtt", "--nope"}, "Unknown option --nope."},
		{"negative number as input", []string{"convert", "-2.5"}, "Unknown option -2.5."},
		{"flag with a value", []string{"convert", "trip.srt", "--bom=1"}, "The option --bom takes no value."},
		{"unknown format", []string{"convert", "trip.srt", "--to", "docx"}, `Unknown format "docx". Run "stbox formats" for the list.`},
		{"from takes no frame rate", []string{"convert", "trip.srt", "--from", "25", "--to", "23.976"}, `Unknown format "25". Run "stbox formats" for the list.`},
		{"subrip is no name", []string{"convert", "trip.srt", "--to", "subrip"}, `Unknown format "subrip". Run "stbox formats" for the list.`},
		{"txt can be neither read nor written", []string{"convert", "trip.srt", "--from", "txt", "--to", "vtt"}, `Unknown format "txt". Run "stbox formats" for the list.`},
		{"lrc can be neither read nor written", []string{"convert", "trip.srt", "--to", "lrc"}, `Unknown format "lrc". Run "stbox formats" for the list.`},
		{"no input", []string{"convert", "--to", "vtt"}, "Pass at least one input file, or - for standard input."},
		{"line ending cr", []string{"convert", "trip.srt", "--to", "vtt", "--line-ending", "cr"}, `The option --line-ending must be lf or crlf, got "cr".`},
		{"bom and no bom", []string{"convert", "trip.srt", "--to", "vtt", "--bom", "--no-bom"}, "Pass only one of --bom and --no-bom."},
		{"output and output dir", []string{"convert", "trip.srt", "--to", "vtt", "-o", "a.vtt", "--output-dir", "out"}, "Pass only one of --output and --output-dir."},
		{"value given twice", []string{"convert", "trip.srt", "--to", "vtt", "--to", "srt"}, "The option --to is given twice."},
		{"unknown encoding", []string{"convert", "trip.srt", "--to", "srt", "--encoding", "CP1125"}, `The encoding "CP1125" is unknown. Known encodings: ` + strings.Join(stbox.Encodings(), ", ") + "."},
		{"info unknown encoding", []string{"info", "trip.srt", "--encoding", "CP1125"}, `The encoding "CP1125" is unknown. Known encodings: ` + strings.Join(stbox.Encodings(), ", ") + "."},
		{"info unknown option", []string{"info", "trip.srt", "--to", "vtt"}, "Unknown option --to."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := newDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult(tt.words[0], tt.want), call{dir: dir}.run(t, tt.words...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestFormatArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want stbox.Format
	}{
		{"srt", stbox.SubRip},
		{"SRT", stbox.SubRip},
		{".srt", stbox.SubRip},
		{"..VTT", stbox.WebVTT},
		{"vtt", stbox.WebVTT},
		{"json", stbox.JSON},
		{"x.srt", stbox.SubRip},
		{"ass", stbox.ASS},
		{"ssa", stbox.ASS},
		{"sami", stbox.SAMI},
		{"SMI", stbox.SAMI},
		{"mpl2", stbox.MPL2},
		{"TMPlayer", stbox.TMPlayer},
		{"mpsub", stbox.MPSub},
		{".MPSub", stbox.MPSub},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := findFormat(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	for _, in := range []string{"subrip", "25", "", ".", "txt", "lrc", "Kass", "sʀt"} {
		_, err := findFormat(in)
		assert.EqualError(t, err, `Unknown format "`+in+`". Run "stbox formats" for the list.`)
	}
}

func TestChoiceValuesIgnoreCase(t *testing.T) {
	t.Parallel()

	lower := runCLI(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--line-ending", "crlf")
	require.Equal(t, 0, lower.Code)
	assert.Equal(t, lower, runCLI(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--line-ending", "CRLF"))
	assert.Equal(t, lower, runCLI(t, "convert", "trip.srt", "--to", "srt", "-o", "-", "--line-ending=Crlf"))
}

func TestDirectoryWithoutSubtitlesIsAUsageError(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "empty"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty", "notes.txt"), []byte("Not a subtitle.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty", "movie.mkv"), []byte("\x1A\x45\xDF\xA3"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "empty", "sub.srt"), 0o755))
	assert.Equal(t, usageResult("convert", "The directory empty holds no file with a known subtitle extension."),
		call{dir: dir}.run(t, "convert", "empty", "--to", "vtt"))
	assert.Equal(t, usageResult("convert", "The directory empty/ holds no file with a known subtitle extension."),
		call{dir: dir}.run(t, "convert", "empty/", "--to", "vtt"))
}

func TestConvertReadsTheSubtitleFilesOfADirectory(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	for _, name := range []string{"trip.srt", "shop.vtt", "notes.txt"} {
		copyFile(t, filepath.Join(dir, name), filepath.Join(dir, "in", name))
	}
	assert.Equal(t,
		result{Stdout: "in/shop.vtt -> out/shop.srt\nin/trip.srt -> out/trip.srt\n2 files: 2 succeeded, 0 failed.\n"},
		call{dir: dir}.run(t, "convert", "in", "--to", "srt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.SubRip), readFile(t, dir, "out/trip.srt"))
	assert.Equal(t, convertAs(t, "shop.vtt", stbox.SubRip), readFile(t, dir, "out/shop.srt"))
}

func TestADirectoryKeepsATxtFileThatDetectionReads(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "in", "trip.srt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "in", "film.txt"))
	copyFile(t, filepath.Join(dir, "notes.txt"), filepath.Join(dir, "in", "notes.txt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "in", "lib.JSON"))
	assert.Equal(t,
		result{Stdout: "in/film.txt -> out/film.srt\nin/lib.JSON -> out/lib.srt\nin/trip.srt -> out/trip.srt\n3 files: 3 succeeded, 0 failed.\n"},
		call{dir: dir}.run(t, "convert", "in/", "--to", "srt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "shop.vtt", stbox.SubRip), readFile(t, dir, "out/film.srt"))
}

func TestADirectoryConvertsAnMPL2TxtFileAndSkipsNotes(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "in", "trip.srt"))
	copyFile(t, testfiles.Path(t, "mpl2/real/subtitle_edit_shape.txt"), filepath.Join(dir, "in", "film.txt"))
	copyFile(t, filepath.Join(dir, "notes.txt"), filepath.Join(dir, "in", "notes.txt"))
	assert.Equal(t,
		result{Stdout: "in/film.txt -> out/film.srt\nin/trip.srt -> out/trip.srt\n2 files: 2 succeeded, 0 failed.\n"},
		call{dir: dir}.run(t, "convert", "in", "--to", "srt", "--output-dir", "out"))
	assert.Equal(t, convertAs(t, "trip.srt", stbox.SubRip), readFile(t, dir, "out/trip.srt"))
	assert.NoFileExists(t, filepath.Join(dir, "out", "notes.srt"))
	film, _, err := stbox.Load(testfiles.Path(t, "mpl2/real/subtitle_edit_shape.txt"), stbox.ReadOptions{})
	require.NoError(t, err)
	var want bytes.Buffer
	require.NoError(t, stbox.Write(&want, film, stbox.SubRip, stbox.WriteOptions{}))
	assert.Equal(t, want.String(), readFile(t, dir, "out/film.srt"))
}

func TestADirectoryConvertsAnMPSubSubFileByItsContent(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "in", "trip.srt"))
	copyFile(t, testfiles.Path(t, "mpsub/real/mplayer_doc_time.sub"), filepath.Join(dir, "in", "film.sub"))
	assert.Equal(t,
		result{Stdout: "in/film.sub -> out/film.srt\nin/trip.srt -> out/trip.srt\n2 files: 2 succeeded, 0 failed.\n"},
		call{dir: dir}.run(t, "convert", "in", "--to", "srt", "--output-dir", "out"))
	film, info, err := stbox.Load(testfiles.Path(t, "mpsub/real/mplayer_doc_time.sub"), stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.MPSub, info.Format)
	var want bytes.Buffer
	require.NoError(t, stbox.Write(&want, film, stbox.SubRip, stbox.WriteOptions{}))
	assert.Equal(t, want.String(), readFile(t, dir, "out/film.srt"))
}

func TestDashReadsStandardInputAndWritesStandardOutput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	trip := readFile(t, dir, "trip.srt")
	vtt := convertAs(t, "trip.srt", stbox.WebVTT)
	assert.Equal(t, result{Stdout: vtt}, call{dir: dir, stdin: trip}.run(t, "convert", "-", "--to", "vtt"))
	assert.Equal(t, result{Stdout: vtt}, call{dir: dir}.run(t, "convert", "trip.srt", "-o", "-", "--to", "vtt"))
	assert.Equal(t, result{Stdout: vtt}, call{dir: dir, stdin: trip}.run(t, "convert", "-", "--from", "srt", "--to", "vtt"))
	assert.Equal(t, result{Stdout: vtt}, call{dir: dir, stdin: trip}.run(t, "convert", "-", "-", "--to", "vtt"))
}

func TestTheContentSetsTheInputFormatBeforeTheExtension(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.Rename(filepath.Join(dir, "trip.srt"), filepath.Join(dir, "trip.txt")))
	assert.Equal(t, result{Stdout: convertAs(t, "trip.srt", stbox.WebVTT)},
		call{dir: dir}.run(t, "convert", "trip.txt", "--to", "vtt", "-o", "-"))
	assert.Equal(t, result{Code: exitFile, Stderr: "notes.txt: Format detection found no subtitle format. Pass --from FORMAT.\n"},
		call{dir: dir}.run(t, "convert", "notes.txt", "--to", "vtt"))
	assert.Equal(t, result{Code: exitFile, Stderr: "missing.srt: The file does not exist.\n"},
		call{dir: dir}.run(t, "convert", "missing.srt", "--to", "vtt"))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub.srt"), 0o755))
	assert.Equal(t, result{Code: exitFile, Stderr: "su*.srt: The file does not exist.\n"},
		call{dir: dir}.run(t, "convert", "su*.srt", "--to", "vtt"))
}

func TestAStrictParseFailureSuggestsLenient(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	hinted := result{Code: exitFile, Stderr: brokenMessage + "\n" + lenientHint + "\n"}
	assert.Equal(t, hinted, call{dir: dir}.run(t, "convert", "broken.srt", "--to", "vtt", "-o", "out.vtt"))
	assert.Equal(t, hinted, call{dir: dir}.run(t, "convert", "broken.srt", "--from", "srt", "--to", "vtt", "-o", "-"))
	stdin := result{Code: exitFile, Stderr: strings.Replace(hinted.Stderr, "broken.srt", "stdin", 1)}
	assert.Equal(t, stdin, call{dir: dir, stdin: readFile(t, dir, "broken.srt")}.run(t, "convert", "-", "--to", "vtt", "-o", "-"))
	assert.Equal(t, 0, call{dir: dir}.run(t, "convert", "broken.srt", "--to", "vtt", "-o", "lenient.vtt", "--lenient").Code)
	assert.Equal(t, result{Code: exitFile, Stdout: "[]\n", Stderr: hinted.Stderr}, call{dir: dir}.run(t, "info", "broken.srt", "--json"))
	assert.NotContains(t, call{dir: dir}.run(t, "convert", "notes.txt", "--to", "vtt").Stderr, "--lenient")
}

func TestLenientEncodingLineEndingAndBOMOptions(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Stdout: "1\r\n00:00:01,000 --> 00:00:02,000\r\nFirst cue.\r\n\r\n2\r\n00:00:05,000 --> 00:00:06,000\r\nLast cue.\r\n",
			Stderr: brokenMessage + " (skipped)\n",
		},
		call{dir: dir}.run(t, "convert", "broken.srt", "--to", "srt", "-o", "-", "--lenient", "--line-ending", "crlf", "--no-bom"))

	got := call{dir: dir}.run(t, "convert", "latin1.srt", "--encoding", "Windows-1252", "--to", "vtt", "-o", "-")
	assert.Equal(t, 0, got.Code)
	assert.Empty(t, got.Stderr)
	assert.True(t, strings.HasPrefix(got.Stdout, bom+"WEBVTT\n"), got.Stdout)
	assert.Contains(t, got.Stdout, "Café au lait.")

	got = call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "srt", "-o", "-", "--bom")
	assert.Equal(t, 0, got.Code)
	assert.True(t, strings.HasPrefix(got.Stdout, bom+"1\n00:00:10,000 --> "), got.Stdout)
	got = call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "srt", "-o", "-", "--no-bom")
	assert.True(t, strings.HasPrefix(got.Stdout, "1\n"), got.Stdout)
	got = call{dir: dir}.run(t, "convert", "shop.vtt", "--to", "sbv", "-o", "-", "--bom")
	assert.Equal(t, result{Stdout: bom + "0:00:10.000,0:00:12.000\nGood morning.\n\n0:00:13.000,0:00:15.500\nThe shop opens at nine.\n"}, got)
}

func TestEmptyInput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.vtt"), nil, 0o600))
	assert.Equal(t, result{Stdout: "empty.vtt -> empty.srt\n"},
		call{dir: dir}.run(t, "convert", "empty.vtt", "--to", "srt", "--no-bom", "-o", "empty.srt"))
	assert.Empty(t, readFile(t, dir, "empty.srt"))
	assert.Equal(t, result{Code: exitFile, Stderr: "stdin: " + unknownFormatMessage + "\n"},
		call{dir: dir}.run(t, "convert", "-", "--to", "srt"))
	assert.Equal(t, result{}, call{dir: dir}.run(t, "convert", "-", "--from", "srt", "--to", "srt", "--no-bom"))
}

func TestUnreadableInput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	path := filepath.Join(dir, "trip.srt")
	require.NoError(t, os.Chmod(path, 0))
	if f, err := os.Open(path); err == nil {
		_ = f.Close()
		t.Skip("the user of the test can read a file without permissions")
	}
	assert.Equal(t, result{Code: exitFile, Stderr: "trip.srt: Cannot read the file.\n"},
		call{dir: dir}.run(t, "convert", "trip.srt", "--to", "vtt", "--output-dir", "out"))
	assert.NoDirExists(t, filepath.Join(dir, "out"))
}

func TestExpand(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "trip.srt"), filepath.Join(dir, "a", "trip.srt"))
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "a", "b.srt"))
	require.NoError(t, os.Symlink(filepath.Join(dir, "trip.srt"), filepath.Join(dir, "link.srt")))
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"several dashes count once", []string{"-", "trip.srt", "-"}, []string{"-", "trip.srt"}},
		{"glob and file", []string{"a/*.srt", "a/b.srt"}, []string{"a/b.srt", "a/trip.srt"}},
		{"the first spelling wins", []string{"trip.srt", "./trip.srt", "a/../trip.srt", "link.srt"}, []string{"trip.srt"}},
		{"directory with a trailing slash", []string{"a/"}, []string{"a/b.srt", "a/trip.srt"}},
		{"missing path stays", []string{"missing.srt", "missing.srt"}, []string{"missing.srt"}},
		{"glob without match stays", []string{"x/*.srt"}, []string{"x/*.srt"}},
		{"glob skips directories", []string{"*"}, []string{"broken.srt", "frames.sub", "frames_23976.vtt", "latin1.srt", "link.srt", "notes.txt", "shop.vtt"}},
		{"absolute path stays absolute", []string{filepath.Join(dir, "a", "b.srt")}, []string{filepath.Join(dir, "a", "b.srt")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &runner{c: &console{dir: dir}}
			got, err := r.expand(tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPeek(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	copyFile(t, filepath.Join(dir, "shop.vtt"), filepath.Join(dir, "shop.srt"))
	copyFile(t, filepath.Join(dir, "notes.txt"), filepath.Join(dir, "notes.srt"))
	copyFile(t, filepath.Join(dir, "notes.txt"), filepath.Join(dir, "notes.json"))
	tests := []struct {
		arg  string
		from stbox.Format
		want stbox.Format
	}{
		{"trip.srt", "", stbox.SubRip},
		{"shop.srt", "", stbox.WebVTT},
		{"notes.srt", "", stbox.SubRip},
		{"notes.json", "", ""},
		{"notes.txt", "", ""},
		{"notes.txt", stbox.WebVTT, stbox.WebVTT},
		{"missing.srt", stbox.WebVTT, ""},
		{"-", stbox.WebVTT, ""},
	}
	for _, tt := range tests {
		t.Run(tt.arg+" "+string(tt.from), func(t *testing.T) {
			t.Parallel()

			r := &runner{c: &console{dir: dir}, readOptions: stbox.ReadOptions{Format: tt.from}}
			assert.Equal(t, tt.want, r.peek(tt.arg))
		})
	}
}
