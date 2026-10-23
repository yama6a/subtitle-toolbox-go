package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

const framesCSV = "csv/own_frame_times.csv"

// Ports testInputFpsReadsCsvTimesInFrames of tests/Cli/BinaryInputTest.php of subtitle-toolbox-php.
func TestInputFPSReadsCSVFrameTimes(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, framesCSV)
	s, _, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: stbox.CSV, CSV: csv.ReadOptions{FrameRate: 25}})
	require.NoError(t, err)
	var srt bytes.Buffer
	require.NoError(t, stbox.Write(&srt, s, stbox.SubRip, stbox.WriteOptions{}))

	dir := fixtureDir(t, map[string]string{"frames.csv": framesCSV})
	assert.Equal(t, result{Stdout: srt.String()}, call{dir: dir}.run(t, "convert", "frames.csv", "--to", "srt", "-o", "-", "--input-fps", "25"))
	assert.Equal(t, result{Stdout: strings.ReplaceAll(string(content), "\r\n", "\n")},
		call{dir: dir}.run(t, "convert", "frames.csv", "--to", "csv", "-o", "-", "--fps", "25", "--no-bom"))
}

func TestCSVFrameTimesNeedARate(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"frames.csv": framesCSV})
	want := result{
		Code:   exitFile,
		Stderr: `frames.csv: line 2: The time "00:00:01:12" counts frames and needs a frame rate. Pass --fps or --input-fps.` + "\n" + lenientHint + "\n",
	}
	assert.Equal(t, want, call{dir: dir}.run(t, "convert", "frames.csv", "--to", "srt", "-o", "-"))
	assert.Equal(t, want, call{dir: dir}.run(t, "info", "frames.csv"))
}

func TestCSVFrameRatesOfInfoDiffAndSync(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"frames.csv": framesCSV})
	for _, words := range [][]string{
		{"info", "frames.csv", "--input-fps", "25"},
		{"info", "frames.csv", "--fps", "25"},
		{"diff", "frames.csv", "frames.csv", "--fps", "25"},
		{"diff", "frames.csv", "frames.csv", "--input-fps", "25"},
		{"sync", "frames.csv", "--reference", "frames.csv", "--fps", "25"},
		{"sync", "frames.csv", "--reference", "frames.csv", "--input-fps", "25"},
	} {
		got := call{dir: dir}.run(t, words...)
		assert.Equal(t, 0, got.Code, "%v: %s", words, got.Stderr)
	}
}

// TSV output writes tabs, and CSV output of a TSV input writes commas, as the decision of #14 says.
func TestConvertTSVAndCSV(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "trip.srt -> trip.tsv\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "tsv", "-o", "trip.tsv"))
	tsv := readFile(t, dir, "trip.tsv")
	assert.True(t, strings.HasPrefix(tsv, "\ufeffstart\tend\ttext\n"), tsv)
	assert.NotContains(t, tsv, ",")

	assert.Equal(t, result{Stdout: "trip.tsv -> trip.csv\n"}, call{dir: dir}.run(t, "convert", "trip.tsv", "--to", "csv", "-o", "trip.csv"))
	out := readFile(t, dir, "trip.csv")
	assert.True(t, strings.HasPrefix(out, "\ufeffstart,end,text\n"), out)
	assert.NotContains(t, out, "\t")
}
