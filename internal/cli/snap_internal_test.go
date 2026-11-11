package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
	"github.com/yama6a/subtitle-toolbox-go/v2/timing"
)

func snapDir(t *testing.T) string {
	t.Helper()

	dir := fixtureDir(t, map[string]string{
		"garden.srt": "shot-changes/own_garden_24fps.srt",
		"scenes.log": "shot-changes/own_ffmpeg_showinfo.log",
		"scenes.txt": "shot-changes/own_scenes.txt",
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("Not a time.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad2.txt"), []byte("1.5\n\n00:01:2x\n"), 0o600))
	return dir
}

func TestConvertSnap(t *testing.T) {
	t.Parallel()

	dir := snapDir(t)
	garden := []string{"convert", "garden.srt", "--to", "srt", "-o", "-"}
	timed := string(testfiles.Read(t, "shot-changes/own_garden_24fps_timed.srt"))
	chained := libraryOutput(t, "shot-changes/own_garden_24fps.srt", stbox.SubRip, stbox.WriteOptions{}, func(s *stbox.Subtitle) error {
		_, err := timing.Apply(s, timing.Options{FrameRate: model.FrameRate{Num: 24, Den: 1}})
		return err
	})
	golden := func(name string) string { return string(testfiles.Read(t, "shot-changes/"+name)) }
	tests := []struct {
		name string
		args []string
		want result
	}{
		{"log", []string{"--video-fps", "24", "--snap-shot-changes", "scenes.log"}, result{Stdout: timed}},
		{"text with --fps", []string{"--fps", "24", "--snap-shot-changes", "scenes.txt"}, result{Stdout: timed}},
		{
			"window",
			[]string{"--video-fps", "24", "--snap-shot-changes", "scenes.txt", "--snap-window-frames", "6"},
			result{Stdout: golden("own_garden_24fps_window_6.srt")},
		},
		{
			"min gap",
			[]string{"--video-fps", "24", "--snap-shot-changes", "scenes.txt", "--snap-min-gap-frames", "3"},
			result{Stdout: golden("own_garden_24fps_min_gap_3.srt")},
		},
		{
			"min duration",
			[]string{"--video-fps", "24", "--snap-shot-changes", "scenes.txt", "--snap-min-duration-frames", "12"},
			result{Stdout: golden("own_garden_24fps_min_duration_12.srt")},
		},
		{
			"no chain",
			[]string{"--video-fps", "24", "--snap-shot-changes", "scenes.txt", "--no-snap-chain"},
			result{Stdout: golden("own_garden_24fps_no_chain.srt")},
		},
		{"chain only", []string{"--fps", "24", "--snap-min-gap-frames", "2"}, result{Stdout: chained}},
		{
			"missing file",
			[]string{"--video-fps", "24", "--snap-shot-changes", "missing.txt"},
			result{Code: exitFile, Stderr: "Error: missing.txt: The file does not exist.\n"},
		},
		{
			"broken file",
			[]string{"--video-fps", "24", "--snap-shot-changes", "bad2.txt"},
			result{Code: exitFile, Stderr: "Error: bad2.txt: line 3: The shot change time \"00:01:2x\" is not valid. Use seconds or hh:mm:ss.mmm.\n"},
		},
		{
			"directory as file",
			[]string{"--video-fps", "24", "--snap-shot-changes", "."},
			result{Code: exitFile, Stderr: "Error: .: The file does not exist.\n"},
		},
		{
			"video-fps alone",
			[]string{"--video-fps", "24"},
			usageResult("convert", "Pass --snap-shot-changes FILE with --video-fps."),
		},
		{
			"no rate",
			[]string{"--snap-shot-changes", "scenes.txt"},
			usageResult("convert", "Pass --video-fps RATE or --fps RATE with --snap-shot-changes."),
		},
		{
			"input-fps is no video rate",
			[]string{"--input-fps", "24", "--snap-window-frames", "6"},
			usageResult("convert", "Pass --video-fps RATE or --fps RATE with --snap-window-frames."),
		},
		{
			"no chain without shots",
			[]string{"--video-fps", "24", "--no-snap-chain"},
			usageResult("convert", "Pass --snap-shot-changes FILE. With --no-snap-chain and no shot changes, snapping changes nothing."),
		},
		{
			"negative window",
			[]string{"--video-fps", "24", "--snap-window-frames", "-1"},
			usageResult("convert", `The option --snap-window-frames needs a whole number of 0 or more, got "-1".`),
		},
		{
			"bad min gap",
			[]string{"--video-fps", "24", "--snap-min-gap-frames", "x"},
			usageResult("convert", `The option --snap-min-gap-frames needs a whole number of 0 or more, got "x".`),
		},
		{
			"bad min duration",
			[]string{"--video-fps", "24", "--snap-min-duration-frames", "1.5"},
			usageResult("convert", `The option --snap-min-duration-frames needs a whole number of 0 or more, got "1.5".`),
		},
		{
			"bad video rate",
			[]string{"--video-fps", "0", "--snap-window-frames", "6"},
			usageResult("convert", "The option --video-fps must be greater than 0."),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: dir}.run(t, append(slicesClone(garden), tt.args...)...))
		})
	}
}

func TestConvertSnapNeverReplacesTheShotChangeFile(t *testing.T) {
	t.Parallel()

	dir := snapDir(t)
	copyFile(t, filepath.Join(dir, "scenes.txt"), filepath.Join(dir, "scenes.srt"))

	got := call{dir: dir}.run(t, "convert", "garden.srt", "-o", "scenes.srt", "--video-fps", "24", "--snap-shot-changes", "scenes.srt")

	assert.Equal(t, usageResult("convert", "The output scenes.srt is a file that the command reads. Pass another output file or directory."), got)
}

func slicesClone(s []string) []string { return append([]string(nil), s...) }

func TestConvertSnapTakesTheVideoAndInputRatesApart(t *testing.T) {
	t.Parallel()

	dir := snapDir(t)
	garden, _, err := stbox.Load(filepath.Join(dir, "garden.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	var sub bytes.Buffer
	_, err = stbox.Write(&sub, garden, stbox.MicroDVD, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: model.FrameRate{Num: 25, Den: 1}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "garden.sub"), sub.Bytes(), 0o600))
	shots, err := timing.ShotChangesFromFFmpegLog(testfiles.Read(t, "shot-changes/own_ffmpeg_showinfo.log"))
	require.NoError(t, err)
	want := func(inputFPS int64) string {
		s, _, err := stbox.Parse(bytes.NewReader(sub.Bytes()), stbox.ReadOptions{
			Format: stbox.MicroDVD, MicroDVD: microdvd.ReadOptions{FrameRate: model.FrameRate{Num: inputFPS, Den: 1}},
		})
		require.NoError(t, err)
		_, err = timing.Apply(s, timing.Options{FrameRate: model.FrameRate{Num: 24, Den: 1}, ShotChanges: shots})
		require.NoError(t, err)
		var out bytes.Buffer
		_, err = stbox.Write(&out, s, stbox.SubRip, stbox.WriteOptions{})
		require.NoError(t, err)
		return out.String()
	}

	apart := call{dir: dir}.run(t, "convert", "garden.sub", "--video-fps", "24", "--input-fps", "25", "--snap-shot-changes", "scenes.log", "--to", "srt", "-o", "-")
	same := call{dir: dir}.run(t, "convert", "garden.sub", "--fps", "24", "--snap-shot-changes", "scenes.log", "--to", "srt", "-o", "-")

	assert.Equal(t, result{Stdout: want(25)}, apart)
	assert.Equal(t, result{Stdout: want(24)}, same)
	assert.NotEqual(t, apart.Stdout, same.Stdout)
}
