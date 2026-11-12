package csv_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

type realFile struct {
	name       string
	file       string
	opts       csv.ReadOptions
	lineEnding model.LineEnding
	bom        bool
}

var dubbingColumns = csv.Columns{Start: csv.ColumnNamed("Start TC"), Text: csv.ColumnNamed("Text"), Speaker: csv.ColumnNamed("Character")}

// realFiles holds the cases of the realFiles provider of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
// The PHP cases pass an empty layout, which turns the synonyms off. The files need none, so Go reads them with
// the zero layout. The read goldens check what each file reads as.
func realFiles() []realFile {
	return []realFile{
		{name: "mantas-done shape", file: "mantas_done_shape.csv"},
		{
			name: "Excel German locale", file: "excel_de_semicolon.csv",
			opts:       csv.ReadOptions{Columns: csv.Columns{End: csv.ColumnNamed("Ende"), Speaker: csv.ColumnNamed("Sprecher")}},
			lineEnding: model.CRLF, bom: true,
		},
		{
			name: "dubbing script", file: "dubbing_script.csv",
			opts: csv.ReadOptions{Columns: dubbingColumns, FrameRate: model.FrameRate{Num: 25, Den: 1}},
		},
		{name: "spreadsheet TSV", file: "sheets_export.tsv"},
		{name: "openai-whisper TSV", file: "own_openai_whisper.tsv"},
	}
}

func readReal(t *testing.T, file string, opts csv.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/"+file)), strict, opts)
	return s
}

// Ports testRealFileFormatsToItsOwnBytes of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileWritesItsOwnBytes(t *testing.T) {
	t.Parallel()

	for _, tt := range realFiles() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			common := model.CommonWriteOptions{LineEnding: tt.lineEnding, BOM: model.BOMRemove}
			if tt.bom {
				common.BOM = model.BOMAdd
			}
			want := string(testfiles.Read(t, realDir+"/"+tt.file))
			assert.Equal(t, want, write(t, readReal(t, tt.file, tt.opts), common, csv.WriteOptions{}))
		})
	}
}

// Ports testRealFileSurvivesARoundTripInTheDefaultLayout of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileRoundTripsInTheDefaultLayout(t *testing.T) {
	t.Parallel()

	for _, tt := range realFiles() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readReal(t, tt.file, tt.opts)
			s.DeleteFormatData("csv")
			back, _ := mustRead(t, write(t, s, model.CommonWriteOptions{}, csv.WriteOptions{}), strict, csv.ReadOptions{})
			assert.Equal(t, cueRows(s), cueRows(back))
			assert.Equal(t, ids(s), ids(back))
		})
	}
}

func ids(s *model.Subtitle) []string {
	var out []string
	for _, c := range s.All() {
		out = append(out, c.ID)
	}
	return out
}

// Ports testWhisperCppTableKeepsItsQuotedTextColumn of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileWhisperCppKeepsItsQuotedTextColumn(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, realDir+"/own_whisper_cpp.csv"))
	// whisper.cpp starts each text with a space, which the cue lines do not keep.
	assert.Equal(t, strings.ReplaceAll(content, `," `, `,"`), write(t, readReal(t, "own_whisper_cpp.csv", csv.ReadOptions{}), noBOM, csv.WriteOptions{}))
}
