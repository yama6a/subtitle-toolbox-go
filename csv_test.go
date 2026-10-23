package stbox_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// Ports the TSV and CSV rows of testLoadWithoutFormatReadsTheFormatOfTheContentOrTheExtension of tests/LoadSaveTest.php
// of subtitle-toolbox-php: a table has no signature, so Load takes the format from the extension.
func TestLoadCSVAndTSVByExtension(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		file string
		want stbox.Format
	}{
		{"csv/real/sheets_export.tsv", stbox.TSV},
		{"csv/real/excel_de_semicolon.csv", stbox.CSV},
	} {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(testfiles.Path(t, tt.file), stbox.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, info.Format)
			explicit, _, err := stbox.Parse(bytes.NewReader(testfiles.Read(t, tt.file)), stbox.ReadOptions{Format: tt.want})
			require.NoError(t, err)
			assert.Equal(t, explicit, s)
		})
	}
}

// Ports testReturnsNullForFrameTimecodeFixtures of tests/FormatDetectorTest.php and the CSV row of
// testDetectionFindsTheFormatAfterTwoBoms of tests/Parsers/RepeatedBomTest.php of subtitle-toolbox-php.
func TestParseDoesNotDetectCSV(t *testing.T) {
	t.Parallel()

	for _, content := range [][]byte{
		testfiles.Read(t, "csv/real/headerless_frame_times.csv"),
		testfiles.Read(t, "csv/real/mantas_done_shape.csv"),
		[]byte("\ufeff\ufeffstart,end,text\n00:00:01.000,00:00:02.000,The ferry leaves.\n00:00:03.000,00:00:04.000,It is late.\n"),
	} {
		_, ok := stbox.Detect(content)
		assert.False(t, ok)
		_, _, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{})
		require.ErrorIs(t, err, stbox.ErrUnknownFormat)
	}
}

// Ports testSaveSetsTheCsvDelimiterFromTheTarfindFormat of tests/LoadSaveTest.php of subtitle-toolbox-php: a TSV read
// written as CSV without CSV.Delimiter gets commas, and TSV output gets tabs.
func TestTSVWrittenAsCSV(t *testing.T) {
	t.Parallel()

	tsv, _, err := stbox.Load(testfiles.Path(t, "csv/real/sheets_export.tsv"), stbox.ReadOptions{})
	require.NoError(t, err)
	asCSV := writeAs(t, tsv, stbox.CSV)
	first, _, _ := strings.Cut(string(asCSV), "\n")
	assert.Contains(t, first, ",")
	assert.NotContains(t, string(asCSV), "\t")
	var comma bytes.Buffer
	require.NoError(t, stbox.Write(&comma, tsv, stbox.CSV, stbox.WriteOptions{CSV: csv.WriteOptions{Delimiter: csv.Comma}}))
	assert.Equal(t, comma.String(), string(asCSV))
	first, _, _ = strings.Cut(string(writeAs(t, tsv, stbox.TSV)), "\n")
	assert.Contains(t, first, "\t")

	semicolons, _, err := stbox.Load(testfiles.Path(t, "csv/real/excel_de_semicolon.csv"), stbox.ReadOptions{})
	require.NoError(t, err)
	first, _, _ = strings.Cut(string(writeAs(t, semicolons, stbox.CSV)), "\n")
	assert.Contains(t, first, ";")
	first, _, _ = strings.Cut(string(writeAs(t, semicolons, stbox.TSV)), "\n")
	assert.Contains(t, first, "\t")

	var semicolon bytes.Buffer
	require.NoError(t, stbox.Write(&semicolon, tsv, stbox.CSV, stbox.WriteOptions{CSV: csv.WriteOptions{Delimiter: csv.Semicolon}}))
	first, _, _ = strings.Cut(semicolon.String(), "\n")
	assert.Contains(t, first, ";")
}

// The .tsv cases of testRealFileFormatsToItsOwnBytes of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php,
// written as TSV.
func TestTSVRealFilesWriteTheirOwnBytes(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"csv/real/sheets_export.tsv", "csv/real/own_openai_whisper.tsv"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, file), stbox.ReadOptions{})
			require.NoError(t, err)
			var out bytes.Buffer
			require.NoError(t, stbox.Write(&out, s, stbox.TSV, stbox.WriteOptions{BOM: stbox.BOMRemove}))
			assert.Equal(t, string(testfiles.Read(t, file)), out.String())
		})
	}
}

func TestTSVReadErrorsNameTSV(t *testing.T) {
	t.Parallel()

	_, _, err := stbox.Parse(strings.NewReader("start\tend\n1\t2\n"), stbox.ReadOptions{Format: stbox.TSV})
	var perr *stbox.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, &stbox.ParseError{Format: stbox.TSV, Line: 1, Msg: `The table has no column "text" for text.`}, perr)
}
