package mpl2_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports testRealFileParses and testRealFileSurvivesARoundTrip of tests/Parsers/Mpl2ParserTest.php of
// subtitle-toolbox-php. The read goldens check the files under real/. Only the Windows-1250 file has no golden,
// so only its row sets the cue count and the first and last cue.
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
		{"mpl2/real/pysubs2_shape_bom.txt", "", 0, cueRow{}, cueRow{}, false},
		{"mpl2/real/subtitle_edit_shape.txt", "", 0, cueRow{}, cueRow{}, false},
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
			rows := cueRows(s)
			if tt.cues > 0 {
				require.Equal(t, tt.cues, s.Len())
				assert.Equal(t, tt.first, rows[0])
				assert.Equal(t, tt.last, rows[len(rows)-1])
			}

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
