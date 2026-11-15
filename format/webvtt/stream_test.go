package webvtt_test

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"path/filepath"
	"slices"
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

func TestWebVTTReaderMatchesTheFullReaderOnEveryFixture(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".vtt") {
		content := testfiles.Read(t, rel)
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(rel+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareWebVTT(t, content, w.wrap, lenient)
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

func TestWebVTTReaderSkipsNotes(t *testing.T) {
	t.Parallel()

	rd := webvtt.NewStreamReader(strings.NewReader(webVTTInputs["note between cues"]), webvtt.StreamOptions{})
	cues, err := collect(rd.Cues())
	require.NoError(t, err)
	require.Len(t, cues, 2)
	assert.Equal(t, "B", cues[1].Text())
}

func TestWebVTTHeaderIsCompleteAtTheFirstCue(t *testing.T) {
	t.Parallel()

	content := "WEBVTT Title\n\nREGION\nid:a\n\nSTYLE\n::cue { color: red; }\n\n00:01.000 --> 00:02.000\nA\n"
	rd := webvtt.NewStreamReader(strings.NewReader(content), webvtt.StreamOptions{})
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

func TestWebVTTReaderSkipsTheSecondHeaderOfJoinedFiles(t *testing.T) {
	t.Parallel()

	rd := webvtt.NewStreamReader(bytes.NewReader(testfiles.Read(t, "lenient/joined_files.vtt")), webvtt.StreamOptions{Lenient: true})
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
				report, err := webvtt.Write(&want, s, o.common, webvtt.WriteOptions{})
				require.NoError(t, err)
				w, err := webvtt.NewStreamWriter(&got, o.common, webvtt.WriteOptions{}, header)
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

func TestWebVTTWriterEscapesTheTimingArrowInCueText(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := webvtt.NewStreamWriter(&out, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
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
	w, err := webvtt.NewStreamWriter(&out, common, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	for c, err := range srt.NewStreamReader(bytes.NewReader(content), srt.StreamOptions{}).Cues() {
		require.NoError(t, err)
		require.NoError(t, w.Write(c))
	}

	s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	var want bytes.Buffer
	report, err := webvtt.Write(&want, s, common, webvtt.WriteOptions{})
	require.NoError(t, err)
	assert.Equal(t, want.String(), out.String())
	assert.Equal(t, report.DroppedColors, w.DroppedColors())
	require.Len(t, w.DroppedColors(), 1)
	assert.Equal(t, "#123456", w.DroppedColors()[0].Color)
}

func TestWebVTTWriterListsNoDroppedColorWhenItStripsTags(t *testing.T) {
	t.Parallel()

	w, err := webvtt.NewStreamWriter(&bytes.Buffer{}, model.CommonWriteOptions{StripTags: true}, webvtt.WriteOptions{}, nil)
	require.NoError(t, err)
	var c model.Cue
	c.SetText(`<font color="#123456">Hi</font>`)
	require.NoError(t, w.Write(c))
	assert.Empty(t, w.DroppedColors())
}

// fixtureFiles returns the paths, relative to testdata, of every file with the extension ext.
func fixtureFiles(tb testing.TB, ext string) []string {
	tb.Helper()

	root := testfiles.Root(tb)
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == ext {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return fmt.Errorf("relative path of %s: %w", p, err)
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(tb, err)
	require.NotEmpty(tb, files)
	return files
}

func modeName(lenient bool) string {
	if lenient {
		return "lenient"
	}
	return "strict"
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
