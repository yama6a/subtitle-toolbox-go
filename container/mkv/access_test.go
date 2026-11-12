package mkv_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const mkvmergeApp = "mkvmerge v99.0 ('Buka') 64-bit"

// cuedFile writes a file with a video track 1 and an S_TEXT/UTF8 track 2 with the TrackUID 2000. Cluster N starts at
// N * 2000 ms and holds a video block and the block "Cue N." of track 2. The Cues list the track 2 blocks of the
// clusters in listed. A NUMBER_OF_FRAMES tag follows them, unless frames is "". The SeekHead points to Cues and Tags.
// replaced holds cluster data in place of cluster N.
func cuedFile(writingApp string, listed []int, frames string, replaced map[int][]byte) []byte {
	head := cat(info(1000000, writingApp), tracks(
		trackEntry(trackFields{number: 1, kind: trackVideo, codecID: "V_UNCOMPRESSED"}), subTrack(2, "S_TEXT/UTF8")))
	position := uint64(len(seekHead([][2]uint64{{0x1C53BB6B, 0}, {0x1254C367, 0}})) + len(head))
	var clusters []byte
	var points [][4]uint64
	for n := 1; n <= 3; n++ {
		ts := uint64(n * 2000)
		video := cat(uintElement(0xE7, ts), simpleBlock(1, 0, "\x00\x01", 0))
		data := replaced[n]
		if data == nil {
			data = cat(video, blockGroup(2, 500, "Cue "+string(rune('0'+n))+".", 1000))
		}
		for _, l := range listed {
			if l == n {
				points = append(points, [4]uint64{ts + 500, 2, position, uint64(len(video))})
			}
		}
		c := element(idCluster, data)
		clusters = cat(clusters, c)
		position += uint64(len(c))
	}
	cuesBytes := cues(points)
	var tags []byte
	if frames != "" {
		tags = frameCountTags(2000, frames)
	}
	sh := seekHead([][2]uint64{{0x1C53BB6B, position}, {0x1254C367, position + uint64(len(cuesBytes))}})
	return segmentFile("matroska", sh, head, clusters, cuesBytes, tags)
}

func TestCuesFastPath(t *testing.T) {
	t.Parallel()

	unlisted := cat(uintElement(0xE7, 4000), element(0xA3, []byte("\x82\x00")))
	videoOnly := cat(uintElement(0xE7, 4000), simpleBlock(1, 0, "\x00\x01", 0), simpleBlock(1, 1, "\x00\x01", 0))
	cue := func(n int) cueRow {
		start := time.Duration(n*2000+500) * time.Millisecond
		return cueRow{start, start + time.Second, "Cue " + string(rune('0'+n)) + "."}
	}
	tests := []struct {
		name string
		file []byte
		want []cueRow
	}{
		{
			"reads only the blocks that the Cues list", cuedFile(mkvmergeApp, []int{1, 3}, "2", map[int][]byte{2: unlisted}),
			[]cueRow{cue(1), cue(3)},
		},
		{"NUMBER_OF_FRAMES larger than the Cues", cuedFile(mkvmergeApp, []int{1, 3}, "3", nil), []cueRow{cue(1), cue(2), cue(3)}},
		{"no NUMBER_OF_FRAMES tag", cuedFile(mkvmergeApp, []int{1, 3}, "", nil), []cueRow{cue(1), cue(2), cue(3)}},
		{"file from ffmpeg", cuedFile("Lavf61.1.100", []int{1, 3}, "2", nil), []cueRow{cue(1), cue(2), cue(3)}},
		{
			"listed position without a block of the track", cuedFile(mkvmergeApp, []int{1, 2, 3}, "3", map[int][]byte{2: videoOnly}),
			[]cueRow{cue(1), cue(3)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := openFile(t, tt.file).Extract(2, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

func TestExtractErrors(t *testing.T) {
	t.Parallel()

	aribFile := segmentFile("matroska", tracks(subTrack(2, "S_ARIBSUB")))
	tests := []struct {
		name  string
		file  []byte
		track int
		want  string
	}{
		{"codec without a reader", aribFile, 2, "Track 2 has the codec S_ARIBSUB. The reader extracts only S_TEXT/UTF8, S_TEXT/ASS, S_TEXT/SSA, " +
			"S_TEXT/WEBVTT, D_WEBVTT/SUBTITLES, D_WEBVTT/CAPTIONS, D_WEBVTT/DESCRIPTIONS."},
		{
			"codec for a later release", segmentFile("matroska", tracks(subTrack(2, "S_HDMV/PGS"))), 2,
			"Track 2 has the codec S_HDMV/PGS. The reader does not extract this codec yet. A later release adds it.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := openFile(t, tt.file).Extract(tt.track, model.CommonReadOptions{})
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			assert.Equal(t, tt.want, pe.Msg)
		})
	}

	t.Run("a video track", func(t *testing.T) {
		t.Parallel()

		_, _, err := openTestdata(t, "text_tracks.mkv").Extract(1, model.CommonReadOptions{})
		var ie *model.InvalidArgumentError
		require.ErrorAs(t, err, &ie)
		assert.Equal(t, "The file has no subtitle track with the number 1.", ie.Msg)
	})
	t.Run("not Matroska", func(t *testing.T) {
		t.Parallel()

		b := []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n")
		_, err := mkv.Open(bytes.NewReader(b), int64(len(b)))
		var pe *model.ParseError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, "The file is not a Matroska or WebM file.", pe.Msg)
	})
	t.Run("EBML header of another DocType", func(t *testing.T) {
		t.Parallel()

		b := cat(ebmlHeader("other"), element(idSegment, nil))
		_, err := mkv.Open(bytes.NewReader(b), int64(len(b)))
		var pe *model.ParseError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, "The file is not a Matroska or WebM file.", pe.Msg)
	})
}

func TestWebVTTCodecs(t *testing.T) {
	t.Parallel()

	entries := make([][]byte, 0, 4)
	for i, kind := range []string{"SUBTITLES", "CAPTIONS", "DESCRIPTIONS", "METADATA"} {
		entries = append(entries, subTrack(uint64(i+1), "D_WEBVTT/"+kind))
	}
	f := openFile(t, segmentFile("webm", tracks(entries...),
		cluster(0, blockGroup(2, 1000, "\r\nline:90%\r\nThe gate closes in five minutes.", 2000))))
	formats := make([]model.Format, 0, len(f.Tracks()))
	for _, tr := range f.Tracks() {
		formats = append(formats, tr.Format)
	}
	assert.Equal(t, []model.Format{model.WebVTT, model.WebVTT, model.WebVTT, ""}, formats)
	s, _, err := f.Extract(2, model.CommonReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, "WEBVTT\n\n1\n00:00:01.000 --> 00:00:03.000 line:90%\nThe gate closes in five minutes.\n", write(t, s, model.WebVTT))
}

// countingReader counts the bytes that reads take from the made-up range [from, to).
type countingReader struct {
	prefix, suffix []byte
	from, to       int64
	filler         repeatReader
	inRange        atomic.Int64
}

func (c *countingReader) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	total := c.to + int64(len(c.suffix))
	if off >= total {
		return 0, errors.New("read past the end")
	}
	n := 0
	for n < len(p) && off+int64(n) < total {
		pos := off + int64(n)
		switch {
		case pos < c.from:
			n += copy(p[n:], c.prefix[pos:])
		case pos < c.to:
			take := int(min(int64(len(p)-n), c.to-pos))
			c.inRange.Add(int64(take))
			m, _ := c.filler.ReadAt(p[n:n+take], pos-c.from)
			n += m
		default:
			n += copy(p[n:], c.suffix[pos-c.to:])
		}
	}
	if n < len(p) {
		return n, errors.New("short read")
	}
	return n, nil
}

func TestExtractReadsNoVideoData(t *testing.T) {
	t.Parallel()

	const videoSize = 64 << 20
	videoHead := cat(idBytes(0xA3), sizeBytes(videoSize+4), blockHead(1, 0, 0x80))
	cl := unknownSizeElement(idCluster, cat(uintElement(0xE7, 0)))
	prefix := cat(ebmlHeader("matroska"), unknownSizeElement(idSegment, cat(info(1000000, "x"),
		tracks(trackEntry(trackFields{number: 1, kind: trackVideo, codecID: "V_UNCOMPRESSED"}), subTrack(2, "S_TEXT/UTF8")), cl, videoHead)))
	suffix := blockGroup(2, 100, "The bus leaves soon.", 800)
	r := &countingReader{prefix: prefix, suffix: suffix, from: int64(len(prefix)), to: int64(len(prefix)) + videoSize, filler: repeatReader{n: videoSize, v: 0x55}}
	size := r.to + int64(len(suffix))

	f, err := mkv.Open(r, size)
	require.NoError(t, err)
	s, _, err := f.Extract(2, model.CommonReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []cueRow{{100 * time.Millisecond, 900 * time.Millisecond, "The bus leaves soon."}}, cueRows(s))
	assert.LessOrEqual(t, r.inRange.Load(), int64(11))
}

func ExampleFile_Extract() {
	b, err := os.ReadFile("../../testdata/mkv/mkvmerge.mkv")
	if err != nil {
		fmt.Println(err)
		return
	}
	f, err := mkv.Open(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, t := range f.Tracks() {
		fmt.Println(t.Number, t.CodecID, t.Format, t.Language)
	}
	s, _, err := f.Extract(2, model.CommonReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(s.Len(), "cues,", s.Cues()[0].Text())
	// Output:
	// 2 S_TEXT/UTF8 srt en
	// 3 S_TEXT/ASS ass de
	// 6 cues, The ferry to the island leaves at nine.
}

func TestTrackInflateCap(t *testing.T) {
	t.Parallel()

	zeros := string(zlibFilled(t, 'a', 1<<20))
	blocks := make([][]byte, 4)
	for i := range blocks {
		blocks[i] = blockGroup(2, int16(i), zeros, 1000)
	}
	file := segmentFile("matroska", tracks(trackEntry(trackFields{
		number: 2, kind: trackSub, codecID: "S_TEXT/UTF8", encodings: compression(0),
	})), cluster(0, blocks...))

	f := openFile(t, file)
	f.SetInflateCap(4 << 20)
	_, _, err := f.Extract(2, model.CommonReadOptions{})
	require.NoError(t, err)

	f.SetInflateCap(3 << 20)
	_, _, err = f.Extract(2, model.CommonReadOptions{})
	var pe *model.ParseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, "The zlib data of track 2 inflates to more than 3 MB in total.", pe.Msg)
}
