package qttext_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// Ports testRealFileRoundTrips of tests/Parsers/QuickTimeTextParserTest.php of subtitle-toolbox-php. The read
// goldens check what each file reads as. The CR LF of the Subtitle Edit file stays in the output with the CRLF option.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file  string
		write model.CommonWriteOptions
	}{
		{"subtitle_edit_shape.txt", model.CommonWriteOptions{LineEnding: model.CRLF}},
		{"quicktime_export_styles.txt", model.CommonWriteOptions{}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, realDir+"/"+tt.file)
			assert.Equal(t, string(testfiles.Read(t, realDir+"/"+tt.file)), write(t, s, tt.write))
		})
	}
}
