package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// hlsDir returns a directory with the CLI fixtures and the node-webvtt file as talk.vtt, plus the files in extra.
func hlsDir(t *testing.T, extra map[string]string) string {
	t.Helper()

	dir := newDir(t)
	copyFile(t, testfiles.Path(t, "hls/node-webvtt-subs1.vtt"), filepath.Join(dir, "talk.vtt"))
	for name, content := range extra {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

const (
	hlsExists    = " exists. The tool never overwrites a file. Remove the playlist and the segments, or pass another --output-dir."
	hlsGoldenCLI = "--segment 00:10 --mpegts 126000 --pattern part%03d.vtt --media-duration 00:02:30 --playlist index.m3u8"
)

func TestHlsWritesTheGoldenFolders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		folder, outDir string
		extra          []string
	}{
		{"node-webvtt-subs1", "out", nil},
		{"node-webvtt-subs1-local", "local", []string{"--local", "10"}},
	}
	for _, tt := range tests {
		t.Run(tt.folder, func(t *testing.T) {
			t.Parallel()

			dir := hlsDir(t, nil)
			words := slices.Concat([]string{"hls", "talk.vtt", "--output-dir", tt.outDir}, strings.Fields(hlsGoldenCLI), tt.extra)
			got := call{dir: dir}.run(t, words...)
			assert.Equal(t, result{Stdout: "talk.vtt -> " + tt.outDir + "/index.m3u8, 15 segments\n"}, got)

			golden := testfiles.Path(t, "hls/expected/"+tt.folder)
			want, err := os.ReadDir(golden)
			require.NoError(t, err)
			have, err := os.ReadDir(filepath.Join(dir, tt.outDir))
			require.NoError(t, err)
			require.Len(t, have, len(want))
			for i, e := range want {
				assert.Equal(t, e.Name(), have[i].Name())
				assert.Equal(t, string(testfiles.Read(t, "hls/expected/"+tt.folder+"/"+e.Name())), readFile(t, dir, tt.outDir+"/"+e.Name()), e.Name())
			}
		})
	}
}

func TestHlsChecksBeforeTheRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra map[string]string
		words []string
		msg   string
	}{
		{"no output directory", nil, []string{"talk.vtt"}, "Pass --output-dir DIR."},
		{"an empty output directory", nil, []string{"talk.vtt", "--output-dir", ""}, "Pass --output-dir DIR."},
		{"an output directory that is a file", nil, []string{"talk.vtt", "--output-dir", "trip.srt"}, "The --output-dir trip.srt is a file. Pass a directory."},
		{"2 inputs", nil, []string{"talk.vtt", "trip.srt", "--output-dir", "new"}, "The hls command takes one input file, got 2."},
		{"a playlist that is a segment", nil, []string{"talk.vtt", "--output-dir", "new", "--playlist", "sub0.vtt"}, "The playlist sub0.vtt has the name of a segment. Pass another --playlist or --pattern."},
		{
			"a playlist that is a segment of another pattern", nil,
			[]string{"talk.vtt", "--output-dir", "new", "--pattern", "p%03d.vtt", "--playlist", "p007.vtt"},
			"The playlist p007.vtt has the name of a segment. Pass another --playlist or --pattern.",
		},
		{
			"a playlist that is a segment of a pattern with a folder", nil,
			[]string{"talk.vtt", "--output-dir", "new", "--pattern", "../outside%d.vtt", "--playlist", "../outside3.vtt"},
			"The playlist ../outside3.vtt has the name of a segment. Pass another --playlist or --pattern.",
		},
		{
			"the input is a segment",
			map[string]string{"out/sub1.vtt": "old"},
			[]string{"out/sub1.vtt", "--output-dir", "out/"},
			"The output would overwrite the input out/sub1.vtt. Pass another --output-dir, --playlist or --pattern.",
		},
		{
			"the input is the playlist",
			map[string]string{"out/subs.m3u8": "old"},
			[]string{"out/subs.m3u8", "--output-dir", "out"},
			"The output would overwrite the input out/subs.m3u8. Pass another --output-dir, --playlist or --pattern.",
		},
		{
			"the input is a segment in the folder of the pattern",
			map[string]string{"outside7.vtt": "old"},
			[]string{"outside7.vtt", "--output-dir", "new", "--pattern", "../outside%d.vtt", "--playlist", "p.m3u8"},
			"The output would overwrite the input outside7.vtt. Pass another --output-dir, --playlist or --pattern.",
		},
		{"a space-padded segment exists", map[string]string{"out/x  7.vtt": "old"}, []string{"talk.vtt", "--output-dir", "out", "--pattern", "x%3d.vtt", "--media-duration", "10"}, "out/x  7.vtt" + hlsExists},
		{"a segment exists", map[string]string{"out/sub99.vtt": "old"}, []string{"talk.vtt", "--output-dir", "out"}, "out/sub99.vtt" + hlsExists},
		{
			"the playlist exists",
			map[string]string{"out/index.m3u8": "old"},
			[]string{"talk.vtt", "--output-dir", "out/", "--playlist", "index.m3u8", "--pattern", "p%d.vtt"},
			"out/index.m3u8" + hlsExists,
		},
		{
			"the playlist exists behind a missing folder",
			map[string]string{"keep.txt": "old"},
			[]string{"missing.srt", "--output-dir", "new", "--playlist", "../keep.txt"},
			"new/../keep.txt" + hlsExists,
		},
		{
			"a segment exists in the folder of the pattern",
			map[string]string{"outside7.vtt": "old"},
			[]string{"talk.vtt", "--output-dir", "new", "--pattern", "../outside%d.vtt"},
			"new/../outside7.vtt" + hlsExists,
		},
		{"--segment 1e999", nil, []string{"talk.vtt", "--output-dir", "new", "--segment", "1e999"}, `The option --segment needs a finite number, got "1e999".`},
		{"--segment 0", nil, []string{"talk.vtt", "--output-dir", "new", "--segment", "0"}, "The option --segment must be greater than 0."},
		{"--local -1", nil, []string{"talk.vtt", "--output-dir", "new", "--local", "-1"}, "The option --local must not be negative."},
		{
			"--local with 5 parts", nil,
			[]string{"talk.vtt", "--output-dir", "new", "--local", "1:2:3:4:5"},
			`The option --local needs seconds or a timecode such as 00:01:02.500, got "1:2:3:4:5".`,
		},
		{"--mpegts -1", nil, []string{"talk.vtt", "--output-dir", "new", "--mpegts", "-1"}, `The option --mpegts needs a whole number of 0 or more, got "-1".`},
		{
			"--mpegts of 2^33", nil,
			[]string{"talk.vtt", "--output-dir", "new", "--mpegts", "8589934592"},
			"The MPEGTS value must be a 33-bit timestamp from 0 to 8589934591, got 8589934592.",
		},
		{"--media-duration 0", nil, []string{"talk.vtt", "--output-dir", "new", "--media-duration", "0"}, "The option --media-duration must be greater than 0."},
		{
			"--pattern without %d", nil,
			[]string{"talk.vtt", "--output-dir", "new", "--pattern", "part.vtt"},
			`The file name pattern must hold exactly one %d and no other placeholder, got "part.vtt".`,
		},
		{"--keep-going", nil, []string{"talk.vtt", "--output-dir", "new", "--keep-going"}, "Unknown option --keep-going."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := hlsDir(t, tt.extra)
			before := snapshot(t, dir)
			assert.Equal(t, usageResult("hls", tt.msg), call{dir: dir}.run(t, append([]string{"hls"}, tt.words...)...))
			assert.Equal(t, before, snapshot(t, dir))
		})
	}
}

func TestHlsWritesNextToOtherFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		extra  map[string]string
		words  []string
		stdout string
		files  []string
	}{
		{
			"sub01.vtt is no segment of sub%d.vtt",
			map[string]string{"out/sub01.vtt": "old"},
			[]string{"talk.vtt", "--output-dir", "out/", "--media-duration", "12"},
			"talk.vtt -> out/subs.m3u8, 2 segments\n",
			[]string{"out/sub0.vtt", "out/sub1.vtt", "out/sub01.vtt", "out/subs.m3u8"},
		},
		{
			"a playlist outside the folder",
			map[string]string{},
			[]string{"talk.vtt", "--output-dir", "new", "--playlist", "../list.m3u8", "--media-duration", "6"},
			"talk.vtt -> new/../list.m3u8, 1 segments\n",
			[]string{"list.m3u8", "new/sub0.vtt"},
		},
		{
			"a pattern with a folder",
			map[string]string{"seg/other.txt": "old"},
			[]string{"talk.vtt", "--output-dir", "new", "--pattern", "../seg/%d.vtt", "--media-duration", "6"},
			"talk.vtt -> new/subs.m3u8, 1 segments\n",
			[]string{"new/subs.m3u8", "seg/0.vtt", "seg/other.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := hlsDir(t, tt.extra)
			before := snapshot(t, dir)
			assert.Equal(t, result{Stdout: tt.stdout}, call{dir: dir}.run(t, append([]string{"hls"}, tt.words...)...))
			for name := range snapshot(t, dir) {
				if _, ok := before[name]; ok {
					continue
				}
				assert.Contains(t, append(tt.files, "new", "out", "seg"), name)
			}
			for _, f := range tt.files {
				assert.FileExists(t, filepath.Join(dir, f))
			}
		})
	}
}

func TestHlsReadsStandardInput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	got := call{dir: dir, stdin: string(testfiles.Read(t, "hls/node-webvtt-subs1.vtt"))}.run(t, "hls", "-", "--output-dir", "stdin", "--segment", "10", "--media-duration", "150")
	assert.Equal(t, result{Stdout: "stdin -> stdin/subs.m3u8, 15 segments\n"}, got)
	entries, err := os.ReadDir(filepath.Join(dir, "stdin"))
	require.NoError(t, err)
	assert.Len(t, entries, 16)
}

func TestHlsFailures(t *testing.T) {
	t.Parallel()

	t.Run("a subtitle without a cue needs --media-duration", func(t *testing.T) {
		t.Parallel()

		dir := hlsDir(t, map[string]string{"empty.vtt": "WEBVTT\n"})
		before := snapshot(t, dir)
		got := call{dir: dir}.run(t, "hls", "empty.vtt", "--output-dir", "out")
		assert.Equal(t, result{Code: exitFile, Stderr: "empty.vtt: The subtitle has no cue that ends after 0 s. Pass --media-duration to segment it.\n"}, got)
		assert.Equal(t, before, snapshot(t, dir))
	})

	t.Run("a failed write removes the files of the run", func(t *testing.T) {
		t.Parallel()

		dir := hlsDir(t, nil)
		race := func(path string) {
			if path == "out/sub3.vtt" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "out", "sub3.vtt"), []byte("racer"), 0o600))
			}
		}
		got := call{dir: dir, beforeCreate: race}.run(t, "hls", "talk.vtt", "--output-dir", "out")
		assert.Equal(t, result{Code: exitFile, Stderr: "Error: out/sub3.vtt exists. Removed the 3 files that this run wrote.\n"}, got)
		assert.Equal(t, "racer", readFile(t, dir, "out/sub3.vtt"))
		entries, err := os.ReadDir(filepath.Join(dir, "out"))
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("a missing input", func(t *testing.T) {
		t.Parallel()

		dir := hlsDir(t, nil)
		got := call{dir: dir}.run(t, "hls", "missing.vtt", "--output-dir", "out")
		assert.Equal(t, result{Code: exitFile, Stderr: "missing.vtt: The file does not exist.\n"}, got)
		assert.NoDirExists(t, filepath.Join(dir, "out"))
	})
}

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"sub%d.vtt", "sub0.vtt", true},
		{"sub%d.vtt", "sub123.vtt", true},
		{"sub%d.vtt", "sub01.vtt", false},
		{"sub%d.vtt", "sub.vtt", false},
		{"sub%d.vtt", "sub-1.vtt", false},
		{"sub%d.vtt", "sub1.vtt.bak", false},
		{"part%03d.vtt", "part007.vtt", true},
		{"part%03d.vtt", "part7.vtt", false},
		{"part%03d.vtt", "part1000.vtt", true},
		{"s_%d%%.vtt", "s_4%.vtt", true},
		{"x%3d.vtt", "x  7.vtt", true},
		{"x%3d.vtt", "x7.vtt", false},
		{"x%3d.vtt", "x 007.vtt", false},
		{"%d", "12", true},
		{"sub%d.vtt", "sub99999999999999999999.vtt", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+" "+tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, matchesPattern(tt.pattern, tt.name))
		})
	}
}

func TestHlsInputOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{"--from", []string{"trip.srt", "--from", "srt"}, result{Stdout: "trip.srt -> out/subs.m3u8, 2 segments\n"}},
		{"--encoding", []string{"latin1.srt", "--encoding", "latin1"}, result{Stdout: "latin1.srt -> out/subs.m3u8, 1 segments\n"}},
		{"--input-fps", []string{"frames.sub", "--input-fps", "25"}, result{Stdout: "frames.sub -> out/subs.m3u8, 1 segments\n"}},
		{"--fps", []string{"frames.sub", "--fps", "25"}, result{Stdout: "frames.sub -> out/subs.m3u8, 1 segments\n"}},
		{"--scc-roll-up", []string{"trip.srt", "--scc-roll-up", "lines"}, result{Stdout: "trip.srt -> out/subs.m3u8, 2 segments\n"}},
		{"--lenient", []string{"broken.srt", "--lenient"}, result{Stdout: "broken.srt -> out/subs.m3u8, 1 segments\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: newDir(t)}.run(t, append([]string{"hls", "--output-dir", "out"}, tt.words...)...)
			assert.Equal(t, tt.want.Code, got.Code)
			assert.Equal(t, tt.want.Stdout, got.Stdout)
		})
	}
}
