package conformance_test

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"io"
	"iter"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Ported from tests/Streaming/StreamMemoryTest.php. It does not run in parallel, because it measures the
// heap of the whole process. The race detector makes it 20 times slower, so the bench job of CI runs it
// with STBOX_GROWTH=1 and without -race.
func TestStreamsManyCuesGrowth(t *testing.T) { //nolint:paralleltest // see above
	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	tests := []struct {
		name      string
		newWriter func(io.Writer) (cueWriter, error)
		newReader func(io.Reader) iter.Seq2[model.Cue, error]
	}{
		{
			name: "SubRip",
			newWriter: func(w io.Writer) (cueWriter, error) {
				return srt.NewStreamWriter(w, model.CommonWriteOptions{}, srt.WriteOptions{})
			},
			newReader: func(r io.Reader) iter.Seq2[model.Cue, error] {
				return srt.NewStreamReader(r, srt.StreamOptions{}).Cues()
			},
		},
		{
			name: "WebVTT",
			newWriter: func(w io.Writer) (cueWriter, error) {
				return webvtt.NewStreamWriter(w, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
			},
			newReader: func(r io.Reader) iter.Seq2[model.Cue, error] {
				return webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues()
			},
		},
	}
	for _, tt := range tests { //nolint:paralleltest // see above
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Create(filepath.Join(t.TempDir(), "many."+tt.name))
			require.NoError(t, err)
			defer f.Close()

			var writeErr error
			growth := heapGrowth(func(sample func()) {
				bw := bufio.NewWriter(f)
				w, err := tt.newWriter(bw)
				if err != nil {
					writeErr = err
					return
				}
				for i := range memoryCues {
					if writeErr = w.Write(memoryCue(i)); writeErr != nil {
						return
					}
					if i%sampleEvery == 0 {
						sample()
					}
				}
				if writeErr = w.Close(); writeErr == nil {
					writeErr = bw.Flush()
				}
			})
			require.NoError(t, writeErr)
			assert.Less(t, growth, uint64(memoryLimit), "write")
			info, err := f.Stat()
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(minFileSize))

			_, err = f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			var count int
			var last model.Cue
			var readErr error
			growth = heapGrowth(func(sample func()) {
				for c, err := range tt.newReader(f) {
					if err != nil {
						readErr = err
						return
					}
					if count%sampleEvery == 0 {
						sample()
					}
					count++
					last = c
				}
			})
			require.NoError(t, readErr)
			assert.Less(t, growth, uint64(memoryLimit), "read")
			assert.Equal(t, memoryCues, count)
			wantLast := memoryCue(memoryCues - 1)
			assert.Equal(t, wantLast.Start, last.Start)
			assert.Equal(t, wantLast.Text(), last.Text())
		})
	}
}

func TestReadersGiveNoCuesForEmptyContent(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", bom + "\n \n", "\x1a", "\x01\x02"} {
		cues, err := collect(srt.NewStreamReader(strings.NewReader(content), srt.StreamOptions{}).Cues())
		require.NoError(t, err)
		assert.Empty(t, cues)

		cues, err = collect(webvtt.NewStreamReader(strings.NewReader(content), webvtt.StreamOptions{Lenient: true}).Cues())
		require.NoError(t, err)
		assert.Empty(t, cues)
	}
}

func TestReadersYieldTheCuesBeforeABrokenBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		read    func(io.Reader) ([]model.Cue, error)
		line    int
	}{
		{
			name:    "SubRip",
			content: "1\n00:00:01,000 --> 00:00:02,000\nGood\n\n2\nbroken\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(srt.NewStreamReader(r, srt.StreamOptions{}).Cues())
			},
			line: 6,
		},
		{
			name:    "SubRip bad time",
			content: "1\n00:00:01,000 --> 00:00:02,000\nGood\n\n2\n00:00:0x,000 --> 00:00:04,000\nIt is late.\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(srt.NewStreamReader(r, srt.StreamOptions{}).Cues())
			},
			line: 6,
		},
		{
			name:    "WebVTT",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\nGood\n\nbroken block\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues())
			},
			line: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cues, err := tt.read(strings.NewReader(tt.content))
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.line, perr.Line)
			require.Len(t, cues, 1)
			assert.Equal(t, "Good", cues[0].Text())
		})
	}
}

func TestLenientReadersRemoveControlCharactersWithOneWarning(t *testing.T) {
	t.Parallel()

	srt := srt.NewStreamReader(strings.NewReader("1\n00:00:01,000 --> 00:00:02,000\nThe\f fer\x00ry\x7f.\r\n\x1a"), srt.StreamOptions{Lenient: true})
	cues, err := collect(srt.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 1)
	assert.Equal(t, "The ferry.", cues[0].Text())
	require.Len(t, srt.Warnings(), 1)
	assert.Equal(t, 3, srt.Warnings()[0].Line)

	vtt := webvtt.NewStreamReader(strings.NewReader("WEBVTT\n\n00:01.000 --> 00:02.000\nThe\f fer\x00ry\x7f.\r\n\x1a"), webvtt.StreamOptions{Lenient: true})
	cues, err = collect(vtt.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 1)
	assert.Equal(t, "The fer\xef\xbf\xbdry.", cues[0].Text())
	require.Len(t, vtt.Warnings(), 1)
	assert.Equal(t, 4, vtt.Warnings()[0].Line)
}

func TestInvalidUTF8StaysAndLenientModeWarnsFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		vtt  bool
	}{
		{"SubRip with one bad byte", "encoding/invalid-utf-8/one-bad-byte.srt", false},
		{"WebVTT with one bad byte", "encoding/invalid-utf-8/one-bad-byte.vtt", true},
		{"SubRip in Windows-1252", "encoding/french-windows-1252.srt", false},
		{"SubRip in Shift JIS with CRLF", "encoding/japanese-shift_jis.srt", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := testfiles.Read(t, tt.file)
			var warnings []model.ParseWarning
			var err error
			var cues []model.Cue
			if tt.vtt {
				rd := webvtt.NewStreamReader(bytes.NewReader(content), webvtt.StreamOptions{Lenient: true})
				cues, err = collect(rd.Cues())
				warnings = rd.Warnings()
			} else {
				rd := srt.NewStreamReader(bytes.NewReader(content), srt.StreamOptions{Lenient: true})
				cues, err = collect(rd.Cues())
				warnings = rd.Warnings()
			}
			require.NoError(t, err)
			assert.NotEmpty(t, cues)
			require.NotEmpty(t, warnings)
			assert.True(t, strings.HasPrefix(warnings[0].Message, "The content is not valid UTF-8."), warnings[0].Message)
			assert.Equal(t, model.NoLine, warnings[0].Line)
		})
	}
}

// Ported from tests/Parsers/LooseTimestampsTest.php and StartAfterEndTest.php, stream rows.
func TestLenientReadersRepairTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		vtt     bool
	}{
		{"SubRip with a colon before the fraction", "1\n00:00:01:105 --> 00:00:02:200\nThe ferry leaves.\n\n2\n02:00:00,000 --> 02:00:02,000\nIt is late.\n", false},
		{"SubRip with 4 fraction digits", "1\n00:28:41,1000 --> 00:28:42,0006\nThe ferry leaves.\n\n2\n02:00:00,000 --> 02:00:02,000\nIt is late.\n", false},
		{"SubRip swapped", "1\n00:00:08,000 --> 00:00:05,000\nThe ferry leaves.\n\n2\n00:00:10,000 --> 00:00:12,000\nIt is late.\n", false},
		{"SubRip over 30 s", "1\n00:01:00,000 --> 00:00:05,000\nThe ferry leaves.\n\n2\n00:00:10,000 --> 00:00:12,000\nIt is late.\n", false},
		{"WebVTT with a comma", "WEBVTT\n\n00:01,105 --> 00:02,200\nThe ferry leaves.\n\n02:00:00.000 --> 02:00:02.000\nIt is late.\n", true},
		{"WebVTT swapped", "WEBVTT\n\n00:00:08.000 --> 00:00:05.000\nThe ferry leaves.\n\n00:00:10.000 --> 00:00:12.000\nIt is late.\n", true},
		{"SubRip that rounds to 100000 hours", "1\n0:0:0,0 --> 99999:59:59,9995\nThe ferry leaves.\n\n2\n02:00:00,000 --> 02:00:02,000\nIt is late.\n", false},
		{"WebVTT that rounds to 100000 hours", "WEBVTT\n\n00:01.000 --> 99999:59:59,9997\nThe ferry leaves.\n\n02:00:00.000 --> 02:00:02.000\nIt is late.\n", true},
	}
	for _, tt := range tests {
		for _, w := range wraps {
			t.Run(tt.name+"/"+w.name, func(t *testing.T) {
				t.Parallel()

				if tt.vtt {
					compareWebVTT(t, []byte(tt.content), w.wrap, true)
					return
				}
				compareSubRip(t, []byte(tt.content), w.wrap, true)
			})
		}
	}
}

func TestAReadErrorDropsTheBlockThatItCut(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk gone")
	tests := []struct {
		name  string
		input string
		read  func(io.Reader) ([]model.Cue, error)
		want  []string
	}{
		{
			name:  "SubRip cut time",
			input: "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,0",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(srt.NewStreamReader(r, srt.StreamOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "SubRip cut text",
			input: "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo li",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(srt.NewStreamReader(r, srt.StreamOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT cut text",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo li",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT before a line of spaces",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo\n \n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT before spaces and an empty line",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo\n \n\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues())
			},
			want: []string{"One", "Two"},
		},
		{
			name:  "WebVTT after an empty line",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(webvtt.NewStreamReader(r, webvtt.StreamOptions{}).Cues())
			},
			want: []string{"One"},
		},
	}
	for _, tt := range tests {
		for _, w := range wraps {
			t.Run(tt.name+"/"+w.name, func(t *testing.T) {
				t.Parallel()

				cues, err := tt.read(w.wrap(io.MultiReader(strings.NewReader(tt.input), errReader{boom})))
				require.ErrorIs(t, err, boom)
				texts := make([]string, 0, len(cues))
				for _, c := range cues {
					texts = append(texts, c.Text())
				}
				assert.Equal(t, tt.want, texts)
			})
		}
	}
}

// Ported from tests/Streaming/StreamWriterTest.php, testShiftsASubRipFileIntoAWebVttFile.
func TestShiftsASubRipFileIntoAWebVTTFile(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := webvtt.NewStreamWriter(&out, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	rd := srt.NewStreamReader(bytes.NewReader(testfiles.Read(t, "srt/real/own_styled.srt")), srt.StreamOptions{})
	for c, err := range rd.Cues() {
		require.NoError(t, err)
		c.Start += 2 * time.Second
		c.End += 2 * time.Second
		require.NoError(t, w.Write(c))
	}
	require.NoError(t, w.Close())

	cues, err := collect(webvtt.NewStreamReader(&out, webvtt.StreamOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 10)
	assert.Equal(t, 19*time.Second+985*time.Millisecond, cues[0].Start)
	assert.Equal(t, 22*time.Second+521*time.Millisecond, cues[0].End)
	assert.Equal(t, 782*time.Second, cues[9].End)
}

func TestWritersWriteTheBOMAtOnce(t *testing.T) {
	t.Parallel()

	var srtOut, vttOut bytes.Buffer
	_, err := srt.NewStreamWriter(&srtOut, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	_, err = webvtt.NewStreamWriter(&vttOut, model.CommonWriteOptions{BOM: model.BOMAdd}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, bom, srtOut.String())
	assert.Equal(t, bom+"WEBVTT\n\n", vttOut.String())
}

func TestWritesAfterCloseFail(t *testing.T) {
	t.Parallel()

	srtWriter, err := srt.NewStreamWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	vttWriter, err := webvtt.NewStreamWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
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
		w, err := srt.NewStreamWriter(&bytes.Buffer{}, common, srt.WriteOptions{})
		require.ErrorIs(t, err, model.ErrInvalidArgument)
		assert.Nil(t, w)

		v, err := webvtt.NewStreamWriter(&bytes.Buffer{}, common, webvtt.WriteOptions{}, nil)
		require.ErrorIs(t, err, model.ErrInvalidArgument)
		assert.Nil(t, v)
	}
}

func TestWritersReturnTheErrorOfTheWriterWrapped(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk full")
	_, err := webvtt.NewStreamWriter(&failingWriter{err: boom}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.ErrorIs(t, err, boom)
	require.ErrorContains(t, err, "write vtt")

	fw := &failingWriter{err: boom, after: 1}
	w, err := srt.NewStreamWriter(fw, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.ErrorIs(t, w.Write(model.Cue{Lines: []string{"A"}}), boom)
	fw.err = nil
	require.NoError(t, w.Write(model.Cue{Lines: []string{"B"}}))
	assert.Contains(t, fw.String(), "B")
}

func TestStreamWritersGateImageCues(t *testing.T) {
	t.Parallel()

	image := model.Cue{End: time.Second, Image: &model.Image{PNG: []byte("x"), Width: 1, Height: 1, ScreenWidth: 2, ScreenHeight: 2}}
	text := model.Cue{Start: 2 * time.Second, End: 3 * time.Second, Lines: []string{"Hello"}}
	tests := []struct {
		name   string
		format model.Format
		open   func(io.Writer, model.CommonWriteOptions) (cueWriter, error)
	}{
		{"SubRip", model.SubRip, func(w io.Writer, o model.CommonWriteOptions) (cueWriter, error) {
			return srt.NewStreamWriter(w, o, srt.WriteOptions{})
		}},
		{"WebVTT", model.WebVTT, func(w io.Writer, o model.CommonWriteOptions) (cueWriter, error) {
			return webvtt.NewStreamWriter(w, o, webvtt.WriteOptions{}, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			w, err := tt.open(&out, model.CommonWriteOptions{})
			require.NoError(t, err)
			require.NoError(t, w.Write(text))
			written := out.Len()
			var uerr *model.UnwritableError
			require.ErrorAs(t, w.Write(image), &uerr)
			assert.Equal(t, tt.format, uerr.Format)
			assert.Equal(t, 1, uerr.CueIndex)
			assert.Equal(t, written, out.Len())

			var skipped, plain bytes.Buffer
			w, err = tt.open(&skipped, model.CommonWriteOptions{SkipImageCues: true})
			require.NoError(t, err)
			require.NoError(t, w.Write(image))
			require.NoError(t, w.Write(text))
			w, err = tt.open(&plain, model.CommonWriteOptions{})
			require.NoError(t, err)
			require.NoError(t, w.Write(text))
			assert.Equal(t, plain.String(), skipped.String())
		})
	}
}

type cueWriter interface {
	Write(c model.Cue) error
	Close() error
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

const bom = "\xEF\xBB\xBF"

func collect(seq iter.Seq2[model.Cue, error]) ([]model.Cue, error) {
	var cues []model.Cue
	for c, err := range seq {
		if err != nil {
			return cues, err
		}
		cues = append(cues, c)
	}
	return cues, nil
}

// wraps are the readers that cut the input at other places than the file does.
var wraps = []struct {
	name string
	wrap func(io.Reader) io.Reader
}{
	{"whole", func(r io.Reader) io.Reader { return r }},
	{"one byte", iotest.OneByteReader},
	{"half", iotest.HalfReader},
	{"data with error", iotest.DataErrReader},
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// compareSubRip checks that the stream reader gives the cues, warnings and first error of srt.Read.
func compareSubRip(tb testing.TB, content []byte, wrap func(io.Reader) io.Reader, lenient bool) {
	tb.Helper()

	s, warnings, fullErr := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, srt.ReadOptions{})
	rd := srt.NewStreamReader(wrap(bytes.NewReader(content)), srt.StreamOptions{Lenient: lenient})
	cues, err := collect(rd.Cues())
	if fullErr != nil {
		compareErrors(tb, fullErr, err)
		return
	}
	require.NoError(tb, err)
	assert.Equal(tb, cuesOf(s), inStartOrder(cues))
	assert.Equal(tb, warnings, rd.Warnings())
}

// compareWebVTT checks that the stream reader gives the cues, header, warnings and first error of webvtt.Read.
func compareWebVTT(tb testing.TB, content []byte, wrap func(io.Reader) io.Reader, lenient bool) {
	tb.Helper()

	s, warnings, fullErr := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
	rd := webvtt.NewStreamReader(wrap(bytes.NewReader(content)), webvtt.StreamOptions{Lenient: lenient})
	cues, err := collect(rd.Cues())
	if fullErr != nil {
		compareErrors(tb, fullErr, err)
		return
	}
	require.NoError(tb, err)
	assert.Equal(tb, cuesOf(s), inStartOrder(cues))
	assert.Equal(tb, sortedByLine(warnings), rd.Warnings())
	header, ok := s.FormatData(string(model.WebVTT))
	if !ok {
		header = model.NewData()
	}
	assert.Equal(tb, header, rd.Header())
}

// inStartOrder puts cues through a subtitle, which sorts them by start as the full readers do.
func inStartOrder(cues []model.Cue) []model.Cue {
	s := &model.Subtitle{}
	if err := s.AddCues(cues...); err != nil {
		panic(err)
	}
	return cuesOf(s)
}

func cuesOf(s *model.Subtitle) []model.Cue {
	var cues []model.Cue
	for _, c := range s.All() {
		cues = append(cues, c)
	}
	return cues
}

func sortedByLine(w []model.ParseWarning) []model.ParseWarning {
	w = slices.Clone(w)
	slices.SortStableFunc(w, func(a, b model.ParseWarning) int { return cmp.Compare(a.Line, b.Line) })
	return w
}

// compareErrors checks the error of the stream reader. Strict SubRip reads a NUL byte as an error before it reads
// any block, so a block error on a line before the NUL line may come first from the stream reader. Every other
// comparison needs the same error.
func compareErrors(tb testing.TB, want, got error) {
	tb.Helper()

	require.Error(tb, got)
	var wantErr, gotErr *model.ParseError
	if errors.As(want, &wantErr) && strings.Contains(wantErr.Msg, "has a NUL character") &&
		errors.As(got, &gotErr) && gotErr.Msg != wantErr.Msg {
		assert.Less(tb, gotErr.Line, wantErr.Line)
		return
	}
	assert.Equal(tb, want.Error(), got.Error())
}

const (
	memoryCues  = 200000
	memoryLimit = 2 << 20
	sampleEvery = memoryCues / 20
	minFileSize = 10 << 20
)

// heapGrowth runs fn, which calls sample now and then, and returns the highest live heap size that sample saw
// above the live heap size before fn. Sample runs a full collection first, so the size holds what fn keeps
// and no garbage. The garbage between two collections depends on the machine, and the test does not measure it.
func heapGrowth(fn func(sample func())) uint64 {
	runtime.GC()
	before := liveHeap()
	var peak uint64
	fn(func() {
		runtime.GC()
		peak = max(peak, liveHeap())
	})
	return peak - min(peak, before)
}

func memoryCue(i int) model.Cue {
	return model.Cue{
		Start: time.Duration(i*2) * time.Second,
		End:   time.Duration(i*2)*time.Second + 1500*time.Millisecond,
		Lines: []string{"Light event number " + strconv.Itoa(i), "<i>Zone</i> " + strconv.Itoa(i%7)},
	}
}

// liveHeap reads the bytes of the heap objects that the last garbage collection marked as live.
func liveHeap() uint64 {
	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(sample)
	return sample[0].Value.Uint64()
}
