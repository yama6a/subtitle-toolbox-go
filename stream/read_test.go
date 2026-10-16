package stream_test

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stream"
)

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

func TestSubRipReaderMatchesTheFullReaderOnEdgeCases(t *testing.T) {
	t.Parallel()

	for name, content := range subRipInputs {
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(name+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareSubRip(t, []byte(content), w.wrap, lenient)
				})
			}
		}
	}
}

func TestWebVTTReaderMatchesTheFullReaderOnEdgeCases(t *testing.T) {
	t.Parallel()

	for name, content := range webVTTInputs {
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(name+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareWebVTT(t, []byte(content), w.wrap, lenient)
				})
			}
		}
	}
}

func TestReadersGiveNoCuesForEmptyContent(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", bom + "\n \n", "\x1a", "\x01\x02"} {
		cues, err := collect(stream.NewSubRipReader(strings.NewReader(content), stream.ReadOptions{}).Cues())
		require.NoError(t, err)
		assert.Empty(t, cues)

		cues, err = collect(stream.NewWebVTTReader(strings.NewReader(content), stream.ReadOptions{Lenient: true}).Cues())
		require.NoError(t, err)
		assert.Empty(t, cues)
	}
}

func TestReadersKeepTheFileOrder(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:05,000 --> 00:00:06,000\nLate\n\n2\n00:00:01,000 --> 00:00:02,000\nEarly\n"
	cues, err := collect(stream.NewSubRipReader(strings.NewReader(content), stream.ReadOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 2)
	assert.Equal(t, "Late", cues[0].Text())
	assert.Equal(t, "Early", cues[1].Text())
}

func TestWebVTTReaderSkipsNotes(t *testing.T) {
	t.Parallel()

	rd := stream.NewWebVTTReader(strings.NewReader(webVTTInputs["note between cues"]), stream.ReadOptions{})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 2)
	assert.Equal(t, "B", cues[1].Text())
}

func TestWebVTTHeaderIsCompleteAtTheFirstCue(t *testing.T) {
	t.Parallel()

	content := "WEBVTT Title\n\nREGION\nid:a\n\nSTYLE\n::cue { color: red; }\n\n00:01.000 --> 00:02.000\nA\n"
	rd := stream.NewWebVTTReader(strings.NewReader(content), stream.ReadOptions{})
	seen := 0
	for _, err := range rd.Cues() {
		require.NoError(t, err)
		seen++
		title, _ := rd.Header().String("header")
		assert.Equal(t, "Title", title)
		regions, _ := rd.Header().List("regions")
		assert.Len(t, regions, 1)
		styles, _ := rd.Header().Strings("styles")
		assert.Equal(t, []string{"::cue { color: red; }"}, styles)
	}
	assert.Equal(t, 1, seen)
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
				return collect(stream.NewSubRipReader(r, stream.ReadOptions{}).Cues())
			},
			line: 6,
		},
		{
			name:    "SubRip bad time",
			content: "1\n00:00:01,000 --> 00:00:02,000\nGood\n\n2\n00:00:0x,000 --> 00:00:04,000\nIt is late.\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewSubRipReader(r, stream.ReadOptions{}).Cues())
			},
			line: 6,
		},
		{
			name:    "WebVTT",
			content: "WEBVTT\n\n00:01.000 --> 00:02.000\nGood\n\nbroken block\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues())
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

			_, err := collect(stream.NewWebVTTReader(strings.NewReader(tt.content), stream.ReadOptions{}).Cues())
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

	rd := stream.NewWebVTTReader(bytes.NewReader(testfiles.Read(t, "lenient/bad_timestamp.srt")), stream.ReadOptions{Lenient: true})
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
				rd := stream.NewSubRipReader(w.wrap(strings.NewReader(tt.content)), stream.ReadOptions{})
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

	srt := stream.NewSubRipReader(strings.NewReader("1\n00:00:01,000 --> 00:00:02,000\nThe\f fer\x00ry\x7f.\r\n\x1a"), stream.ReadOptions{Lenient: true})
	cues, err := collect(srt.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 1)
	assert.Equal(t, "The ferry.", cues[0].Text())
	require.Len(t, srt.Warnings(), 1)
	assert.Equal(t, 3, srt.Warnings()[0].Line)

	vtt := stream.NewWebVTTReader(strings.NewReader("WEBVTT\n\n00:01.000 --> 00:02.000\nThe\f fer\x00ry\x7f.\r\n\x1a"), stream.ReadOptions{Lenient: true})
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
				rd := stream.NewWebVTTReader(bytes.NewReader(content), stream.ReadOptions{Lenient: true})
				cues, err = collect(rd.Cues())
				warnings = rd.Warnings()
			} else {
				rd := stream.NewSubRipReader(bytes.NewReader(content), stream.ReadOptions{Lenient: true})
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
	rd := stream.NewSubRipReader(iotest.OneByteReader(strings.NewReader(content)), stream.ReadOptions{Lenient: true})
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

	rd := stream.NewWebVTTReader(transform.NewReader(bytes.NewReader(raw), unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()), stream.ReadOptions{})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	assert.Equal(t, cuesOf(s), inStartOrder(cues))
}

func TestWebVTTReaderSkipsTheSecondHeaderOfJoinedFiles(t *testing.T) {
	t.Parallel()

	rd := stream.NewWebVTTReader(bytes.NewReader(testfiles.Read(t, "lenient/joined_files.vtt")), stream.ReadOptions{Lenient: true})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 2)
	assert.Equal(t, []string{"One"}, cues[0].Lines)
	assert.Equal(t, []string{"Two"}, cues[1].Lines)
	warnings := rd.Warnings()
	lines := make([]int, 0, len(warnings))
	for _, w := range warnings {
		lines = append(lines, w.Line)
	}
	assert.Equal(t, []int{6, 9}, lines)
}

func TestWarningsGrowWhileTheLoopRuns(t *testing.T) {
	t.Parallel()

	content := "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\nbad\n\n3\n00:00:03,000 --> 00:00:04,000\nThree\n"
	rd := stream.NewSubRipReader(strings.NewReader(content), stream.ReadOptions{Lenient: true})
	var counts []int
	for _, err := range rd.Cues() {
		require.NoError(t, err)
		counts = append(counts, len(rd.Warnings()))
	}
	assert.Equal(t, []int{0, 1}, counts)
}

func TestCuesYieldsNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	rd := stream.NewSubRipReader(strings.NewReader(subRipInputs["BOM and CRLF"]), stream.ReadOptions{})
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
	for _, err := range stream.NewSubRipReader(counter, stream.ReadOptions{}).Cues() {
		require.NoError(t, err)
		break
	}
	assert.Less(t, counter.n, content.Len()/4)
}

func TestReadErrorEndsTheSequenceAfterTheCuesBeforeIt(t *testing.T) {
	t.Parallel()

	boom := errors.New("disk gone")
	r := io.MultiReader(strings.NewReader("1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo\n\n3\n00:00"), errReader{boom})
	cues, err := collect(stream.NewSubRipReader(r, stream.ReadOptions{}).Cues())
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
	w, err := stream.NewSubRipWriter(&out, model.CommonWriteOptions{}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Write(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"A"}}))
	require.NoError(t, w.Close())
	cues, err := collect(stream.NewSubRipReader(&out, stream.ReadOptions{}).Cues())
	require.NoError(t, err)
	require.Len(t, cues, 1)
	assert.Equal(t, "A", cues[0].Text())
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += n
	return n, err //nolint:wrapcheck // a test double passes the error through
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

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
				return collect(stream.NewSubRipReader(r, stream.ReadOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "SubRip cut text",
			input: "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nTwo li",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewSubRipReader(r, stream.ReadOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT cut text",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo li",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT before a line of spaces",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo\n \n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues())
			},
			want: []string{"One"},
		},
		{
			name:  "WebVTT before spaces and an empty line",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n00:03.000 --> 00:04.000\nTwo\n \n\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues())
			},
			want: []string{"One", "Two"},
		},
		{
			name:  "WebVTT after an empty line",
			input: "WEBVTT\n\n00:01.000 --> 00:02.000\nOne\n\n",
			read: func(r io.Reader) ([]model.Cue, error) {
				return collect(stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues())
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
	cues, err := collect(stream.NewSubRipReader(strings.NewReader(content), stream.ReadOptions{}).Cues())
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, 4, perr.Line)
	assert.Empty(t, cues)

	content = "1\n00:00:01,000 --> 00:00:02,000\nOne\n\n2\n00:00:03,000 --> 00:00:04,000\nHello\nWor\x00ld\n"
	cues, err = collect(stream.NewSubRipReader(strings.NewReader(content), stream.ReadOptions{}).Cues())
	require.ErrorAs(t, err, &perr)
	require.Len(t, cues, 1)
	assert.Equal(t, "One", cues[0].Text())
}
