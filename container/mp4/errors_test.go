package mp4_test

import (
	"bytes"
	"cmp"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestOpenErrors(t *testing.T) {
	t.Parallel()

	trex := fullBox("trex", 0, 0, u32(1), u32(1), u32(0), u32(0), u32(0))
	fragmented := cat(ftyp(), box("moov", fullBox("mvhd", 0, 0, u32(0), u32(0), u32(1000), u32(0), zeros(80)), box("mvex", trex)))
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{"fewer than 8 bytes", []byte("ftyp"), "The file is not an MP4 file."},
		{"bytes 4 to 7 are not ftyp", []byte("RIFF\x00\x00\x00\x00WAVEfmt "), "The file is not an MP4 file."},
		{"a top-level box that does not fit", cat(ftyp(), u32(4096), []byte("moov")), `The box "moov" at byte 28 does not fit into its parent box.`},
		{"a box type that is not printable", cat(ftyp(), u32(4096), []byte{0, 1, 2, 3}), "The box 0x00010203 at byte 28 does not fit into its parent box."},
		{"a box size below its header", cat(ftyp(), u32(7), []byte("free")), `The box "free" at byte 28 does not fit into its parent box.`},
		{"a 64-bit size without its 8 bytes", cat(ftyp(), u32(1), []byte("free")), `The box "free" at byte 28 does not fit into its parent box.`},
		{"no moov box", cat(ftyp(), box("mdat", []byte("abc"))), "The MP4 file has no moov box."},
		{"a moof box before the moov box", cat(ftyp(), box("moof")), fragmentedMessage},
		{"an mvex box in the moov box", fragmented, fragmentedMessage},
		{"a track with a timescale of 0", single(trackSpec{zeroTime: true}, textSample("Hi")), "Track 1 has a timescale of 0."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := mp4.Open(bytes.NewReader(tt.content), int64(len(tt.content)))
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			require.ErrorIs(t, err, model.ErrParse)
			assert.Equal(t, tt.want, pe.Msg)
			assert.Empty(t, pe.Format)
			assert.Zero(t, pe.Line)
		})
	}
}

const fragmentedMessage = "The MP4 file is fragmented. The reader reads only samples in the moov box."

func TestOpenReportsMp4ffErrors(t *testing.T) {
	t.Parallel()

	stts := fullBox("stts", 0, 0, u32(5))
	tests := []struct {
		name    string
		content []byte
	}{
		{"an stts box with fewer entries than its count", single(trackSpec{boxes: map[string][]byte{"stts": stts}}, textSample("Hi"))},
		{"the same damage in a video track", file([]trackSpec{
			func() trackSpec { v := videoTrack(1, 1, 2, 4); v.boxes = map[string][]byte{"stts": stts}; return v }(),
		}, true)},
		{"huge_sample_count.mp4", testfiles.Read(t, "mp4/huge_sample_count.mp4")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := mp4.Open(bytes.NewReader(tt.content), int64(len(tt.content)))
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			assert.Contains(t, pe.Msg, "The MP4 file is not valid: ")
			assert.Error(t, pe.Err)
		})
	}
}

func TestExtractErrors(t *testing.T) {
	t.Parallel()

	text := textSample("The shop is closed.")
	c708 := box("c708", zeros(6), u16(1))
	outside := func(size uint32) []byte {
		return cat(ftyp(), moovBox([]trackSpec{{
			id: 1, handler: "sbtl", entry: tx3gEntry(),
			chunks: []chunkSpec{{1 << 30, [][2]uint32{{1000, size}}}},
		}}))
	}
	broken := func(name string, replacement []byte) []byte {
		return single(trackSpec{boxes: map[string][]byte{name: replacement}}, text)
	}
	longer, notUTF8, oddUTF16 := cat(u16(50), []byte("The shop")), textSample("Caf\xE9"), textSample("\xFE\xFF\x00")
	badPacket := box("ccdp", []byte("not a packet"))
	tests := []struct {
		name    string
		content []byte
		number  int    // 1 when 0
		sample  []byte // the sample whose file offset the message holds
		want    string // with %d for the offset
	}{
		{
			name: "an unread codec", content: single(trackSpec{entry: box("wvtt", zeros(6), u16(1))}, text),
			want: "Track 1 has the codec wvtt. The reader extracts only tx3g, c608, c708.",
		},
		{
			name: "an encrypted codec", content: testfiles.Read(t, "mp4/text_tracks.mp4"), number: 5,
			want: "Track 5 has the codec enct and is encrypted.",
		},
		{
			name: "a tx3g entry with a sinf box", content: single(trackSpec{entry: tx3gEntry(box("sinf", box("frma", []byte("tx3g"))))}, text),
			want: "Track 1 has the codec tx3g and is encrypted.",
		},
		{name: "a sample outside the file", content: outside(20), want: "A sample of track 1 at byte 1073741824 lies outside the file."},
		{name: "a 1-byte sample outside the file", content: outside(1), want: "A sample of track 1 at byte 1073741824 lies outside the file."},
		{
			name: "an stts box with fewer samples than the size box", content: broken("stts", fullBox("stts", 0, 0, u32(0))),
			want: "The stts box of track 1 has fewer samples than the sample size box.",
		},
		{name: "no stts box", content: broken("stts", []byte{}), want: "Track 1 has no stts box."},
		{name: "no stsc box", content: broken("stsc", []byte{}), want: "Track 1 has no stsc box."},
		{name: "no stco and no co64 box", content: broken("stco", []byte{}), want: "Track 1 has no stco box."},
		{name: "no stsz and no stz2 box", content: broken("stsz", []byte{}), want: "Track 1 has no stsz or stz2 box."},
		{
			name: "an stsz sample size that does not fit the file", content: broken("stsz", fullBox("stsz", 0, 0, u32(1000), u32(5000))),
			want: "The stsz box of track 1 holds 5000 samples of 1000 bytes, which do not fit into the file.",
		},
		{
			name: "a stz2 box with a field size of 3 bits", content: broken("stsz", fullBox("stz2", 0, 0, []byte{0, 0, 0, 3}, u32(1), []byte{0})),
			want: "The stz2 box of track 1 is not valid.",
		},
		{
			name: "a stz2 box with fewer sizes than its count", content: broken("stsz", fullBox("stz2", 0, 0, []byte{0, 0, 0, 0x10}, u32(2), []byte{0, 5})),
			want: "The stz2 box of track 1 is not valid.",
		},
		{
			name: "a stz2 box without a count", content: broken("stsz", fullBox("stz2", 0, 0, []byte{0, 0, 0, 0x10})),
			want: "The stz2 box of track 1 is not valid.",
		},
		{
			name: "a text longer than its sample", content: single(trackSpec{}, longer), sample: longer,
			want: "The text of the sample of track 1 at byte %d is longer than the sample.",
		},
		{
			name: "a text that is not UTF-8", content: single(trackSpec{}, notUTF8), sample: notUTF8,
			want: "The text of the sample of track 1 at byte %d is not valid UTF-8.",
		},
		{
			name: "UTF-16 text with an odd number of bytes", content: single(trackSpec{}, oddUTF16), sample: oddUTF16,
			want: "The text of the sample of track 1 at byte %d is not valid UTF-8.",
		},
		{
			name: "a sample that ends at 100000 hours", content: single(trackSpec{timescale: 1, samples: []sampleSpec{{360000000, text}}}, nil), sample: text,
			want: "The sample of track 1 at byte %d ends at 360000000 s. Times must be below 100000 hours.",
		},
		{
			name: "a ccdp box that is no packet", content: single(trackSpec{entry: c708, handler: "clcp"}, badPacket), sample: badPacket,
			want: "The sample of track 1 at byte %d holds a ccdp box that is not valid. The data holds no caption distribution packet.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			number := cmp.Or(tt.number, 1)
			want := tt.want
			if tt.sample != nil {
				want = strings.Replace(want, "%d", strconv.Itoa(bytes.Index(tt.content, tt.sample)), 1)
			}
			_, _, err := openBytes(t, tt.content).Extract(number, model.CommonReadOptions{}, mp4.ExtractOptions{})
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			assert.Equal(t, want, pe.Msg)
		})
	}
}

func TestExtractRejectsABadTrackNumberAndBadOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		file   string
		number int
		opts   mp4.ExtractOptions
		option string
		msg    string
	}{
		{"a video track", "text_tracks.mp4", 1, mp4.ExtractOptions{}, "ReadOptions.Track", "The file has no subtitle track with the number 1."},
		{
			"a bad SCC channel on a c608 track", "ffmpeg_c608.mov", 1,
			mp4.ExtractOptions{SCC: scc.ReadOptions{Channel: 3}},
			"ReadOptions.SCC.Channel", "The SCC data channel must be 1 or 2, got 3.",
		},
		{
			"a bad MCC service on a c708 track", "c708_track.mov", 1,
			mp4.ExtractOptions{MCC: mcc.ReadOptions{Service: 64}},
			"ReadOptions.MCC.Service", "The CEA-708 service must be from 1 to 63, got 64.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := openFixture(t, tt.file).Extract(tt.number, model.CommonReadOptions{}, tt.opts)
			var ie *model.InvalidArgumentError
			require.ErrorAs(t, err, &ie)
			assert.Equal(t, tt.option, ie.Option)
			assert.Equal(t, tt.msg, ie.Msg)
		})
	}
}

type failingReader struct {
	r     io.ReaderAt
	okFor int // the number of reads before the failure
	reads int
	err   error
}

func (f *failingReader) ReadAt(p []byte, off int64) (int, error) {
	if f.reads >= f.okFor {
		return 0, f.err
	}
	f.reads++
	return f.r.ReadAt(p, off)
}

func TestReadErrorsKeepTheirCause(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "mp4/one_track.mp4")
	cause := errors.New("disk gone")
	for _, okFor := range []int{0, 4} {
		t.Run(strconv.Itoa(okFor)+" reads before the failure", func(t *testing.T) {
			t.Parallel()

			r := &failingReader{r: bytes.NewReader(content), okFor: okFor, err: cause}
			f, err := mp4.Open(r, int64(len(content)))
			if err == nil {
				_, _, err = f.Extract(2, model.CommonReadOptions{}, mp4.ExtractOptions{})
			}
			require.ErrorIs(t, err, cause)
			assert.ErrorContains(t, err, "read mp4: ")
		})
	}
}
