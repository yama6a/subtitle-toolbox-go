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
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

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

func TestReadersKeepTheFileOrder(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:05,000 --> 00:00:06,000\nLate\n\n2\n00:00:01,000 --> 00:00:02,000\nEarly\n"
	cues, err := collect(srt.NewStreamReader(strings.NewReader(content), srt.StreamOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 2)
	assert.Equal(t, "Late", cues[0].Text())
	assert.Equal(t, "Early", cues[1].Text())
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

// Ported from tests/Parsers/ParseErrorLineNumbersTest.php, brokenStreams.
func TestStrictWebVTTReaderReportsTheLineOfTheError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		msg     string
		line    int
	}{
		{"without WEBVTT", "\n\nThe ferry leaves.\n", "The file does not start with WEBVTT.", 3},
		{"arrow in the header", webVTTInputs["arrow in the header"], "The WEBVTT header has no empty line before the first cue.", 2},
		{"bad block", webVTTInputs["bad block"], "", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := collect(webvtt.NewStreamReader(strings.NewReader(tt.content), webvtt.StreamOptions{}).Cues())
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.line, perr.Line)
			assert.Equal(t, model.WebVTT, perr.Format)
			assert.Contains(t, perr.Msg, tt.msg)
		})
	}
}

func TestLenientWebVTTReaderWithoutTheSignatureStillFails(t *testing.T) {
	t.Parallel()

	rd := webvtt.NewStreamReader(bytes.NewReader(testfiles.Read(t, "lenient/bad_timestamp.srt")), webvtt.StreamOptions{Lenient: true})
	cues, err := collect(rd.Cues())
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The file does not start with WEBVTT.", perr.Msg)
	assert.Empty(t, cues)
}

func TestStrictNULFailsOnItsLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
	}{
		{"in the text", "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nThe fer\x00ry leaves.\n", 7},
		{"between CR and LF", "1\r\x00\n00:00:01,000 --> 00:00:02,000\r\x00\nOne\r\x00\n", 2},
		{"after a BOM and a Ctrl-Z", bom + "1\x1a\n\x00", 2},
	}
	for _, tt := range tests {
		for _, w := range wraps {
			t.Run(tt.name+"/"+w.name, func(t *testing.T) {
				t.Parallel()

				want := "Line " + strconv.Itoa(tt.line) + " has a NUL character. Check the encoding of the file."
				rd := srt.NewStreamReader(w.wrap(strings.NewReader(tt.content)), srt.StreamOptions{})
				_, err := collect(rd.Cues())
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.line, perr.Line)
				assert.Equal(t, want, perr.Msg)

				_, _, fullErr := srt.Read(strings.NewReader(tt.content), model.CommonReadOptions{}, srt.ReadOptions{})
				require.ErrorAs(t, fullErr, &perr)
				assert.Equal(t, want, perr.Msg)
			})
		}
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

func TestInvalidUTF8OffsetCountsTheBytesBeforeTheBadByte(t *testing.T) {
	t.Parallel()

	content := bom + "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nT\xc3\xa9\xffo\n"
	rd := srt.NewStreamReader(iotest.OneByteReader(strings.NewReader(content)), srt.StreamOptions{Lenient: true})
	_, err := collect(rd.Cues())
	require.NoError(t, err)
	want := strings.Index(content, "\xff")
	require.Positive(t, want)
	assert.Contains(t, rd.Warnings()[0].Message, "offset "+strconv.Itoa(want)+".")
}

func TestReadersReadUTF16ThroughATransformReader(t *testing.T) {
	t.Parallel()

	raw := testfiles.Read(t, "encoding/notepad-utf-16le.vtt")
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(raw), unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()))
	require.NoError(t, err)
	s, _, err := webvtt.Read(bytes.NewReader(decoded), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)

	rd := webvtt.NewStreamReader(transform.NewReader(bytes.NewReader(raw), unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()), webvtt.StreamOptions{})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	assert.Equal(t, cuesOf(s), inStartOrder(cues))
}

func TestWarningsGrowWhileTheLoopRuns(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\nbad\n\n3\n00:00:03,000 --> 00:00:04,000\nThree\n"
	rd := srt.NewStreamReader(strings.NewReader(content), srt.StreamOptions{Lenient: true})
	var counts []int
	for _, err := range rd.Cues() {
		require.NoError(t, err)
		counts = append(counts, len(rd.Warnings()))
	}
	assert.Equal(t, []int{0, 1}, counts)
}

func TestCuesYieldsNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	rd := srt.NewStreamReader(strings.NewReader(subRipInputs["BOM and CRLF"]), srt.StreamOptions{})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	assert.Len(t, cues, 2)
	cues, err = collect(rd.Cues())
	require.NoError(t, err)
	assert.Empty(t, cues)
}

func TestBreakingOutOfTheLoopStopsTheRead(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	for i := range 20000 {
		content.WriteString(strconv.Itoa(i+1) + "\n00:00:01,000 --> 00:00:02,000\nLine number " + strconv.Itoa(i) + "\n\n")
	}
	counter := &countingReader{r: strings.NewReader(content.String())}
	for _, err := range srt.NewStreamReader(counter, srt.StreamOptions{}).Cues() {
		require.NoError(t, err)
		break
	}
	assert.Less(t, counter.n, content.Len()/4)
}

func TestReadErrorEndsTheSequenceAfterTheCuesBeforeIt(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk gone")
	r := io.MultiReader(strings.NewReader("1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo\n\n3\n00:00"), errReader{boom})
	cues, err := collect(srt.NewStreamReader(r, srt.StreamOptions{}).Cues())
	require.ErrorIs(t, err, boom)
	assert.Len(t, cues, 2)
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

func TestReadersRoundTripThroughWriters(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := srt.NewStreamWriter(&out, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Write(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"A"}}))
	require.NoError(t, w.Close())
	cues, err := collect(srt.NewStreamReader(&out, srt.StreamOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 1)
	assert.Equal(t, "A", cues[0].Text())
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

func TestAStrictNULDropsTheBlockThatItCut(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:01,000 --> 00:00:02,000\nHello\nWor\x00ld\n"
	cues, err := collect(srt.NewStreamReader(strings.NewReader(content), srt.StreamOptions{}).Cues())
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, 4, perr.Line)
	assert.Empty(t, cues)

	content = "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nHello\nWor\x00ld\n"
	cues, err = collect(srt.NewStreamReader(strings.NewReader(content), srt.StreamOptions{}).Cues())
	require.ErrorAs(t, err, &perr)
	require.Len(t, cues, 1)
	assert.Equal(t, "One", cues[0].Text())
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

// Ported from tests/Streaming/StreamWriterTest.php, testCopiesTheWebVttHeaderFromReaderToWriter.
func TestCopiesTheWebVTTHeaderFromReaderToWriter(t *testing.T) {
	t.Parallel()

	rd := webvtt.NewStreamReader(bytes.NewReader(testfiles.Read(t, "vtt/real/w3c_regions.vtt")), webvtt.StreamOptions{})
	var out bytes.Buffer
	var w *webvtt.StreamWriter
	for c, err := range rd.Cues() {
		require.NoError(t, err)
		if w == nil {
			var err error
			w, err = webvtt.NewStreamWriter(&out, model.CommonWriteOptions{}, webvtt.WriteOptions{}, rd.Header())
			require.NoError(t, err)
		}
		require.NoError(t, w.Write(c))
	}
	require.NoError(t, w.Close())

	copied := webvtt.NewStreamReader(&out, webvtt.StreamOptions{})
	_, err := collect(copied.Cues())
	require.NoError(t, err)
	assert.Equal(t, rd.Header(), copied.Header())
	regions, _ := copied.Header().List("regions")
	assert.NotEmpty(t, regions)
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

func TestCloseLeavesTheCallersWriterOpen(t *testing.T) {
	t.Parallel()

	spy := &closeSpy{}
	w, err := srt.NewStreamWriter(spy, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close())
	assert.False(t, spy.closed)
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

func TestDroppedColorsReturnsACopy(t *testing.T) {
	t.Parallel()

	w, err := webvtt.NewStreamWriter(&bytes.Buffer{}, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	var c model.Cue
	c.SetText(`<font color="#123456">Hi</font>`)
	require.NoError(t, w.Write(c))
	got := w.DroppedColors()
	got[0].Color = "changed"
	assert.True(t, slices.ContainsFunc(w.DroppedColors(), func(d webvtt.DroppedColor) bool { return d.Color == "#123456" }))
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

// inStartOrder puts cues through a subtitle, which sorts them by start as the full readers do.
func inStartOrder(cues []model.Cue) []model.Cue {
	s := &model.Subtitle{}
	s.AddCues(cues...)
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

// liveHeap reads the bytes of the heap objects that the last garbage collection marked as live.
func liveHeap() uint64 {
	sample := []metrics.Sample{{Name: "/gc/heap/live:bytes"}}
	metrics.Read(sample)
	return sample[0].Value.Uint64()
}

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

type cueWriter interface {
	Write(c model.Cue) error
	Close() error
}

func memoryCue(i int) model.Cue {
	return model.Cue{
		Start: time.Duration(i*2) * time.Second,
		End:   time.Duration(i*2)*time.Second + 1500*time.Millisecond,
		Lines: []string{"Light event number " + strconv.Itoa(i), "<i>Zone</i> " + strconv.Itoa(i%7)},
	}
}

const bom = "\xEF\xBB\xBF"

// Ported from tests/Streaming/StreamReaderEdgeCasesTest.php, subRipInputs.
var subRipInputs = map[string]string{
	"BOM and CRLF":               bom + "1\r\n00:00:01,000 --> 00:00:02,000\r\nFirst\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nSecond\r\n",
	"CR only":                    "1\r00:00:01,000 --> 00:00:02,000\rFirst\r\r2\r00:00:03,000 --> 00:00:04,000\rSecond\r",
	"two CRs and LF":             "1\r\r\n00:00:01,000 --> 00:00:02,000\r\r\nFirst\r\r\n\r\r\n2\r\r\n00:00:03,000 --> 00:00:04,000\r\r\nSecond",
	"blank lines and spaces":     "\n\n  1\n00:00:01,000  -->  00:00:02,000\n\tA \t tab\n \n\t\n\n2\n00:00:03,000 --> 00:00:04,000\nB & <i>c</i> <3\n\n\n",
	"BOM only":                   bom,
	"empty":                      "",
	"white space only":           " \n\t\r\n\n  ",
	"no text":                    "1\n00:00:01,000 --> 00:00:02,000\n\n2\n00:00:03,000 --> 00:00:04,000\nB\n",
	"BOM at the start of a line": "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n" + bom + bom + "2\n00:00:03,000 --> 00:00:04,000\nTwo\n",
	"two BOMs at the start":      bom + bom + "1\n00:00:01,000 --> 00:00:02,000\nOne\n",
	"trailing Ctrl-Z":            "1\n00:00:01,000 --> 00:00:02,000\nOne\n\x1a",
	"Ctrl-Z and blank lines":     "1\n00:00:01,000 --> 00:00:02,000\nOne\r\n\x1a \r\n \n",
	"Ctrl-Z in the middle":       "1\n00:00:01,000 --> 00:00:02,000\nO\x1ane\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo\n",
	"two Ctrl-Z":                 "1\n00:00:01,000 --> 00:00:02,000\nOne\x1a\x1a\n\x1a\n",
	"control characters":         "1\n00:00:01,000 --> 00:00:02,000\nThe\f fer\x01ry\x7f.\n",
	"NUL between CR and LF":      "1\r\x00\n00:00:01,000 --> 00:00:02,000\r\x00\nOne\r\x00\n",
	"NUL":                        "1\n00:00:01,000 --> 00:00:02,000\nThe fer\x00ry leaves.\n",
	"one bad byte":               "1\n00:00:01,000 --> 00:00:02,000\nThe fer\xffry leaves.\n",
	"cut multibyte character":    "1\n00:00:01,000 --> 00:00:02,000\nThe caf\xc3",
}

// Ported from tests/Streaming/StreamReaderEdgeCasesTest.php, webVttInputs.
var webVTTInputs = map[string]string{
	"BOM and CRLF":               bom + "WEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nFirst\r\n",
	"CR only":                    "WEBVTT\r\r00:01.000 --> 00:02.000\rFirst\r\r00:03.000 --> 00:04.000\rSecond",
	"two CRs and LF":             "WEBVTT\r\r\n\r\r\n00:01.000 --> 00:02.000\r\r\nFirst\r\r\n",
	"white space around":         "\n \n  WEBVTT  title \nX-Meta: 1 \n\nSTYLE\n::cue { color: red; }  \n\n\n00:01.000 --> 00:02.000\nA\n \n\t\n",
	"header only":                "WEBVTT\nKind: captions  \n  \n",
	"style last":                 "WEBVTT\n\nSTYLE\n::cue { color: red; }   \n\n",
	"style after cue":            "WEBVTT\n\n00:01.000 --> 00:02.000\nA\n\nSTYLE\n::cue { color: red; }\n",
	"cue without gap":            "WEBVTT\n\n00:01.000 --> 00:02.000\nA\n00:03.000 --> 00:04.000\nB\n",
	"region and note":            "WEBVTT\n\nREGION\nid:a width:40%\n\nNOTE x\n\n00:01.000 --> 00:02.000 region:a\nA\n",
	"missing gap after header":   "WEBVTT\n00:01.000 --> 00:02.000\nA\n",
	"empty":                      "",
	"unknown block":              "WEBVTT\n\nhello\n",
	"BOM only":                   bom,
	"white space only":           bom + "\n \n",
	"BOM after blank lines":      "\n" + bom + "WEBVTT\n\n00:01.000 --> 00:02.000\nA\n",
	"BOM at the start of a line": "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n" + bom + "00:03.000 --> 00:04.000\nTwo\n",
	"without WEBVTT":             "\n\nThe ferry leaves.\n",
	"arrow in the header":        "WEBVTT\n00:00:01.000 --> 00:00:02.000\nThe ferry leaves.\n",
	"bad block":                  "WEBVTT\n\nIt is late.\nVery late.\n\n00:00:01.000 --> 00:00:02.000\nThe ferry leaves.\n",
	"trailing Ctrl-Z":            "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\x1a",
	"control characters":         "WEBVTT\n\n00:01.000 --> 00:02.000\nThe\f fer\x01ry\x7f.\n",
	"NUL":                        "WEBVTT\n\n00:01.000 --> 00:02.000\nThe fer\x00ry leaves.\n",
	"one bad byte":               "WEBVTT\n\n00:01.000 --> 00:02.000\nThe fer\xffry leaves.\n",
	"rolling cues":               "WEBVTT\n\n00:01.000 --> 00:03.000\nA<00:01.500><c> b</c>\n\n00:03.000 --> 00:03.010\nA b\n\n00:03.010 --> 00:05.000\nA b\nC<00:04.000><c> d</c>\n",
	"note between cues":          "WEBVTT\n\n00:01.000 --> 00:02.000\nA\n\nNOTE x\ny\n\n00:03.000 --> 00:04.000\nB\n",
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += n
	return n, err
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

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
