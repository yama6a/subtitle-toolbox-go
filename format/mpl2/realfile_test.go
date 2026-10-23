package mpl2_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/yama6a/subtitle-toolbox-go/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileParses and testRealFileSurvivesARoundTrip of tests/Parsers/Mpl2ParserTest.php of
// subtitle-toolbox-php.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file     string
		encoding string
		cues     int
		first    cueRow
		last     cueRow
		// crlf is true when the file is the CR LF output of the writer, so a round trip is byte-identical.
		crlf bool
	}{
		{
			"mpl2/napiprojekt_cp1250.txt", "Windows-1250", 8,
			cueRow{1200 * ms, 4500 * ms, []string{"Dzie\u0144 dobry, piekarnia jest ju\u017c otwarta."}},
			cueRow{360 * time.Second, 364500 * ms, []string{"Do zobaczenia w poniedzia\u0142ek!"}},
			true,
		},
		{
			"mpl2/real/pysubs2_shape_bom.txt", "", 6,
			cueRow{0, 2500 * ms, []string{"The bakery opens at six."}},
			cueRow{3600 * time.Second, 3604200 * ms, []string{"Closed on Sundays."}},
			false,
		},
		{
			"mpl2/real/subtitle_edit_shape.txt", "", 5,
			cueRow{1500 * ms, 4 * time.Second, []string{"The ferry leaves at nine."}},
			cueRow{18 * time.Second, 22 * time.Second, []string{"Arrival in two hours."}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			raw := testfiles.Read(t, tt.file)
			decoded, err := textenc.Decode(raw, tt.encoding)
			require.NoError(t, err)
			s, warnings, err := mpl2.Read(bytes.NewReader(decoded.Text), model.CommonReadOptions{}, mpl2.ReadOptions{})
			require.NoError(t, err)
			require.Empty(t, warnings)
			require.Equal(t, tt.cues, s.Len())
			rows := cueRows(s)
			assert.Equal(t, tt.first, rows[0])
			assert.Equal(t, tt.last, rows[len(rows)-1])

			written := write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF})
			back, _ := mustRead(t, written, false)
			assert.Equal(t, rows, cueRows(back))
			if tt.crlf {
				encoded, err := charmap.Windows1250.NewEncoder().Bytes([]byte(written))
				require.NoError(t, err)
				assert.Equal(t, raw, encoded)
			}
		})
	}
}
