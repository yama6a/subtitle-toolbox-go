package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

// videoFiles lists the fixtures that videoDir copies next to the files of testdata/cli.
var videoFiles = []string{
	"mkv/text_tracks.mkv", "mkv/seek_head.mkv", "mkv/pgs.mkv", "mkv/mkvmerge.mkv", "mkv/ffmpeg_webvtt.webm",
	"mp4/text_tracks.mp4", "mp4/one_track.mp4", "mp4/ffmpeg_mov_text.mp4", "mp4/ffmpeg_c608.mov",
	"mpegts/teletext.ts", "pgs/text_1080p.sup", "vobsub/text-pal.idx", "vobsub/text-pal.sub",
}

func videoDir(t *testing.T) string {
	t.Helper()

	dir := newDir(t)
	for _, f := range videoFiles {
		copyFile(t, testfiles.Path(t, f), filepath.Join(dir, filepath.Base(f)))
	}
	return dir
}

// libraryWrite returns the library write of a track of a testdata file, with default options.
func libraryWrite(t *testing.T, rel string, track int, f stbox.Format) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, rel), stbox.ReadOptions{Track: track})
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = stbox.Write(&buf, s, f, stbox.WriteOptions{})
	require.NoError(t, err)
	return buf.String()
}

func TestDescribeTrack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		track stbox.Track
		want  string
	}{
		{"codec only", stbox.Track{CodecID: "S_TEXT/UTF8"}, "S_TEXT/UTF8"},
		{"both flags", stbox.Track{CodecID: "S_HDMV/PGS", Language: "eng", Default: true, Forced: true}, "S_HDMV/PGS, eng, default, forced"},
		{"name", stbox.Track{CodecID: "tx3g", Language: "eng", Name: "English"}, `tx3g, eng, "English"`},
		{"name without language", stbox.Track{CodecID: "tx3g", Name: "Cast"}, `tx3g, "Cast"`},
		{"quote in name", stbox.Track{CodecID: "tx3g", Name: `a "b" <c>`}, `tx3g, "a \"b\" <c>"`},
		{"invalid UTF-8 in name", stbox.Track{CodecID: "tx3g", Name: "a\xffb"}, "tx3g, \"a�b\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, describeTrack(tt.track))
		})
	}
}

func TestTrackErrorMessage(t *testing.T) {
	t.Parallel()

	two := []stbox.Track{{Number: 2, CodecID: "tx3g", Language: "eng"}, {Number: 3, CodecID: "c608", Default: true}}
	tests := []struct {
		name   string
		err    *stbox.TrackError
		option string
		want   string
	}{
		{"no track", &stbox.TrackError{Container: stbox.MP4}, "--track", "The MP4 file has no subtitle track."},
		{
			"track", &stbox.TrackError{Container: stbox.MP4, Tracks: two}, "--track",
			"The MP4 file has 2 subtitle tracks. Pass --track N with one of them:\n  2: tx3g, eng\n  3: c608, default",
		},
		{
			"new track", &stbox.TrackError{Container: stbox.MPEGTS, Tracks: two}, "--new-track",
			"The MPEG-TS file has 2 subtitle tracks. Pass --new-track N with one of them:\n  2: tx3g, eng\n  3: c608, default",
		},
		{
			"reference track", &stbox.TrackError{Container: stbox.Matroska, Tracks: two}, "--reference-track",
			"The MKV or WebM file has 2 subtitle tracks. Pass --reference-track N with one of them:\n  2: tx3g, eng\n  3: c608, default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, trackErrorMessage(tt.err, tt.option))
		})
	}
}

func TestConvertReadsATrack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file   string
		rel    string
		track  int
		format stbox.Format
	}{
		{"mkvmerge.mkv", "mkv/mkvmerge.mkv", 2, stbox.SubRip},
		{"ffmpeg_webvtt.webm", "mkv/ffmpeg_webvtt.webm", 2, stbox.WebVTT},
		{"ffmpeg_mov_text.mp4", "mp4/ffmpeg_mov_text.mp4", 2, stbox.SubRip},
		{"ffmpeg_c608.mov", "mp4/ffmpeg_c608.mov", 1, stbox.SubRip},
		{"teletext.ts", "mpegts/teletext.ts", 888, stbox.SubRip},
		{"pgs.mkv", "mkv/pgs.mkv", 4, stbox.PGS},
	}
	dir := videoDir(t)
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, "convert", tt.file, "--track", strconv.Itoa(tt.track), "--to", string(tt.format), "-o", "-")
			assert.Equal(t, 0, got.Code, got.Stderr)
			assert.Equal(t, libraryWrite(t, tt.rel, tt.track, tt.format), got.Stdout)
		})
	}
}

func TestConvertTrackInputAndOutputNames(t *testing.T) {
	t.Parallel()

	dir := videoDir(t)
	got := call{dir: dir}.run(t, "convert", "text_tracks.mkv", "--to", "srt", "--track", "3", "--output-dir", "out")
	assert.Equal(t, result{Stdout: "text_tracks.mkv -> out/text_tracks.srt\n"}, got)
	assert.Equal(t, libraryWrite(t, "mkv/text_tracks.mkv", 3, stbox.SubRip), readFile(t, dir, "out/text_tracks.srt"))

	got = call{dir: dir}.run(t, "convert", "text_tracks.mkv", "--track", "8", "--output-dir", "keep")
	assert.Equal(t, result{Stdout: "text_tracks.mkv -> keep/text_tracks.srt\n"}, got)

	got = call{dir: dir}.run(t, "convert", "ffmpeg_c608.mov", "--track", "1", "-o", "-")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, libraryWrite(t, "mp4/ffmpeg_c608.mov", 1, stbox.SCC), got.Stdout)

	got = call{dir: dir, stdin: string(testfiles.Read(t, "mkv/text_tracks.mkv"))}.run(t, "convert", "-", "--to", "vtt", "--track", "5", "-o", "-")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, libraryWrite(t, "mkv/text_tracks.mkv", 5, stbox.WebVTT), got.Stdout)

	got = call{dir: dir}.run(t, "convert", "seek_head.mkv", "--to", "vtt", "-o", "one.vtt")
	assert.Equal(t, result{Stdout: "seek_head.mkv -> one.vtt\n"}, got)
	assert.Equal(t, libraryWrite(t, "mkv/seek_head.mkv", 0, stbox.WebVTT), readFile(t, dir, "one.vtt"))
}

func TestTrackReadFailures(t *testing.T) {
	t.Parallel()

	mkvTracks := "The MKV or WebM file has 6 subtitle tracks. Pass %s N with one of them:\n" +
		"  3: S_TEXT/UTF8, de, \"Deutsch (Forced)\", forced\n  4: S_TEXT/ASS, eng, \"English\", default\n" +
		"  5: S_TEXT/WEBVTT, fre, \"Français\"\n  6: S_TEXT/SSA, spa\n  7: S_DVBSUB, ita\n  8: S_TEXT/UTF8, eng\n"
	tests := []struct {
		name  string
		words []string
		want  result
	}{
		{
			"many tracks",
			[]string{"convert", "text_tracks.mkv", "--to", "srt", "-o", "-"},
			result{Code: exitFile, Stderr: "text_tracks.mkv: " + strings.Replace(mkvTracks, "%s", "--track", 1)},
		},
		{
			"unknown track",
			[]string{"convert", "text_tracks.mkv", "--to", "srt", "--track", "9", "-o", "-"},
			result{Code: exitFile, Stderr: "text_tracks.mkv: The file has no subtitle track with the number 9.\n"},
		},
		{
			"track of a subtitle file",
			[]string{"convert", "trip.srt", "--to", "vtt", "--track", "3", "-o", "-"},
			result{Code: exitFile, Stderr: "trip.srt: Only MKV, WebM, MP4 and MPEG-TS files have tracks.\n"},
		},
		{
			"from with an MP4 file",
			[]string{"convert", "text_tracks.mp4", "--from", "srt", "-o", "-"},
			result{Code: exitFile, Stderr: "text_tracks.mp4: The input is an MP4 file. Pass --track N.\n"},
		},
		{
			"from with an MPEG-TS file",
			[]string{"convert", "teletext.ts", "--from", "srt", "-o", "-"},
			result{Code: exitFile, Stderr: "teletext.ts: The input is an MPEG-TS file. Pass --track N.\n"},
		},
		{
			"encrypted track",
			[]string{"convert", "text_tracks.mp4", "--track", "5", "--to", "srt", "-o", "-"},
			result{Code: exitFile, Stderr: "text_tracks.mp4: Track 5 has the codec enct and is encrypted.\n" + lenientHint + "\n"},
		},
		{
			"sync reference",
			[]string{"sync", "trip.srt", "--reference", "text_tracks.mkv"},
			result{Code: exitFile, Stderr: "Error: text_tracks.mkv: " + strings.Replace(mkvTracks, "%s", "--reference-track", 1)},
		},
		{
			"diff new file",
			[]string{"diff", "trip.srt", "text_tracks.mkv"},
			result{Code: exitFile, Stderr: "text_tracks.mkv: " + strings.Replace(mkvTracks, "%s", "--new-track", 1)},
		},
		{
			"dual primary",
			[]string{"dual", "--primary", "text_tracks.mkv", "--secondary", "trip.srt"},
			result{Code: exitFile, Stderr: "text_tracks.mkv: " + strings.Replace(mkvTracks, "%s", "--primary-track", 1)},
		},
		{
			"dual secondary",
			[]string{"dual", "--primary", "trip.srt", "--secondary", "text_tracks.mkv"},
			result{Code: exitFile, Stderr: "text_tracks.mkv: " + strings.Replace(mkvTracks, "%s", "--secondary-track", 1)},
		},
	}
	dir := videoDir(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, call{dir: dir}.run(t, tt.words...))
		})
	}
}

func TestTrackOptionValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		words []string
		want  string
	}{
		{[]string{"convert", "trip.srt", "--track", "V"}, "The option --track needs a whole number of 1 or more, got \"V\"."},
		{[]string{"info", "trip.srt", "--track", "0"}, "The option --track needs a whole number of 1 or more, got \"0\"."},
		{[]string{"sync", "trip.srt", "--reference", "shop.vtt", "--reference-track", "x"}, "The option --reference-track needs a whole number of 1 or more, got \"x\"."},
		{[]string{"diff", "trip.srt", "shop.vtt", "--new-track", "-1"}, "The option --new-track needs a whole number of 1 or more, got \"-1\"."},
		{[]string{"dual", "--primary", "trip.srt", "--secondary", "shop.vtt", "--primary-track", "x"}, "The option --primary-track needs a whole number of 1 or more, got \"x\"."},
		{[]string{"dual", "--primary", "trip.srt", "--secondary", "shop.vtt", "--secondary-track", "x"}, "The option --secondary-track needs a whole number of 1 or more, got \"x\"."},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.words, " "), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, usageResult(tt.words[0], tt.want), runCLI(t, tt.words...))
		})
	}
}

func TestTracksOnEveryReadingCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		code  int
	}{
		{"validate", []string{"validate", "text_tracks.mkv", "--track", "8", "--preset", "bbc"}, 0},
		{"hls", []string{"hls", "text_tracks.mkv", "--track", "8", "--output-dir", "hls"}, 0},
		{"sync reference", []string{"sync", "trip.srt", "--reference", "text_tracks.mkv", "--reference-track", "8"}, 0},
		{"sync input", []string{"sync", "text_tracks.mkv", "--track", "8", "--reference", "trip.srt", "--skip-image-cues", "-o", "-"}, 0},
		{"sync image reference", []string{"sync", "trip.srt", "--reference", "pgs.mkv", "--reference-track", "3"}, 0},
		{"diff both tracks", []string{"diff", "text_tracks.mkv", "text_tracks.mkv", "--track", "8"}, 0},
		{"diff new track", []string{"diff", "trip.srt", "text_tracks.mkv", "--new-track", "8"}, 1},
		{"diff inherited track", []string{"diff", "text_tracks.mkv", "trip.srt", "--track", "8"}, 1},
		{
			"dual", []string{
				"dual", "--primary", "text_tracks.mkv", "--primary-track", "8", "--secondary", "text_tracks.mkv", "--secondary-track", "3",
				"--skip-image-cues",
			}, 0,
		},
	}
	dir := videoDir(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := call{dir: dir}.run(t, tt.words...)
			assert.Equal(t, tt.code, got.Code, got.Stderr)
			assert.NotContains(t, got.Stderr, "Error")
		})
	}
}

func TestInfoListsTracks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		words []string
		stdin string
		want  string
	}{
		{
			"mp4",
			[]string{"info", "text_tracks.mp4"},
			"",
			"text_tracks.mp4\n  Container: mp4\n  Track 2: tx3g, eng, \"English\", default\n  Track 3: tx3g, fr-CA, forced\n" +
				"  Track 4: c608, eng\n  Track 5: enct, deu\n",
		},
		{
			"mpegts",
			[]string{"info", "teletext.ts"},
			"",
			"teletext.ts\n  Container: mpegts\n  Track 888: dvb_teletext, deu\n  Track 777: dvb_teletext, deu, \"hearing impaired\"\n" +
				"  Track 801: dvb_teletext, eng\n  Track 802: dvb_teletext, pol\n  Track 301: dvb_teletext, eng\n",
		},
		{
			"pgs on stdin",
			[]string{"info", "-"},
			"pgs",
			"stdin\n  Container: matroska\n  Track 3: S_HDMV/PGS, ger, default\n  Track 4: S_HDMV/PGS, eng, default, forced\n",
		},
		{
			"two files",
			[]string{"info", "pgs.mkv", "one_track.mp4"},
			"",
			"pgs.mkv\n  Container: matroska\n  Track 3: S_HDMV/PGS, ger, default\n  Track 4: S_HDMV/PGS, eng, default, forced\n" +
				"\none_track.mp4\n  Container: mp4\n  Track 2: tx3g, eng, default\n",
		},
	}
	dir := videoDir(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdin := tt.stdin
			if stdin == "pgs" {
				stdin = string(testfiles.Read(t, "mkv/pgs.mkv"))
			}
			got := call{dir: dir, stdin: stdin}.run(t, tt.words...)
			assert.Equal(t, 0, got.Code, got.Stderr)
			assert.True(t, strings.HasPrefix(got.Stdout, tt.want), got.Stdout)
		})
	}
}

func TestInfoJSONOfAContainer(t *testing.T) {
	t.Parallel()

	got := call{dir: videoDir(t), stdin: string(testfiles.Read(t, "mkv/pgs.mkv"))}.run(t, "info", "-", "--json")
	want := `[
    {
        "file": "-",
        "container": "matroska",
        "tracks": [
            {
                "number": 3,
                "codecId": "S_HDMV/PGS",
                "language": "ger",
                "name": null,
                "default": true,
                "forced": false
            },
            {
                "number": 4,
                "codecId": "S_HDMV/PGS",
                "language": "eng",
                "name": null,
                "default": true,
                "forced": true
            }
        ]
    }
]
`
	assert.Equal(t, result{Stdout: want}, got)
}

func TestInfoOfATrackShowsImageCues(t *testing.T) {
	t.Parallel()

	dir := videoDir(t)
	got := call{dir: dir}.run(t, "info", "pgs.mkv", "--track", "4")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "  Format:                pgs\n")
	assert.Regexp(t, `(?m)^  Image cues: +\d+, 0 with text$`, got.Stdout)

	got = call{dir: dir}.run(t, "info", "pgs.mkv", "--track", "4", "--json")
	assert.Regexp(t, `"imageCues": \{\s+"count": \d+,\s+"withText": 0\s+\}`, got.Stdout)
}

func TestImageCuesNeedAnAnswer(t *testing.T) {
	t.Parallel()

	dir := videoDir(t)
	got := call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "-o", "-")
	assert.Equal(t, result{Code: exitFile, Stderr: "text_1080p.sup: Cue #0 holds an image but no text, and srt needs text. " +
		"Pass --ocr to read the text, --to pgs to keep the images, or --skip-image-cues to leave them out.\n"}, got)

	got = call{dir: dir}.run(t, "convert", "text_1080p.sup", "--to", "srt", "--skip-image-cues", "-o", "-")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, "\ufeff", got.Stdout)

	got = call{dir: dir}.run(t, "convert", "text-pal.idx", "--to", "sup", "-o", "-")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Equal(t, libraryWriteOf(t, "vobsub/text-pal.idx", stbox.PGS), got.Stdout)
}

func libraryWriteOf(t *testing.T, rel string, f stbox.Format) string {
	t.Helper()

	return libraryWrite(t, rel, 0, f)
}

func TestConvertReadsALargeWebM(t *testing.T) {
	t.Parallel()

	const voidSize = 60 << 20
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "large.webm"))
	require.NoError(t, err)
	seekHead := testfiles.Read(t, "mkv/seek_head.mkv")
	// A Void element (ID 0xEC) with a 4-byte size of 60 MiB.
	voidHeader := []byte{0xEC, 0x13, 0xC0, 0x00, 0x00}
	_, err = file.Write(append(seekHead, voidHeader...))
	require.NoError(t, err)
	require.NoError(t, file.Truncate(int64(len(seekHead)+len(voidHeader)+voidSize)))
	require.NoError(t, file.Close())

	want := libraryWrite(t, "mkv/seek_head.mkv", 2, stbox.SubRip)
	for _, options := range [][]string{{"--fps", "25"}, {"--input-fps", "25"}, nil} {
		got := call{dir: dir}.run(t, append([]string{"convert", "large.webm", "--to", "srt", "-o", "-"}, options...)...)
		assert.Equal(t, 0, got.Code, got.Stderr)
		assert.Equal(t, want, got.Stdout)
	}
}
