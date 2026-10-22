package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

const (
	framesOutput     = "{25}{75}Hello from the frames.\n{100}{150}{y:i}Second line.\n"
	framesAt23976    = "{24}{72}Hello from the frames.\n{96}{144}{y:i}Second line.\n"
	unknownRate      = "frames.sub: The frame rate is unknown. Pass --fps or --input-fps, start the file with {1}{1}<fps>, or pass --lenient for 23.976 fps.\n"
	noOutputRate     = "trip.srt: MicroDVD output needs the frame rate of the video. Pass --fps or --output-fps.\n"
	framesAt25Header = "1\n00:00:01,000 --> 00:00:03,000\nHello from the frames.\n"
)

// Ports testRateFallsBackToFps and testRateChecksFpsAlsoWhenTheOwnOptionIsGiven of tests/Cli/ArgumentsTest.php of
// subtitle-toolbox-php.
func TestRateGetter(t *testing.T) {
	t.Parallel()

	spec := []option{valueOption("fps", "RATE", "Frame rate."), valueOption("input-fps", "RATE", "Input frame rate.")}
	tests := []struct {
		name   string
		words  []string
		want   float64
		wantOK bool
		err    string
	}{
		{"fps alone", []string{"--fps", "25"}, 25, true, ""},
		{"own option wins", []string{"--fps", "25", "--input-fps", "24"}, 24, true, ""},
		{"own option alone", []string{"--input-fps", "23.976"}, 23.976, true, ""},
		{"neither", nil, 0, false, ""},
		{"bad fps with own option", []string{"--fps", "0", "--input-fps", "24"}, 0, false, "The option --fps must be greater than 0."},
		{"bad own option", []string{"--input-fps", "-1"}, 0, false, "The option --input-fps must be greater than 0."},
		{"no number", []string{"--input-fps", "x"}, 0, false, `The option --input-fps needs a number, got "x".`},
		{"infinite", []string{"--fps", "1e999"}, 0, false, `The option --fps needs a finite number, got "1e999".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a, err := parseArgs(tt.words, spec)
			require.NoError(t, err)
			got, ok, err := a.Rate("input-fps")
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOK, ok)
			assert.InDelta(t, tt.want, got, 0)
		})
	}
}

// Ports testMicroDvdNeedsTheFrameRate and testInputAndOutputFrameRatesAreSeparate of tests/Cli/BinaryInputTest.php of
// subtitle-toolbox-php.
func TestMicroDVDFrameRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			"input without a rate",
			[]string{"convert", "frames.sub", "--to", "srt", "-o", "-"},
			result{Code: exitFile, Stderr: unknownRate + lenientHint + "\n"},
		},
		{
			"fps sets the input rate",
			[]string{"convert", "frames.sub", "--to", "srt", "-o", "-", "--fps", "25"},
			result{Stdout: "\xEF\xBB\xBF" + framesAt25Header + "\n2\n00:00:04,000 --> 00:00:06,000\n<i>Second line.</i>\n"},
		},
		{"fps sets the output rate", []string{"convert", "frames.sub", "--shift", "0", "--fps", "25"}, result{Stdout: framesOutput}},
		{"fps with to microdvd", []string{"convert", "frames.sub", "--fps", "25", "--to", "microdvd", "-o", "-"}, result{Stdout: framesOutput}},
		{
			"input and output rates",
			[]string{"convert", "frames.sub", "--input-fps", "25", "--output-fps", "23.976", "--to", "microdvd", "-o", "-"},
			result{Stdout: framesAt23976},
		},
		{
			"output rate wins over fps",
			[]string{"convert", "frames.sub", "--fps", "25", "--output-fps", "23.976", "--to", "microdvd", "-o", "-"},
			result{Stdout: framesAt23976},
		},
		{
			"output rate after =",
			[]string{"convert", "frames.sub", "--input-fps=25", "--output-fps=23.976", "--to", "sub", "-o", "-"},
			result{Stdout: framesAt23976},
		},
		{
			"output rate without an input rate",
			[]string{"convert", "frames.sub", "--output-fps", "25", "--to", "microdvd", "-o", "-"},
			result{Code: exitFile, Stderr: unknownRate + lenientHint + "\n"},
		},
		{
			"lenient reads at 23.976",
			[]string{"convert", "frames.sub", "--lenient", "--to", "microdvd", "-o", "-"},
			result{Stdout: framesOutput, Stderr: "frames.sub: The file has no {1}{1}<fps> line. The parser used 23.976 fps. (repaired)\n"},
		},
		{"output without a rate", []string{"convert", "trip.srt", "--to", "microdvd", "-o", "-"}, result{Code: exitFile, Stderr: noOutputRate}},
		{
			"output rate on SubRip output",
			[]string{"convert", "trip.srt", "--to", "srt", "-o", "-", "--output-fps", "25"},
			result{Stdout: convertAs(t, "trip.srt", stbox.SubRip)},
		},
		{"info without a rate", []string{"info", "frames.sub"}, result{Code: exitFile, Stderr: unknownRate + lenientHint + "\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, runCLI(t, tt.words...))
		})
	}
}

func TestMicroDVDOutputToAFile(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "trip.srt -> trip.sub\n"}, call{dir: dir}.run(t, "convert", "trip.srt", "--to", "microdvd", "-o", "trip.sub", "--fps", "23.976"))
	assert.Regexp(t, `^\{24\}\{72\}`, readFile(t, dir, "trip.sub"))
}

func TestInfoReadsMicroDVDWithTheRate(t *testing.T) {
	t.Parallel()

	for _, words := range [][]string{{"--input-fps", "25"}, {"--fps", "25"}} {
		got := runCLI(t, append([]string{"info", "frames.sub", "--json"}, words...)...)
		require.Equal(t, 0, got.Code, got.Stderr)
		var entries []map[string]any
		require.NoError(t, json.Unmarshal([]byte(got.Stdout), &entries))
		require.Len(t, entries, 1)
		assert.Equal(t, "microdvd", entries[0]["format"])
	}
}

func TestFrameRateUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  string
	}{
		// Ports inputUsageErrors of tests/Cli/BinaryInputTest.php of subtitle-toolbox-php.
		{"input fps 0", []string{"convert", "frames.sub", "--input-fps", "0", "--to", "srt", "-o", "-"}, "The option --input-fps must be greater than 0."},
		// Ports testAStrictParseFailureSuggestsLenient of tests/Cli/BinaryInputTest.php of subtitle-toolbox-php.
		{"output fps without a number", []string{"convert", "trip.srt", "--to", "vtt", "--output-fps", "x"}, `The option --output-fps needs a number, got "x".`},
		{"bad fps fails with an input fps", []string{"convert", "frames.sub", "--fps", "-1", "--input-fps", "25"}, "The option --fps must be greater than 0."},
		{"input fps before from", []string{"convert", "frames.sub", "--input-fps", "0", "--from", "docx"}, "The option --input-fps must be greater than 0."},
		{"to before output fps", []string{"convert", "frames.sub", "--to", "docx", "--output-fps", "0"}, `Unknown format "docx". Run "stbox formats" for the list.`},
		{"info fps", []string{"info", "frames.sub", "--fps", "0"}, "The option --fps must be greater than 0."},
		{"diff input fps", []string{"diff", "frames.sub", "frames.sub", "--input-fps", "x"}, `The option --input-fps needs a number, got "x".`},
		{"sync output fps", []string{"sync", "frames.sub", "--reference", "trip.srt", "--output-fps", "0"}, "The option --output-fps must be greater than 0."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, usageResult(tt.words[0], tt.want), runCLI(t, tt.words...))
		})
	}
}

func TestFrameRatesOfDiffAndSync(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "diff", "frames.sub", "frames.sub", "--fps", "25")
	assert.Equal(t, 0, got.Code, got.Stderr)

	got = runCLI(t, "sync", "frames.sub", "--reference", "frames.sub", "--input-fps", "25", "--output-fps", "50", "--to", "microdvd")
	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "{50}{150}Hello from the frames.\n")

	got = runCLI(t, "sync", "frames.sub", "--reference", "frames.sub", "--fps", "25")
	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, framesOutput)
}

// Ports the frame rate change of cli/frames_23976.vtt of subtitle-toolbox-php. The PHP output is the cue oracle,
// the Go golden next to it holds the bytes.
func TestMicroDVDFrameRateChange(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "convert", "frames.sub", "--input-fps", "25", "--from-fps", "25", "--to-fps", "23.976", "--to", "vtt")
	require.Equal(t, 0, got.Code, got.Stderr)
	assertCuesOf(t, "cli/frames_23976.vtt", got.Stdout, stbox.WebVTT)
	golden.Equal(t, testfiles.Path(t, "cli/expected/frames_23976.vtt"), []byte(got.Stdout), *update)
}

// Ports the MicroDVD row of tests/Cli/ConvertGoldensTest.php of subtitle-toolbox-php.
func TestConvertMicroDVDGolden(t *testing.T) {
	t.Parallel()

	dir := fixtureDir(t, map[string]string{"styles.sub": "microdvd/real/sub_microdvd_with_styles.sub"})
	want := string(testfiles.Read(t, "microdvd/real/expected/sub_microdvd_with_styles.sub.to.srt"))
	assert.Equal(t, result{Stdout: want}, call{dir: dir}.run(t, "convert", "styles.sub", "--input-fps", "23.976", "--to", "srt"))
	assert.Equal(t, result{Stdout: "styles.sub -> out.srt\n"},
		call{dir: dir}.run(t, "convert", "styles.sub", "--input-fps", "23.976", "--to", "srt", "-o", "out.srt"))
	assert.Equal(t, want, readFile(t, dir, "out.srt"))
}

func TestMicroDVDOptionsOnlyForMicroDVDOutput(t *testing.T) {
	t.Parallel()

	w := writeOutput{outputFPS: 25}
	assert.Equal(t, microdvd.WriteOptions{FrameRate: 25}, w.optionsFor(stbox.MicroDVD).MicroDVD)
	assert.Equal(t, microdvd.WriteOptions{}, w.optionsFor(stbox.SubRip).MicroDVD)
}
