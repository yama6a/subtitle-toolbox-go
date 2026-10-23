package qttext_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileParses and testRealFileRoundTrips of tests/Parsers/QuickTimeTextParserTest.php of
// subtitle-toolbox-php. The CR LF of the Subtitle Edit file stays in the output with the CRLF option.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last cueRow
		write       model.CommonWriteOptions
	}{
		{
			"subtitle_edit_shape.txt", 3,
			cueRow{2500 * ms, 5 * time.Second, "The museum opens at ten,\nthe café at nine."},
			cueRow{3600 * time.Second, 3603333 * ms, "Closing time."},
			model.CommonWriteOptions{LineEnding: model.CRLF},
		},
		{
			"quicktime_export_styles.txt", 4,
			cueRow{1500 * ms, 4 * time.Second, "The museum opens at ten."},
			cueRow{9750 * ms, 12 * time.Second, "Who is there? <b>Hello?</b>"},
			model.CommonWriteOptions{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+tt.file)
			require.Equal(t, tt.cues, s.Len())
			rows := cueRows(s)
			assert.Equal(t, tt.first, rows[0])
			assert.Equal(t, tt.last, rows[len(rows)-1])
			assert.Equal(t, string(testfiles.Read(t, realDir+"/"+tt.file)), write(t, s, tt.write))
		})
	}
}
