package hls_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/hls"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestSegment(t *testing.T) {
	t.Parallel()

	const header = "WEBVTT\n" + defaultMap + "\n\n"
	ferryOne := "2\n00:00:05.000 --> 00:00:07.500\nPlease keep your tickets ready.\n"
	tests := []struct {
		name      string
		sub       *model.Subtitle
		opts      hls.SegmentOptions
		names     []string
		contents  map[int]string
		durations []string
	}{
		{
			name:  "default options",
			sub:   ferry(),
			names: []string{"sub0.vtt", "sub1.vtt", "sub2.vtt", "sub3.vtt"},
			contents: map[int]string{
				0: header + "1\n00:00:01.000 --> 00:00:03.000\nThe ferry leaves at noon.\n\n" + ferryOne,
				1: header + ferryOne,
				2: header,
				3: header + "3\n00:00:18.500 --> 00:00:20.000\nThank you for travelling with us.\n",
			},
			durations: []string{"6.000", "6.000", "6.000", "2.000"},
		},
		{
			name:      "a target duration of 6.006 s rounds down",
			sub:       ferry(),
			opts:      hls.SegmentOptions{SegmentDuration: ms(6006)},
			names:     []string{"sub0.vtt", "sub1.vtt", "sub2.vtt", "sub3.vtt"},
			durations: []string{"6.006", "6.006", "6.006", "1.982"},
		},
		{
			name:      "a segment of 6.5 s rounds the target duration up",
			sub:       ferry(),
			opts:      hls.SegmentOptions{SegmentDuration: ms(6500)},
			names:     []string{"sub0.vtt", "sub1.vtt", "sub2.vtt", "sub3.vtt"},
			durations: []string{"6.500", "6.500", "6.500", "0.500"},
		},
		{
			name:      "a media duration adds empty segments",
			sub:       ferry(),
			opts:      hls.SegmentOptions{MediaDuration: 31 * time.Second},
			names:     []string{"sub0.vtt", "sub1.vtt", "sub2.vtt", "sub3.vtt", "sub4.vtt", "sub5.vtt"},
			contents:  map[int]string{5: header},
			durations: []string{"6.000", "6.000", "6.000", "6.000", "6.000", "1.000"},
		},
		{
			name:      "a shorter media duration cuts the last segment",
			sub:       ferry(),
			opts:      hls.SegmentOptions{MediaDuration: 7 * time.Second},
			names:     []string{"sub0.vtt", "sub1.vtt"},
			contents:  map[int]string{1: header + ferryOne},
			durations: []string{"6.000", "1.000"},
		},
		{
			name: "the map, the LOCAL time and a pattern with a folder",
			sub:  ferry(),
			opts: hls.SegmentOptions{
				SegmentDuration: 10 * time.Second, Map: &hls.TimestampMap{MPEGTS: 181083, Local: time.Hour},
				FileNamePattern: "text/seg_%03d.webvtt",
			},
			names: []string{"text/seg_000.webvtt", "text/seg_001.webvtt"},
			contents: map[int]string{
				1: "WEBVTT\nX-TIMESTAMP-MAP=LOCAL:01:00:00.000,MPEGTS:181083\n\n3\n01:00:18.500 --> 01:00:20.000\nThank you for travelling with us.\n",
			},
			durations: []string{"10.000", "10.000"},
		},
		{
			name:  "MPEGTS 0 is legal and %% stays a percent sign",
			sub:   build(cue{ms(1000), ms(2000), "One"}),
			opts:  hls.SegmentOptions{Map: &hls.TimestampMap{}, FileNamePattern: "s_%d%%.vtt"},
			names: []string{"s_0%.vtt"},
			contents: map[int]string{
				0: "WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:0\n\n1\n00:00:01.000 --> 00:00:02.000\nOne\n",
			},
			durations: []string{"2.000"},
		},
		{
			name:  "a cue of zero length goes to the segment that it starts in",
			sub:   build(cue{ms(6000), ms(6000), "Mark"}, cue{ms(8000), ms(9000), "End"}),
			names: []string{"sub0.vtt", "sub1.vtt"},
			contents: map[int]string{
				0: header,
				1: header + "1\n00:00:06.000 --> 00:00:06.000\nMark\n\n2\n00:00:08.000 --> 00:00:09.000\nEnd\n",
			},
			durations: []string{"6.000", "3.000"},
		},
		{
			name: "an empty subtitle with a media duration",
			sub:  &model.Subtitle{},
			opts: hls.SegmentOptions{MediaDuration: 4 * time.Second}, names: []string{"sub0.vtt"},
			contents:  map[int]string{0: header},
			durations: []string{"4.000"},
		},
		{
			name: "the file data stays and an old map goes",
			sub: readVTT(t, "WEBVTT Ferry\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:0\nKind: captions\n\nNOTE gone\n\n"+
				"STYLE\n::cue { color: yellow }\n\nintro\n00:00:01.000 --> 00:00:02.000 line:0\n<i>Welcome aboard.</i>\n"),
			names: []string{"sub0.vtt"},
			contents: map[int]string{
				0: "WEBVTT Ferry\n" + defaultMap + "\nKind: captions\n\nSTYLE\n::cue { color: yellow }\n\n" +
					"intro\n00:00:01.000 --> 00:00:02.000 line:0\n<i>Welcome aboard.</i>\n",
			},
			durations: []string{"2.000"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, err := hls.Segment(tt.sub, tt.opts)
			require.NoError(t, err)
			names, contents := segments(t, r)
			assert.Equal(t, tt.names, names)
			assert.Equal(t, len(tt.names), r.Count())
			for i, want := range tt.contents {
				assert.Equal(t, want, contents[i], tt.names[i])
			}
			durations := r.Durations()
			require.Len(t, durations, len(tt.durations))
			for i, d := range durations {
				assert.Equal(t, tt.names[i], d.Name)
				assert.Equal(t, tt.durations[i], strconv.FormatFloat(d.Duration.Seconds(), 'f', 3, 64))
			}
			target, extinfs := readPlaylist(t, string(r.Playlist()))
			assert.Equal(t, tt.durations, extinfs)
			assertTargetRule(t, target, extinfs)
		})
	}
}

var (
	targetRegexp = regexp.MustCompile(`(?m)^#EXT-X-TARGETDURATION:(\d+)$`)
	extinfRegexp = regexp.MustCompile(`(?m)^#EXTINF:([\d.]+),?$`)
)

// readPlaylist returns the target duration and the EXTINF values of a media playlist.
func readPlaylist(tb testing.TB, playlist string) (target int, extinfs []string) {
	tb.Helper()

	m := targetRegexp.FindStringSubmatch(playlist)
	require.NotNil(tb, m, playlist)
	target, _ = strconv.Atoi(m[1])
	for _, m := range extinfRegexp.FindAllStringSubmatch(playlist, -1) {
		extinfs = append(extinfs, m[1])
	}
	return target, extinfs
}

// assertTargetRule checks RFC 8216 section 4.3.3.1: each EXTINF value, rounded to the nearest integer, must not
// exceed the target duration.
func assertTargetRule(tb testing.TB, target int, extinfs []string) {
	tb.Helper()

	require.NotEmpty(tb, extinfs)
	for _, v := range extinfs {
		f, err := strconv.ParseFloat(v, 64)
		require.NoError(tb, err)
		assert.LessOrEqual(tb, int(math.Round(f)), target, v)
	}
}

func TestSegmentPlaylist(t *testing.T) {
	t.Parallel()

	r, err := hls.Segment(ferry(), hls.SegmentOptions{})
	require.NoError(t, err)
	assert.Equal(t, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:0\n"+
		"#EXT-X-PLAYLIST-TYPE:VOD\n#EXTINF:6.000,\nsub0.vtt\n#EXTINF:6.000,\nsub1.vtt\n"+
		"#EXTINF:6.000,\nsub2.vtt\n#EXTINF:2.000,\nsub3.vtt\n#EXT-X-ENDLIST\n", string(r.Playlist()))
}

func TestSegmentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sub    *model.Subtitle
		opts   hls.SegmentOptions
		option string
		msg    string
	}{
		{"segment duration below 1 ms", ferry(), hls.SegmentOptions{SegmentDuration: 400 * time.Microsecond}, "hls.SegmentOptions.SegmentDuration", "The segment duration must be at least 0.001 s, got 0.0004."},
		{"negative segment duration", ferry(), hls.SegmentOptions{SegmentDuration: -time.Second}, "hls.SegmentOptions.SegmentDuration", "The segment duration must be at least 0.001 s, got -1."},
		{"media duration below 1 ms", ferry(), hls.SegmentOptions{MediaDuration: 100 * time.Microsecond}, "hls.SegmentOptions.MediaDuration", "The media duration must be at least 0.001 s, got 0.0001."},
		{"map with a negative LOCAL time", ferry(), hls.SegmentOptions{Map: &hls.TimestampMap{Local: -time.Second}}, "hls.TimestampMap.Local", "The LOCAL time must not be negative, got -1."},
		{"map with MPEGTS 2^33", ferry(), hls.SegmentOptions{Map: &hls.TimestampMap{MPEGTS: hls.MPEGTSWrap}}, "hls.TimestampMap.MPEGTS", "The MPEGTS value must be a 33-bit timestamp from 0 to 8589934591, got 8589934592."},
		{"pattern without a verb", ferry(), hls.SegmentOptions{FileNamePattern: "sub.vtt"}, "hls.SegmentOptions.FileNamePattern", `The file name pattern must hold exactly one %d and no other placeholder, got "sub.vtt".`},
		{"pattern with %s", ferry(), hls.SegmentOptions{FileNamePattern: "%s%d.vtt"}, "hls.SegmentOptions.FileNamePattern", `The file name pattern must hold exactly one %d and no other placeholder, got "%s%d.vtt".`},
		{"pattern with 2 verbs", ferry(), hls.SegmentOptions{FileNamePattern: "%d_%d.vtt"}, "hls.SegmentOptions.FileNamePattern", `The file name pattern must hold exactly one %d and no other placeholder, got "%d_%d.vtt".`},
		{"pattern with a flag", ferry(), hls.SegmentOptions{FileNamePattern: "%-3d.vtt"}, "hls.SegmentOptions.FileNamePattern", `The file name pattern must hold exactly one %d and no other placeholder, got "%-3d.vtt".`},
		{"empty subtitle", &model.Subtitle{}, hls.SegmentOptions{}, "hls.SegmentOptions.MediaDuration", "The subtitle has no cue that ends after 0 s. Set the mediaDuration option to segment it."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := hls.Segment(tt.sub, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var ierr *model.InvalidArgumentError
			require.ErrorAs(t, err, &ierr)
			assert.Equal(t, tt.option, ierr.Option)
			assert.Equal(t, tt.msg, ierr.Msg)
		})
	}
}

func TestEachSegmentStopsOnTheErrorOfFn(t *testing.T) {
	t.Parallel()

	r, err := hls.Segment(ferry(), hls.SegmentOptions{})
	require.NoError(t, err)
	stop := errors.New("stop")
	var calls int
	err = r.EachSegment(func(string, []byte) error {
		calls++
		return stop
	})
	require.ErrorIs(t, err, stop)
	assert.Equal(t, 1, calls)
}

func TestRealPlaylistsKeepTheTargetDurationRule(t *testing.T) {
	t.Parallel()

	for file, wantTarget := range map[string]int{"shaka-playlist-vtt.m3u8": 2, "own-prog_index.m3u8": 6} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			target, extinfs := readPlaylist(t, string(testfiles.Read(t, "hls/"+file)))
			assert.Equal(t, wantTarget, target)
			assertTargetRule(t, target, extinfs)
		})
	}
}

func TestSegmentGoldens(t *testing.T) {
	t.Parallel()

	input := readFixture(t)
	tests := []struct {
		folder string
		local  time.Duration
	}{
		{"node-webvtt-subs1", 0},
		{"node-webvtt-subs1-local", 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.folder, func(t *testing.T) {
			t.Parallel()

			r, err := hls.Segment(input, hls.SegmentOptions{
				SegmentDuration: 10 * time.Second, Map: &hls.TimestampMap{MPEGTS: 126000, Local: tt.local},
				FileNamePattern: "part%03d.vtt", MediaDuration: 150 * time.Second,
			})
			require.NoError(t, err)
			dir := testfiles.Path(t, "hls/expected/"+tt.folder)
			names, contents := segments(t, r)
			for i, name := range names {
				golden.Equal(t, filepath.Join(dir, name), []byte(contents[i]), *update)
			}
			golden.Equal(t, filepath.Join(dir, "index.m3u8"), r.Playlist(), *update)

			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			assert.Len(t, entries, len(names)+1)
		})
	}
}
