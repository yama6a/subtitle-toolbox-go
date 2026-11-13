package hls_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/hls"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ownSegments(tb testing.TB) [][]byte {
	tb.Helper()

	out := make([][]byte, 0, 4)
	for i := range 4 {
		out = append(out, readFixtureText(tb, fmt.Sprintf("own-fileSequence%d.webvtt", i)))
	}
	return out
}

func TestJoin(t *testing.T) {
	t.Parallel()

	const (
		mapped    = "WEBVTT\nX-TIMESTAMP-MAP=LOCAL:01:00:00.000,MPEGTS:324000000\n\n01:02:20.000 --> 01:02:22.000\nOne\n"
		boundary1 = "WEBVTT\n\n00:00:04.000 --> 00:00:06.000\nAll aboard.\n"
		boundary2 = "WEBVTT\n\n00:00:06.000 --> 00:00:07.000\nAll aboard.\n"
	)
	start := func(n int64) *int64 { return &n }
	tests := []struct {
		name     string
		segments [][]byte
		opts     hls.JoinOptions
		want     []cue
	}{
		{
			name:     "cues that repeat across segments appear once",
			segments: ownSegments(t),
			want: []cue{
				{ms(1000), ms(3000), "The ferry leaves at noon."},
				{ms(5000), ms(7500), "Please keep your tickets ready."},
				{ms(9000), ms(11000), "Mind the gap between\nthe ramp and the deck."},
				{ms(18500), ms(20000), "Thank you for travelling with us."},
			},
		},
		{
			name:     "an explicit stream start moves the cues",
			segments: ownSegments(t)[:1],
			opts:     hls.JoinOptions{StreamStart: start(181083 + 90000)},
			want: []cue{
				{0, ms(2000), "The ferry leaves at noon."},
				{ms(4000), ms(6500), "Please keep your tickets ready."},
			},
		},
		{
			name:     "times count from the MPEGTS of the first segment",
			segments: [][]byte{[]byte(mapped)},
			want:     []cue{{ms(140000), ms(142000), "One"}},
		},
		{
			name:     "a stream start of 0 adds the whole MPEGTS",
			segments: [][]byte{[]byte(mapped)},
			opts:     hls.JoinOptions{StreamStart: start(0)},
			want:     []cue{{ms(3740000), ms(3742000), "One"}},
		},
		{
			name: "a segment without a map has MPEGTS 0",
			segments: [][]byte{
				[]byte("WEBVTT\nX-TIMESTAMP-MAP=MPEGTS:90000,LOCAL:00:00:00.000\n\n00:00:01.000 --> 00:00:02.000\nOne\n"),
				[]byte("WEBVTT\n\n00:00:03.000 --> 00:00:04.000\nTwo\n"),
			},
			want: []cue{{ms(1000), ms(2000), "One"}, {ms(2000), ms(3000), "Two"}},
		},
		{
			name: "a cue before the stream start starts at 0",
			segments: [][]byte{
				[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:90000\n\n00:00:01.000 --> 00:00:03.000\nOne\n"),
				[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:0\n\n00:00:01.000 --> 00:00:03.000\nTwo\n"),
			},
			want: []cue{{0, ms(2000), "Two"}, {ms(1000), ms(3000), "One"}},
		},
		{
			name: "a wrapped MPEGTS value",
			segments: [][]byte{
				[]byte(fmt.Sprintf("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:%d\n\n00:00:00.000 --> 00:00:01.000\nOne\n", hls.MPEGTSWrap-90000)),
				[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:0\n\n00:00:00.000 --> 00:00:01.000\nTwo\n"),
			},
			want: []cue{{0, ms(1000), "One"}, {ms(1000), ms(2000), "Two"}},
		},
		{
			name:     "a cue that was split at a boundary is whole again",
			segments: [][]byte{[]byte(boundary1), []byte(boundary2)},
			want:     []cue{{ms(4000), ms(7000), "All aboard."}},
		},
		{
			name: "overlapping cues with the same text join",
			segments: [][]byte{
				[]byte("WEBVTT\n\n00:00:04.000 --> 00:00:06.000\nAll aboard.\n\n00:00:05.500 --> 00:00:06.500\nAll aboard.\n"),
			},
			want: []cue{{ms(4000), ms(6500), "All aboard."}},
		},
		{
			name:     "empty segments give no cues",
			segments: [][]byte{readFixtureText(t, "shaka-vtt-071.vtt"), readFixtureText(t, "shaka-vtt-072.vtt")},
			want:     []cue{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := hls.Join(tt.segments, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, tt.want, summarize(got))
		})
	}
}

func TestJoinKeepsTheFileDataWithoutTheMap(t *testing.T) {
	t.Parallel()

	segments := [][]byte{
		[]byte("WEBVTT Ferry\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:0\nKind: captions\n\nSTYLE\n::cue { color: yellow }\n\n00:00:01.000 --> 00:00:02.000\nOne\n"),
		[]byte("WEBVTT Other\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:90000\n\n00:00:01.000 --> 00:00:02.000\nTwo\n"),
	}
	got, err := hls.Join(segments, hls.JoinOptions{})
	require.NoError(t, err)
	data, ok := got.FormatData("vtt")
	require.True(t, ok)
	header, _ := data.String("header")
	lines, _ := data.Strings("headerLines")
	styles, _ := data.Strings("styles")
	assert.Equal(t, "Ferry", header)
	assert.Equal(t, []string{"Kind: captions"}, lines)
	assert.Equal(t, []string{"::cue { color: yellow }"}, styles)

	empty, err := hls.Join([][]byte{readFixtureText(t, "shaka-vtt-071.vtt")}, hls.JoinOptions{})
	require.NoError(t, err)
	assert.Empty(t, empty.FormatDataKeys())
}

func TestJoinErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		segments [][]byte
		opts     hls.JoinOptions
		target   error
		msg      string
	}{
		{"a broken cue", [][]byte{[]byte("WEBVTT\n\n00:00:01.000 --> nonsense\nOne\n")}, hls.JoinOptions{}, model.ErrParse, ""},
		{"a broken map", [][]byte{[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000\n\n")}, hls.JoinOptions{}, model.ErrParse, ""},
		{"an MPEGTS of 2^33", [][]byte{[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:8589934592\n\n")}, hls.JoinOptions{}, model.ErrInvalidArgument, ""},
		{
			"a stream start of 2^33",
			[][]byte{[]byte("WEBVTT\n\n")},
			hls.JoinOptions{StreamStart: new(int64(hls.MPEGTSWrap))},
			model.ErrInvalidArgument,
			"The stream start must be a 33-bit timestamp from 0 to 8589934591, got 8589934592.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := hls.Join(tt.segments, tt.opts)
			require.ErrorIs(t, err, tt.target)
			if tt.msg != "" {
				var ierr *model.InvalidArgumentError
				require.ErrorAs(t, err, &ierr)
				assert.Equal(t, tt.msg, ierr.Msg)
			}
		})
	}
}

func TestSegmentThenJoinGivesTheOriginalCues(t *testing.T) {
	t.Parallel()

	original := readFixture(t)
	tests := []struct {
		name string
		opts hls.SegmentOptions
	}{
		{"defaults", hls.SegmentOptions{}},
		{"4 s segments and LOCAL 1 h", hls.SegmentOptions{SegmentDuration: 4 * time.Second, Map: &hls.TimestampMap{MPEGTS: 181083, Local: time.Hour}}},
		{"2.002 s segments and a MPEGTS near the wrap", hls.SegmentOptions{SegmentDuration: ms(2002), Map: &hls.TimestampMap{MPEGTS: hls.MPEGTSWrap - 90000}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, err := hls.Segment(original, tt.opts)
			require.NoError(t, err)
			var contents [][]byte
			require.NoError(t, r.EachSegment(func(_ string, content []byte) error {
				contents = append(contents, append([]byte(nil), content...))
				return nil
			}))
			joined, err := hls.Join(contents, hls.JoinOptions{})
			require.NoError(t, err)
			assert.Equal(t, summarize(original), summarize(joined))
			assert.Equal(t, written(t, original), written(t, joined))
		})
	}
}

func written(tb testing.TB, s *model.Subtitle) string {
	tb.Helper()

	var b strings.Builder
	_, err := webvtt.Write(&b, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
	require.NoError(tb, err)
	return b.String()
}
