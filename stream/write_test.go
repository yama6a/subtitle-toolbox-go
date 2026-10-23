package stream_test

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stream"
)

// Ported from tests/Streaming/StreamFixturesTest.php, optionSets.
var optionSets = []struct {
	name   string
	common model.CommonWriteOptions
}{
	{"default", model.CommonWriteOptions{}},
	{"CRLF", model.CommonWriteOptions{LineEnding: model.CRLF}},
	{"no BOM", model.CommonWriteOptions{BOM: model.BOMRemove}},
	{"BOM", model.CommonWriteOptions{BOM: model.BOMAdd}},
	{"strip tags", model.CommonWriteOptions{StripTags: true}},
	{"CRLF without BOM", model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}},
}

func TestSubRipWriterWritesTheBytesOfTheFullWriter(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".srt") {
		s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, srt.ReadOptions{})
		if err != nil {
			continue
		}
		for _, o := range optionSets {
			t.Run(rel+"/"+o.name, func(t *testing.T) {
				t.Parallel()

				var want, got bytes.Buffer
				require.NoError(t, srt.Write(&want, s, o.common, srt.WriteOptions{}))
				w, err := stream.NewSubRipWriter(&got, o.common, srt.WriteOptions{})
				require.NoError(t, err)
				for _, c := range s.All() {
					require.NoError(t, w.Write(c))
				}
				require.NoError(t, w.Close())
				assert.Equal(t, want.String(), got.String())
			})
		}
	}
}

func TestWebVTTWriterWritesTheBytesOfTheFullWriter(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".vtt") {
		parsed, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, webvtt.ReadOptions{})
		if err != nil {
			continue
		}
		header, _ := parsed.FormatData(string(model.WebVTT))
		s := &model.Subtitle{}
		require.NoError(t, s.SetFormatData(string(model.WebVTT), header))
		s.AddCues(cuesOf(parsed)...)
		for _, o := range optionSets {
			t.Run(rel+"/"+o.name, func(t *testing.T) {
				t.Parallel()

				var want, got bytes.Buffer
				report, err := webvtt.WriteWithReport(&want, s, o.common, webvtt.WriteOptions{})
				require.NoError(t, err)
				w, err := stream.NewWebVTTWriter(&got, o.common, webvtt.WriteOptions{}, header)
				require.NoError(t, err)
				for _, c := range s.All() {
					require.NoError(t, w.Write(c))
				}
				require.NoError(t, w.Close())
				assert.Equal(t, want.String(), got.String())
				assert.Equal(t, report.DroppedColors, w.DroppedColors())
			})
		}
	}
}

// Ported from tests/Streaming/StreamWriterTest.php, testShiftsASubRipFileIntoAWebVttFile.
func TestShiftsASubRipFileIntoAWebVTTFile(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := stream.NewWebVTTWriter(&out, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	rd := stream.NewSubRipReader(bytes.NewReader(testfiles.Read(t, "srt/real/own_styled.srt")), stream.ReadOptions{})
	for c, err := range rd.Cues() {
		require.NoError(t, err)
		c.Start += 2 * time.Second
		c.End += 2 * time.Second
		require.NoError(t, w.Write(c))
	}
	require.NoError(t, w.Close())

	cues, err := collect(stream.NewWebVTTReader(&out, stream.ReadOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 10)
	assert.Equal(t, 19*time.Second+985*time.Millisecond, cues[0].Start)
	assert.Equal(t, 22*time.Second+521*time.Millisecond, cues[0].End)
	assert.Equal(t, 782*time.Second, cues[9].End)
}

// Ported from tests/Streaming/StreamWriterTest.php, testCopiesTheWebVttHeaderFromReaderToWriter.
func TestCopiesTheWebVTTHeaderFromReaderToWriter(t *testing.T) {
	t.Parallel()

	rd := stream.NewWebVTTReader(bytes.NewReader(testfiles.Read(t, "vtt/real/w3c_regions.vtt")), stream.ReadOptions{})
	var out bytes.Buffer
	var w *stream.WebVTTWriter
	for c, err := range rd.Cues() {
		require.NoError(t, err)
		if w == nil {
			var err error
			w, err = stream.NewWebVTTWriter(&out, model.CommonWriteOptions{}, webvtt.WriteOptions{}, rd.Header())
			require.NoError(t, err)
		}
		require.NoError(t, w.Write(c))
	}
	require.NoError(t, w.Close())

	copied := stream.NewWebVTTReader(&out, stream.ReadOptions{})
	_, err := collect(copied.Cues())
	require.NoError(t, err)
	assert.Equal(t, rd.Header(), copied.Header())
	regions, _ := copied.Header().List("regions")
	assert.NotEmpty(t, regions)
}

func TestWebVTTWriterEscapesTheTimingArrowInCueText(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := stream.NewWebVTTWriter(&out, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	c := model.Cue{Start: time.Second, End: 2 * time.Second}
	c.SetText("a --> b\n\nc")
	require.NoError(t, w.Write(c))
	require.NoError(t, w.Close())
	assert.Equal(t, string(testfiles.Read(t, "vtt/real/own_arrow_in_text.vtt")), out.String())
}

func TestWebVTTWriterListsTheColorsThatItDrops(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "srt/real/own_font_colors.srt")
	common := model.CommonWriteOptions{BOM: model.BOMRemove}
	var out bytes.Buffer
	w, err := stream.NewWebVTTWriter(&out, common, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	for c, err := range stream.NewSubRipReader(bytes.NewReader(content), stream.ReadOptions{}).Cues() {
		require.NoError(t, err)
		require.NoError(t, w.Write(c))
	}

	s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	var want bytes.Buffer
	report, err := webvtt.WriteWithReport(&want, s, common, webvtt.WriteOptions{})
	require.NoError(t, err)
	assert.Equal(t, want.String(), out.String())
	assert.Equal(t, report.DroppedColors, w.DroppedColors())
	require.Len(t, w.DroppedColors(), 1)
	assert.Equal(t, "#123456", w.DroppedColors()[0].Color)
}

func TestWebVTTWriterListsNoDroppedColorWhenItStripsTags(t *testing.T) {
	t.Parallel()

	w, err := stream.NewWebVTTWriter(&bytes.Buffer{}, model.CommonWriteOptions{StripTags: true}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	var c model.Cue
	c.SetText(`<font color="#123456">Hi</font>`)
	require.NoError(t, w.Write(c))
	assert.Empty(t, w.DroppedColors())
}

func TestSubRipWriterWritesCRLFWithoutBOM(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := stream.NewSubRipWriter(&out, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Write(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"A"}}))
	require.NoError(t, w.Write(model.Cue{Start: 3 * time.Second, End: 4500 * time.Millisecond, Lines: []string{"B"}}))
	require.NoError(t, w.Close())
	assert.Equal(t, "1\r\n00:00:01,000 --> 00:00:02,000\r\nA\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,500\r\nB\r\n", out.String())
}

func TestWritersWriteTheBOMAtOnce(t *testing.T) {
	t.Parallel()

	var srtOut, vttOut bytes.Buffer
	_, err := stream.NewSubRipWriter(&srtOut, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	_, err = stream.NewWebVTTWriter(&vttOut, model.CommonWriteOptions{BOM: model.BOMAdd}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, bom, srtOut.String())
	assert.Equal(t, bom+"WEBVTT\n\n", vttOut.String())
}

func TestCloseLeavesTheCallersWriterOpen(t *testing.T) {
	t.Parallel()

	spy := &closeSpy{}
	w, err := stream.NewSubRipWriter(spy, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
	assert.False(t, spy.closed)
}

func TestWritesAfterCloseFail(t *testing.T) {
	t.Parallel()

	srtWriter, err := stream.NewSubRipWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	vttWriter, err := stream.NewWebVTTWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	for name, w := range map[string]interface {
		Write(c model.Cue) error
		Close() error
	}{"SubRip": srtWriter, "WebVTT": vttWriter} {
		require.NoError(t, w.Close(), name)
		err := w.Write(model.Cue{})
		require.ErrorIs(t, err, model.ErrInvalidArgument, name)
		assert.ErrorContains(t, err, "The writer is closed.", name)
	}
}

func TestWritersRejectInvalidOptions(t *testing.T) {
	t.Parallel()

	for _, common := range []model.CommonWriteOptions{{LineEnding: 9}, {BOM: 9}} {
		w, err := stream.NewSubRipWriter(&bytes.Buffer{}, common, srt.WriteOptions{})
		require.ErrorIs(t, err, model.ErrInvalidArgument)
		assert.Nil(t, w)

		v, err := stream.NewWebVTTWriter(&bytes.Buffer{}, common, webvtt.WriteOptions{}, nil)
		require.ErrorIs(t, err, model.ErrInvalidArgument)
		assert.Nil(t, v)
	}
}

func TestWritersReturnTheErrorOfTheWriterWrapped(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")
	_, err := stream.NewWebVTTWriter(&failingWriter{err: boom}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.ErrorIs(t, err, boom)
	require.ErrorContains(t, err, "write vtt")

	fw := &failingWriter{err: boom, after: 1}
	w, err := stream.NewSubRipWriter(fw, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.ErrorIs(t, w.Write(model.Cue{Lines: []string{"A"}}), boom)
	fw.err = nil
	require.NoError(t, w.Write(model.Cue{Lines: []string{"B"}}))
	assert.Contains(t, fw.String(), "B")
}

func TestDroppedColorsReturnsACopy(t *testing.T) {
	t.Parallel()

	w, err := stream.NewWebVTTWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	var c model.Cue
	c.SetText(`<font color="#123456">Hi</font>`)
	require.NoError(t, w.Write(c))
	got := w.DroppedColors()
	got[0].Color = "changed"
	assert.True(t, slices.ContainsFunc(w.DroppedColors(), func(d webvtt.DroppedColor) bool { return d.Color == "#123456" }))
}

type closeSpy struct {
	bytes.Buffer
	closed bool
}

func (c *closeSpy) Close() error {
	c.closed = true
	return nil
}

// failingWriter fails once err is set and after calls to Write have passed.
type failingWriter struct {
	bytes.Buffer
	err   error
	after int
	calls int
}

func (f *failingWriter) Write(b []byte) (int, error) {
	f.calls++
	if f.err != nil && f.calls > f.after {
		return 0, f.err
	}
	return f.Buffer.Write(b)
}
