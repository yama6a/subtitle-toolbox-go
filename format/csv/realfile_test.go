package csv_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

type realFile struct {
	name       string
	file       string
	opts       csv.ReadOptions
	cues       int
	first      cueRow
	last       cueRow
	lineEnding model.LineEnding
	bom        bool
}

var dubbingColumns = csv.Columns{Start: csv.ColumnNamed("Start TC"), Text: csv.ColumnNamed("Text"), Speaker: csv.ColumnNamed("Character")}

// realFiles holds the cases of the realFiles provider of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
// The PHP cases pass an empty layout, which turns the synonyms off. The files need none, so Go reads them with
// the zero layout.
func realFiles() []realFile {
	return []realFile{
		{
			name: "mantas-done shape", file: "mantas_done_shape.csv", cues: 5,
			first: cueRow{1500 * ms, 4 * time.Second, []string{"The bakery opens at seven, every day."}},
			last:  cueRow{16 * time.Second, 18200 * ms, []string{"Please queue at the door, not inside."}},
		},
		{
			name: "Excel German locale", file: "excel_de_semicolon.csv", cues: 4,
			opts:       csv.ReadOptions{Columns: csv.Columns{End: csv.ColumnNamed("Ende"), Speaker: csv.ColumnNamed("Sprecher")}},
			first:      cueRow{2 * time.Second, 4500 * ms, []string{"<v Lena>Der Zug nach Basel f\u00e4hrt um 7:15 Uhr."}},
			last:       cueRow{12 * time.Second, 14640 * ms, []string{"<v Lena>Danke; bis morgen!"}},
			lineEnding: model.CRLF, bom: true,
		},
		{
			name: "dubbing script", file: "dubbing_script.csv", cues: 4,
			opts:  csv.ReadOptions{Columns: dubbingColumns, FrameRate: 25},
			first: cueRow{36001480 * ms, 36004 * time.Second, []string{"<v NARRATOR>The weather turns cold tonight."}},
			last:  cueRow{36008200 * ms, 36013200 * ms, []string{"<v NARRATOR>Snow falls in the hills, rain in the valley."}},
		},
		{
			name: "spreadsheet TSV", file: "sheets_export.tsv", cues: 4,
			first: cueRow{500 * ms, 3 * time.Second, []string{"Welcome to the station."}},
			last:  cueRow{10 * time.Second, 12 * time.Second, []string{"Have a safe trip."}},
		},
		{
			name: "openai-whisper TSV", file: "own_openai_whisper.tsv", cues: 6,
			first: cueRow{0, 2340 * ms, []string{"Good morning and welcome to the harbour tour."}},
			last:  cueRow{17020 * ms, 19300 * ms, []string{"Thank you, and enjoy the trip!"}},
		},
	}
}

func readReal(t *testing.T, file string, opts csv.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _ := mustRead(t, string(testfiles.Read(t, realDir+"/"+file)), strict, opts)
	return s
}

// Ports testRealFileParses of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileParses(t *testing.T) {
	t.Parallel()

	for _, tt := range realFiles() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rows := cueRows(readReal(t, tt.file, tt.opts))
			require.Len(t, rows, tt.cues)
			assert.Equal(t, tt.first, rows[0])
			assert.Equal(t, tt.last, rows[len(rows)-1])
		})
	}
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

// Ports testDubbingScriptKeepsTheNotesColumn of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileDubbingScriptKeepsTheNotesColumn(t *testing.T) {
	t.Parallel()

	s := readReal(t, "dubbing_script.csv", csv.ReadOptions{Columns: dubbingColumns, FrameRate: 25})
	c, _ := s.Cue(1)
	d, _ := c.FormatData("csv")
	assert.Equal(t, model.NewData(model.Field{Key: "columns", Value: model.NewData(model.Field{Key: "Notes", Value: "warm tone"})}), d)
	c, _ = s.Cue(2)
	assert.Equal(t, 36005800*ms, c.Start)
}

// Ports testTsvKeepsTheIdentifiers of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileTSVKeepsTheIdentifiers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"intro", "platform", "", "end"}, ids(readReal(t, "sheets_export.tsv", csv.ReadOptions{})))
}
