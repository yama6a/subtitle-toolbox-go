package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// syncDir returns a directory with the CLI fixtures and the sync fixtures as de.srt, en.srt and tv.srt.
func syncDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	for from, to := range map[string]string{
		"own_target_de_25fps.srt": "de.srt", "own_reference_en.srt": "en.srt", "own_reference_en_tv_break.srt": "tv.srt",
		"own_ffmpeg_silencedetect.log": "silence.log",
	} {
		copyFile(t, testfiles.Path(t, "sync/"+from), filepath.Join(dir, to))
	}
	return dir
}

// Adapted from BinaryTimingTest::testSyncToAReferenceSubtitle.
func TestSyncToAReferenceSubtitle(t *testing.T) {
	t.Parallel()

	t.Run("scale and offset", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--reference", "en.srt")
		want := result{
			Stdout: string(testfiles.Read(t, "sync/own_target_de_synced.srt")),
			Stderr: "de.srt: scale 1.04271, offset -2.3 s, score 0.89\n",
		}
		assert.Equal(t, want, got)
	})

	t.Run("2 segments", func(t *testing.T) {
		t.Parallel()

		dir := syncDir(t)
		got := call{dir: dir}.run(t, "sync", "de.srt", "--reference", "tv.srt", "--min-offset", "-00:03:00",
			"--max-offset", "03:00", "--max-splits", "2", "-o", "synced.srt")
		want := result{
			Stdout: "de.srt -> synced.srt\n",
			Stderr: "de.srt: scale 1.04271, offset -2.31 s, score 0.89\nde.srt: from 0 s: offset -2.31 s\n" +
				"de.srt: from 414.32 s: offset 147.7 s\n",
		}
		assert.Equal(t, want, got)
		assert.Equal(t, string(testfiles.Read(t, "sync/own_target_de_split_synced.srt")), readFile(t, dir, "synced.srt"))
	})

	t.Run("a penalty above the gain keeps 1 segment", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--reference", "tv.srt", "--min-offset", "-180",
			"--max-offset", "180", "--max-splits", "2", "--split-penalty", "5")
		assert.Equal(t, 0, got.Code)
		assert.Equal(t, string(testfiles.Read(t, "sync/own_target_de_split_penalty_synced.srt")), got.Stdout)
		assert.NotContains(t, got.Stderr, "from")
	})

	t.Run("a low score", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--reference", "trip.srt", "--no-scale")
		assert.Equal(t, 0, got.Code)
		assert.True(t, strings.HasPrefix(got.Stderr, "de.srt: scale 1, "), got.Stderr)
		assert.True(t, strings.HasSuffix(got.Stderr, "de.srt: the score is below 0.5, so the files likely do not match.\n"), got.Stderr)
	})

	t.Run("a batch", func(t *testing.T) {
		t.Parallel()

		dir := syncDir(t)
		got := call{dir: dir}.run(t, "sync", "de.srt", "tv.srt", "--reference", "en.srt", "--output-dir", "out",
			"--keep-going", "--line-ending", "crlf", "--encoding", "windows-1252")
		assert.Equal(t, 0, got.Code)
		assert.Equal(t, "de.srt -> out/de.srt\ntv.srt -> out/tv.srt\n2 files: 2 succeeded, 0 failed.\n", got.Stdout)
		assert.Contains(t, got.Stderr, "de.srt: scale 1.04271, offset -2.3 s, score 0.89\ntv.srt: scale 1, ")
		assert.Contains(t, readFile(t, dir, "out/de.srt"), "\r\n")
	})
}

func TestSyncGolden(t *testing.T) {
	t.Parallel()

	got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--reference", "en.srt", "--to", "srt")
	require.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, "de.srt: scale 1.04271, offset -2.3 s, score 0.89\n", got.Stderr)
	golden.Equal(t, testfiles.Path(t, "sync/cli_sync_de_to_srt.out.srt"), []byte(got.Stdout), *update)
}

func TestSyncUsageErrors(t *testing.T) {
	t.Parallel()

	const onlyOne = "Pass one of --reference FILE and --silence-log FILE."
	sync := []string{"sync", "de.srt", "--reference", "missing.srt"}
	tests := []struct {
		name  string
		words []string
		msg   string
	}{
		{"no reference", []string{"sync", "de.srt"}, onlyOne},
		{"--media-duration alone", []string{"sync", "de.srt", "--media-duration", "840"}, onlyOne},
		{"both inputs", []string{"sync", "de.srt", "--reference", "en.srt", "--silence-log", "silence.log", "--media-duration", "840"}, onlyOne},
		{"--silence-log without a duration", []string{"sync", "de.srt", "--silence-log", "silence.log"}, "Pass --media-duration with --silence-log."},
		{
			"--reference-track without --reference",
			[]string{"sync", "de.srt", "--silence-log", "silence.log", "--media-duration", "840", "--reference-track", "2"},
			"Pass --reference with --reference-track.",
		},
		{
			"bad duration",
			[]string{"sync", "de.srt", "--silence-log", "silence.log", "--media-duration", "0"},
			"The option --media-duration must be greater than 0.",
		},
		{
			"bad track",
			[]string{"sync", "de.srt", "--reference", "en.srt", "--reference-track", "x"},
			`The option --reference-track needs a whole number of 1 or more, got "x".`,
		},
		{
			"output is the silence log",
			[]string{"sync", "de.srt", "--silence-log", "silence.log", "--media-duration", "840", "-o", "silence.log"},
			"The output silence.log is a file that the command reads. Pass another output file or directory.",
		},
		{
			"offsets reversed", slices.Concat(sync, []string{"--min-offset", "10", "--max-offset", "-10"}),
			"The minimum offset must not be greater than the maximum offset, got 10 and -10.",
		},
		{
			"offset beyond a day", slices.Concat(sync, []string{"--min-offset", "-90000"}),
			"The minimum offset must be from -86400 to 86400 seconds, got -90000.",
		},
		{
			"offsets too far apart", slices.Concat(sync, []string{"--min-offset", "-3600", "--max-offset", "3600.5"}),
			"The minimum and the maximum offset must be at most 7200 seconds apart, got -3600 and 3600.5.",
		},
		{
			"bad offset", slices.Concat(sync, []string{"--max-offset", "soon"}),
			`The option --max-offset needs seconds or a timecode such as 00:01:02.500, got "soon".`,
		},
		{
			"--max-splits two", slices.Concat(sync, []string{"--max-splits", "two"}),
			`The option --max-splits needs a whole number from 0 to 10, got "two".`,
		},
		{
			"--max-splits 11", slices.Concat(sync, []string{"--max-splits", "11"}),
			`The option --max-splits needs a whole number from 0 to 10, got "11".`,
		},
		{"negative penalty", slices.Concat(sync, []string{"--split-penalty", "-1"}), "The option --split-penalty must not be negative."},
		{"bom twice", slices.Concat(sync, []string{"--bom", "--no-bom"}), "Pass only one of --bom and --no-bom."},
		{
			"output is the reference",
			[]string{"sync", "de.srt", "--reference", "en.srt", "-o", "en.srt"},
			"The output en.srt is a file that the command reads. Pass another output file or directory.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := syncDir(t)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("sync", tt.msg), call{dir: dir}.run(t, tt.words...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestSyncReferenceFailures(t *testing.T) {
	t.Parallel()

	broken := `broken.srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:03,000 => 00:00:04,000".`
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			name:  "missing",
			words: []string{"sync", "de.srt", "--reference", "missing.srt"},
			want:  result{Code: exitFile, Stderr: "Error: missing.srt: The file does not exist.\n"},
		},
		{
			name:  "a directory",
			words: []string{"sync", "de.srt", "--reference", "."},
			want:  result{Code: exitFile, Stderr: "Error: .: The file does not exist.\n"},
		},
		{
			name:  "broken",
			words: []string{"sync", "de.srt", "--reference", "broken.srt"},
			want:  result{Code: exitFile, Stderr: "Error: " + broken + "\n" + lenientHint + "\n"},
		},
		{
			name:  "not a subtitle",
			words: []string{"sync", "de.srt", "--reference", "notes.txt"},
			want:  result{Code: exitFile, Stderr: "Error: notes.txt: " + sideFileUnknownFormat + "\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: syncDir(t)}.run(t, tt.words...))
		})
	}

	t.Run("lenient prints the warnings with the path", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--reference", "broken.srt", "--lenient")
		assert.Equal(t, 0, got.Code)
		assert.True(t, strings.HasPrefix(got.Stderr, broken+" (skipped)\nde.srt: scale "), got.Stderr)
	})

	t.Run("--from does not apply to the reference", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "trip.srt", "--reference", "shop.vtt", "--from", "srt")
		assert.Equal(t, 0, got.Code, got.Stderr)
		assert.True(t, strings.HasPrefix(got.Stderr, "trip.srt: scale "), got.Stderr)
	})
}

// Adapted from BinaryTimingTest::testSyncToTheSpeech and BinaryGeneralTest.
func TestSyncToTheSpeech(t *testing.T) {
	t.Parallel()

	t.Run("the golden", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--silence-log", "silence.log", "--media-duration", "00:14:00")
		want := result{
			Stdout: string(testfiles.Read(t, "sync/own_target_de_synced.srt")),
			Stderr: "de.srt: scale 1.04271, offset -2.3 s, score 0.78\n",
		}
		assert.Equal(t, want, got)
	})

	tests := []struct {
		name string
		log  string
		want string
	}{
		{"a mono log", logLine + "channel: 0 | silence_start: 1.5\n", "Error: bad.log: line 1: Cannot read a silencedetect log with one line per channel. Run silencedetect without mono.\n"},
		{"a log out of order", logLine + "silence_end: 1.5\n", "Error: bad.log: line 1: The line silence_end is out of order. The silence_start and silence_end lines must alternate.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := syncDir(t)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.log"), []byte(tt.log), 0o600))
			got := call{dir: dir}.run(t, "sync", "de.srt", "--silence-log", "bad.log", "--media-duration", "840")
			assert.Equal(t, result{Code: exitFile, Stderr: tt.want}, got)
		})
	}

	t.Run("a missing log", func(t *testing.T) {
		t.Parallel()

		got := call{dir: syncDir(t)}.run(t, "sync", "de.srt", "--silence-log", "missing.log", "--media-duration", "60")
		assert.Equal(t, result{Code: exitFile, Stderr: "Error: missing.log: The file does not exist.\n"}, got)
	})
}

const logLine = "[silencedetect @ 0x1] "

func TestSyncHelp(t *testing.T) {
	t.Parallel()

	head := `Usage: stbox sync <input>... --reference FILE [options]
       stbox sync <input>... --silence-log FILE --media-duration SECONDS
                  [options]

Find the offset and frame-rate scale against a reference subtitle or the speech,
and retime the input.

Only the cue times count, so the reference can be in another language. The scale
is 1 or a factor between 23.976, 24 and 25 fps. sync prints the scale, the
offset and a score from 0 to 1. A score below 0.5 means that the files likely do
not match. Without -o or --output-dir, the output goes to standard output.
For a sync to the speech, run this command and pass --silence-log silence.log:
  ffmpeg -i movie.mkv -af silencedetect=n=-30dB:d=0.4 -f null - 2> silence.log
A Whisper JSON transcript of the audio also works as --reference.

Options:
  --reference FILE          Subtitle in sync with the video, in any format that
                            the tool reads.
  --reference-track NUMBER  Subtitle track of an MKV, WebM, MP4 or MPEG-TS
                            reference file. Needed when the file has 2 or more
                            subtitle tracks.
  --silence-log FILE        Log of the FFmpeg silencedetect filter. The speech
                            between the silences is the reference.
  --media-duration SECONDS  Duration of the video, for --silence-log.
  --min-offset SECONDS      Smallest offset to try. Default: -60.
  --max-offset SECONDS      Largest offset to try. Default: 60.
  --no-scale                Keep the scale at 1 and find only the offset.
  --max-splits SPLITS       Find up to this many points where the offset jumps,
                            for example at ad breaks. Default: 0.
  --split-penalty SCORE     Score that a split must add to stay. Default: 0.1.
  --to FORMAT               Output format. Default: the input format.
`
	got := call{}.run(t, "help", "sync")
	assert.Equal(t, 0, got.Code)
	assert.True(t, strings.HasPrefix(got.Stdout, head), got.Stdout)
	assert.True(t, strings.HasSuffix(got.Stdout, "  -h, --help                Show this help.\n"), got.Stdout)
}

func TestFormatDecimals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in       float64
		decimals int
		want     string
	}{
		{1.0427109, 5, "1.04271"},
		{1.000005, 5, "1.00001"},
		{1, 5, "1"},
		{0.885, 2, "0.89"},
		{0.8949, 2, "0.89"},
		{0.995, 2, "1"},
		{0.004, 2, "0"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatDecimals(tt.in, tt.decimals))
		})
	}
}
