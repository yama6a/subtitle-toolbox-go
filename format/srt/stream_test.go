package srt_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestSubRipReaderMatchesTheFullReaderOnEveryFixture(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".srt") {
		content := testfiles.Read(t, rel)
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(rel+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareSubRip(t, content, w.wrap, lenient)
				})
			}
		}
	}
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
				_, err := srt.Write(&want, s, o.common, srt.WriteOptions{})
				require.NoError(t, err)
				w, err := srt.NewStreamWriter(&got, o.common, srt.WriteOptions{})
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

func TestSubRipWriterWritesCRLFWithoutBOM(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w, err := srt.NewStreamWriter(&out, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, srt.WriteOptions{})
	require.NoError(t, err)
	require.NoError(t, w.Write(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"A"}}))
	require.NoError(t, w.Write(model.Cue{Start: 3 * time.Second, End: 4500 * time.Millisecond, Lines: []string{"B"}}))
	require.NoError(t, w.Close())
	assert.Equal(t, "1\r\n00:00:01,000 --> 00:00:02,000\r\nA\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,500\r\nB\r\n", out.String())
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
