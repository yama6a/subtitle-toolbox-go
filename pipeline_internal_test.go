package stbox

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	utf8BOM    = "\ufeff"
	stubFormat = Format("stub")
)

// stubRead reads each line of text as a cue of 1 s. It is a reader of the shape that the format packages have.
func stubRead(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read stub: %w", err)
	}
	st := parsing.NewState(stubFormat, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	for i, line := range strings.Split(strings.TrimRight(string(content), "\r\n"), "\n") {
		start := time.Duration(i) * time.Second
		s.AddCue(Cue{Start: start, End: start + time.Second, Lines: []string{strings.TrimRight(line, "\r")}})
	}
	return s, st.Warnings(), nil
}

// stubWrite writes the lines of each cue and an empty line after each cue. Its format default is a BOM.
func stubWrite(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var b strings.Builder
	for _, c := range s.All() {
		b.WriteString(strings.Join(c.Lines, "\n") + "\n\n")
	}
	return writing.Write(w, stubFormat, []byte(b.String()), common, true)
}

var stubRow = formatFuncs{format: stubFormat, read: stubRead, write: stubWrite}

func utf8Text(content string) textenc.Decoded {
	return textenc.Decoded{Text: []byte(content), Encoding: "UTF-8"}
}

func TestDecodeWarningComesFirstInLenientMode(t *testing.T) {
	t.Parallel()

	const msg = "The content is UTF-16LE without a BOM."
	text := textenc.Decoded{Text: []byte("Hel\x01lo\n"), Encoding: "UTF-16LE", Warning: msg}
	_, info, err := read(stubRow, text, ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Equal(t, "UTF-16LE", info.SourceEncoding)
	assert.Equal(t, []ParseWarning{
		{Message: msg, Line: NoLine, Block: NoBlock, Action: Repaired},
		{Message: "The file has control characters, the first on line 1. The parser removed them.", Line: 1, Block: NoBlock, Action: Repaired},
	}, info.Warnings)

	_, info, err = read(stubRow, text, ReadOptions{})
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
		{name: "two cues", content: "Hello\nworld\n", cues: 2},
		{name: "strict removes a control character without a warning", content: "Hel\x01lo\n", cues: 1},
		{name: "lenient warns for a control character", content: "Hel\x01lo\n", lenient: true, cues: 1, warnings: 1},
		{name: "empty content gives an empty subtitle", content: ""},
		{name: "blank content keeps the warning", content: "\x01\n", lenient: true, warnings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := read(stubRow, utf8Text(tt.content), ReadOptions{Lenient: tt.lenient})
			require.NoError(t, err)
			assert.Equal(t, tt.cues, s.Len())
			assert.Equal(t, stubFormat, info.Format)
			assert.Equal(t, "UTF-8", info.SourceEncoding)
			assert.Len(t, info.Warnings, tt.warnings)
		})
	}
}

func TestReadErrorGivesNoSubtitleAndNoInfo(t *testing.T) {
	t.Parallel()

	s, info, err := read(stubRow, utf8Text("Hello\nThe fer\x00ry\n"), ReadOptions{})
	var perr *ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, 2, perr.Line)
	assert.Equal(t, stubFormat, perr.Format)
	assert.Nil(t, s)
	assert.Zero(t, info)
}

// Ports testEachParseCallStartsWithoutWarnings and testAStrictParseAfterALenientParseThrows
// of tests/Parsers/LenientParsingTest.php of subtitle-toolbox-php.
func TestEachReadStartsWithoutWarnings(t *testing.T) {
	t.Parallel()

	broken := utf8Text("Hello\nThe fer\x00ry\n")
	_, info, err := read(stubRow, broken, ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Equal(t, []ParseWarning{{
		Message: "The file has control characters, the first on line 2. The parser removed them.",
		Line:    2,
		Block:   NoBlock,
		Action:  Repaired,
	}}, info.Warnings)

	_, info, err = read(stubRow, utf8Text("Hello\n"), ReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Empty(t, info.Warnings)

	_, _, err = read(stubRow, broken, ReadOptions{})
	require.ErrorIs(t, err, ErrParse)
}

func TestReadPassesTheCommonOptions(t *testing.T) {
	t.Parallel()

	var got model.CommonReadOptions
	row := formatFuncs{format: stubFormat, read: func(_ io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
		got = common
		return &Subtitle{}, nil, nil
	}}
	last := new(7 * time.Second)
	_, _, err := read(row, utf8Text(""), ReadOptions{Lenient: true, LastCueDuration: last, Encoding: "UTF-8"})
	require.NoError(t, err)
	assert.Equal(t, model.CommonReadOptions{Lenient: true, LastCueDuration: last}, got)
}

// textReaders returns the stub row and every registry row with a reader. 1.x has text formats only.
func textReaders() []formatFuncs {
	rows := []formatFuncs{stubRow}
	for _, row := range registry {
		if row.read != nil {
			rows = append(rows, row)
		}
	}
	return rows
}

// Ports testEmptyContentGivesAnEmptySubtitleInBothModes of tests/Parsers/NoCuesTest.php of subtitle-toolbox-php.
func TestEmptyContentGivesAnEmptySubtitle(t *testing.T) {
	t.Parallel()

	contents := []struct{ name, text string }{{"empty", ""}, {"BOM and white space", utf8BOM + " \r\n\t\n"}}
	for _, row := range textReaders() {
		for _, content := range contents {
			for _, lenient := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s, %s, lenient %t", row.format, content.name, lenient), func(t *testing.T) {
					t.Parallel()

					s, info, err := read(row, utf8Text(content.text), ReadOptions{Format: row.format, Lenient: lenient})
					require.NoError(t, err)
					assert.Zero(t, s.Len())
					assert.Empty(t, info.Warnings)
					assert.Equal(t, row.format, info.Format)
				})
			}
		}
	}
}

// writesBOM lists the formats whose output starts with a UTF-8 BOM by default.
var writesBOM = map[Format]bool{
	stubFormat: true, ASS: true, CSV: true, TSV: true, LRC: true, MPSub: true, SubRip: true, WebVTT: true,
}

// textWriters returns the stub row and every registry row with a writer. 1.x has text formats only.
func textWriters() []formatFuncs {
	rows := []formatFuncs{stubRow}
	for _, row := range registry {
		if row.write != nil {
			rows = append(rows, row)
		}
	}
	return rows
}

func writeString(t *testing.T, row formatFuncs, s *Subtitle, opts WriteOptions) string {
	t.Helper()

	require.NoError(t, opts.Validate())
	var out bytes.Buffer
	require.NoError(t, row.write(&out, s, opts.common(), opts))
	return out.String()
}

func outputSubtitle(t *testing.T) *Subtitle {
	t.Helper()

	s := &Subtitle{}
	s.SetMetadata(MetadataTitle, "Cafe")
	s.AddCue(Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<i>Bonjour</i>", "ca va ?"}})
	s.AddCue(Cue{Start: 5 * time.Second, End: 7250 * time.Millisecond, Lines: []string{"Second cue"}})
	require.NoError(t, s.AddComment("Note", 1))
	return s
}

// Ports tests/Formatters/OutputOptionsTest.php of subtitle-toolbox-php for the text formats.
func TestOutputOptions(t *testing.T) {
	t.Parallel()

	for _, row := range textWriters() {
		t.Run(string(row.format), func(t *testing.T) {
			t.Parallel()

			s := outputSubtitle(t)
			def := writeString(t, row, s, WriteOptions{})
			assert.Equal(t, writesBOM[row.format], strings.HasPrefix(def, utf8BOM), "format BOM")
			assert.NotContains(t, def, "\r")
			assert.Equal(t, def, writeString(t, row, s, WriteOptions{LineEnding: LF}))

			plain := strings.TrimPrefix(def, utf8BOM)
			crlf := strings.ReplaceAll(def, "\n", "\r\n")
			assert.Equal(t, crlf, writeString(t, row, s, WriteOptions{LineEnding: CRLF}))
			assert.Equal(t, utf8BOM+plain, writeString(t, row, s, WriteOptions{BOM: BOMAdd}))
			assert.Equal(t, plain, writeString(t, row, s, WriteOptions{BOM: BOMRemove}))
			assert.Equal(t, utf8BOM+strings.ReplaceAll(plain, "\n", "\r\n"), writeString(t, row, s, WriteOptions{LineEnding: CRLF, BOM: BOMAdd}))
			assert.Equal(t, strings.ReplaceAll(plain, "\n", "\r\n"), writeString(t, row, s, WriteOptions{LineEnding: CRLF, BOM: BOMRemove}))
		})
	}
}

// Ports testANegativeTimeIsWrittenAsZero of tests/Formatters/NegativeCueTimesTest.php of subtitle-toolbox-php.
func TestNegativeTimesAreWrittenAsZero(t *testing.T) {
	t.Parallel()

	for _, row := range textWriters() {
		if row.format == JSON {
			continue
		}
		t.Run(string(row.format), func(t *testing.T) {
			t.Parallel()

			negative, zero := &Subtitle{}, &Subtitle{}
			negative.AddCues(Cue{Start: -2500 * time.Millisecond, End: -1500 * time.Millisecond, Lines: []string{"One"}}, Cue{Start: -500 * time.Millisecond, End: time.Second, Lines: []string{"Two"}})
			zero.AddCues(Cue{Lines: []string{"One"}}, Cue{End: time.Second, Lines: []string{"Two"}})
			assert.Equal(t, writeString(t, row, zero, WriteOptions{}), writeString(t, row, negative, WriteOptions{}))
		})
	}
}

// Ports testStripTagsChangesTheOutput of tests/Formatters/OutputOptionsTest.php of subtitle-toolbox-php.
func TestStripTagsChangesTheOutput(t *testing.T) {
	t.Parallel()

	for _, f := range []Format{ASS, MicroDVD, SAMI, SubRip, TTML, WebVTT} {
		row, ok := lookup(f)
		require.True(t, ok)
		if row.write == nil {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s := outputSubtitle(t)
			assert.NotEqual(t, writeString(t, row, s, WriteOptions{}), writeString(t, row, s, WriteOptions{StripTags: true}))
		})
	}
}
