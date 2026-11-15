package stbox

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

const utf8BOM = "\ufeff"

func utf8Text(content string) textenc.Decoded {
	return textenc.Decoded{Text: []byte(content), Encoding: "UTF-8"}
}

func TestDecodeWarningComesFirstInLenientMode(t *testing.T) {
	t.Parallel()

	const msg = "The content is UTF-16LE without a BOM."
	text := textenc.Decoded{Text: []byte(srtCue("Hel\x01lo")), Encoding: "UTF-16LE", Warning: msg}
	_, info, err := read(SubRip, text, ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Equal(t, "UTF-16LE", info.SourceEncoding)
	assert.Equal(t, []ParseWarning{
		{Message: msg, Line: NoLine, Block: NoBlock, Action: Repaired},
		{Message: "The file has control characters, the first on line 3. The parser removed them.", Line: 3, Block: NoBlock, Action: Repaired},
	}, info.Warnings)

	_, info, err = read(SubRip, text, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, "UTF-16LE", info.SourceEncoding)
	assert.Empty(t, info.Warnings)
}

func TestReadBuildsReadInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		lenient  bool
		cues     int
		warnings int
	}{
		{name: "two cues", content: srtCue("Hello") + "\n2\n00:00:03,000 --> 00:00:04,000\nworld\n", cues: 2},
		{name: "strict removes a control character without a warning", content: srtCue("Hel\x01lo"), cues: 1},
		{name: "lenient warns for a control character", content: srtCue("Hel\x01lo"), lenient: true, cues: 1, warnings: 1},
		{name: "empty content gives an empty subtitle", content: ""},
		{name: "blank content keeps the warning", content: "\x01\n", lenient: true, warnings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := read(SubRip, utf8Text(tt.content), ReadOptions{Lenient: tt.lenient})
			require.NoError(t, err)
			assert.Equal(t, tt.cues, s.Len())
			assert.Equal(t, SubRip, info.Format)
			assert.Equal(t, "UTF-8", info.SourceEncoding)
			assert.Len(t, info.Warnings, tt.warnings)
		})
	}
}

func TestReadErrorGivesNoSubtitleAndNoInfo(t *testing.T) {
	t.Parallel()

	s, info, err := read(SubRip, utf8Text(srtCue("The fer\x00ry")), ReadOptions{})
	var perr *ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, 3, perr.Line)
	assert.Equal(t, SubRip, perr.Format)
	assert.Nil(t, s)
	assert.Zero(t, info)
}

// Ports testEachParseCallStartsWithoutWarnings and testAStrictParseAfterALenientParseThrows
// of tests/Parsers/LenientParsingTest.php of subtitle-toolbox-php.
func TestEachReadStartsWithoutWarnings(t *testing.T) {
	t.Parallel()

	broken := utf8Text(srtCue("The fer\x00ry"))
	_, info, err := read(SubRip, broken, ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Equal(t, []ParseWarning{{
		Message: "The file has control characters, the first on line 3. The parser removed them.",
		Line:    3,
		Block:   NoBlock,
		Action:  Repaired,
	}}, info.Warnings)

	_, info, err = read(SubRip, utf8Text(srtCue("Hello")), ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Empty(t, info.Warnings)

	_, _, err = read(SubRip, broken, ReadOptions{})
	require.ErrorIs(t, err, ErrParse)
}

func TestReadPassesLastCueDuration(t *testing.T) {
	t.Parallel()

	last := new(7 * time.Second)
	sub, _, err := read(TMPlayer, utf8Text("00:00:01:Hello\n"), ReadOptions{LastCueDuration: last})
	require.NoError(t, err)
	require.Equal(t, 1, sub.Len())
	cue, _ := sub.Cue(0)
	assert.Equal(t, 8*time.Second, cue.End)
}

func srtCue(text string) string {
	return "1\n00:00:01,000 --> 00:00:02,000\n" + text + "\n"
}

// textReaders returns every format with a reader that is not binary.
func textReaders() []Format {
	return slices.DeleteFunc(Formats(), func(f Format) bool { return !f.CanRead() || f.IsBinary() })
}

// Ports testEmptyContentGivesAnEmptySubtitleInBothModes of tests/Parsers/NoCuesTest.php of subtitle-toolbox-php.
func TestEmptyContentGivesAnEmptySubtitle(t *testing.T) {
	t.Parallel()

	contents := []struct{ name, text string }{{"empty", ""}, {"BOM and white space", utf8BOM + " \r\n\t\n"}}
	for _, f := range textReaders() {
		for _, content := range contents {
			for _, lenient := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s, %s, lenient %t", f, content.name, lenient), func(t *testing.T) {
					t.Parallel()

					s, info, err := read(f, utf8Text(content.text), ReadOptions{Format: f, Lenient: lenient})
					require.NoError(t, err)
					assert.Zero(t, s.Len())
					assert.Empty(t, info.Warnings)
					assert.Equal(t, f, info.Format)
				})
			}
		}
	}
}

// writesBOM lists the formats whose output starts with a UTF-8 BOM by default.
var writesBOM = map[Format]bool{
	ASS: true, CSV: true, TSV: true, LRC: true, MPSub: true, SubRip: true, WebVTT: true,
}

// textWriters returns every format with a writer that is not binary. A binary format ignores the line ending and the BOM.
func textWriters() []Format {
	return slices.DeleteFunc(Formats(), func(f Format) bool { return !f.CanWrite() || f.IsBinary() })
}

func writeString(t *testing.T, f Format, s *Subtitle, opts WriteOptions) string {
	t.Helper()

	if opts.MicroDVD.FrameRate.IsZero() {
		opts.MicroDVD.FrameRate = FrameRate{Num: 25, Den: 1} // a subtitle without MicroDVD format data has no frame rate to write with
	}
	if opts.ITT.FrameRate.IsZero() {
		opts.ITT.FrameRate = FrameRate25 // a subtitle without iTT format data has no frame rate to write with
	}
	require.NoError(t, opts.Validate())
	var out bytes.Buffer
	_, err := writeFormat(&out, s, f, opts)
	require.NoError(t, err)
	return out.String()
}

// outputSubtitle returns the subtitle of the output option tests. CSV and TSV keep a line break inside a cell as
// LF, so their cue holds one line.
func outputSubtitle(t *testing.T, f Format) *Subtitle {
	t.Helper()

	lines := []string{"<i>Bonjour</i>", "ca va ?"}
	if f == CSV || f == TSV {
		lines = []string{"<i>Bonjour</i> ca va ?"}
	}
	s := &Subtitle{}
	s.SetMetadata(MetadataTitle, "Cafe")
	require.NoError(t, s.AddCue(Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: lines}))
	require.NoError(t, s.AddCue(Cue{Start: 5 * time.Second, End: 7250 * time.Millisecond, Lines: []string{"Second cue"}}))
	require.NoError(t, s.AddComment("Note", 1))
	if f == DCP {
		header := model.NewData(model.Field{Key: "IssueDate", Value: "2026-10-12T00:00:00"})
		require.NoError(t, s.SetFormatData("dcp", model.NewData(model.Field{Key: "header", Value: header})))
	}
	return s
}

// Ports tests/Formatters/OutputOptionsTest.php of subtitle-toolbox-php for the text formats.
func TestOutputOptions(t *testing.T) {
	t.Parallel()

	for _, f := range textWriters() {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s := outputSubtitle(t, f)
			def := writeString(t, f, s, WriteOptions{})
			assert.Equal(t, writesBOM[f], strings.HasPrefix(def, utf8BOM), "format BOM")
			assert.NotContains(t, def, "\r")
			assert.Equal(t, def, writeString(t, f, s, WriteOptions{LineEnding: LF}))

			plain := strings.TrimPrefix(def, utf8BOM)
			crlf := strings.ReplaceAll(def, "\n", "\r\n")
			assert.Equal(t, crlf, writeString(t, f, s, WriteOptions{LineEnding: CRLF}))
			assert.Equal(t, utf8BOM+plain, writeString(t, f, s, WriteOptions{BOM: BOMAdd}))
			assert.Equal(t, plain, writeString(t, f, s, WriteOptions{BOM: BOMRemove}))
			assert.Equal(t, utf8BOM+strings.ReplaceAll(plain, "\n", "\r\n"), writeString(t, f, s, WriteOptions{LineEnding: CRLF, BOM: BOMAdd}))
			assert.Equal(t, strings.ReplaceAll(plain, "\n", "\r\n"), writeString(t, f, s, WriteOptions{LineEnding: CRLF, BOM: BOMRemove}))
		})
	}
}

// Ports testANegativeTimeIsWrittenAsZero of tests/Formatters/NegativeCueTimesTest.php of subtitle-toolbox-php.
func TestNegativeTimesAreWrittenAsZero(t *testing.T) {
	t.Parallel()

	for _, f := range textWriters() {
		if f == JSON {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			negative, zero := &Subtitle{}, &Subtitle{}
			require.NoError(t, negative.AddCues(Cue{Start: -2500 * time.Millisecond, End: -1500 * time.Millisecond, Lines: []string{"One"}}, Cue{Start: -500 * time.Millisecond, End: time.Second, Lines: []string{"Two"}}))
			require.NoError(t, zero.AddCues(Cue{Lines: []string{"One"}}, Cue{End: time.Second, Lines: []string{"Two"}}))
			assert.Equal(t, writeString(t, f, zero, WriteOptions{}), writeString(t, f, negative, WriteOptions{}))
		})
	}
}

// Ports testStripTagsChangesTheOutput of tests/Formatters/OutputOptionsTest.php of subtitle-toolbox-php.
func TestStripTagsChangesTheOutput(t *testing.T) {
	t.Parallel()

	for _, f := range []Format{ASS, MicroDVD, SAMI, SubRip, TTML, WebVTT} {
		if !f.CanWrite() {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s := outputSubtitle(t, f)
			assert.NotEqual(t, writeString(t, f, s, WriteOptions{}), writeString(t, f, s, WriteOptions{StripTags: true}))
		})
	}
}
