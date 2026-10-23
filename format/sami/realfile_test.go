package sami_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testRealFileParses of tests/Parsers/SamiParserTest.php of subtitle-toolbox-php. The read goldens hold the
// Korean text, so the rows of the Korean files check only the times.
func TestRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		language    string
		first, last cueRow
	}{
		{
			"mantas_smi.smi", 2, "en-US",
			cueRow{137400 * ms, 140400 * ms, []string{"Passengers, we're now", "arriving at the central station."}},
			cueRow{3740500 * ms, 3742500 * ms, []string{"Thank you, conductor."}},
		},
		{
			"mantas_smi_formatted.smi", 3, "en-US",
			cueRow{9209 * ms, 12312 * ms, []string{"( bell ringing )"}},
			cueRow{17350 * ms, 22350 * ms, []string{"we watch the fields go by"}},
		},
		{"multi_language.smi", 3, "ko-KR", cueRow{time.Second, 4200 * ms, nil}, cueRow{8 * time.Second, 10500 * ms, nil}},
		{
			"pysubs2_source_id.smi", 9, "en-US-CC",
			cueRow{0, 10 * ms, []string{"Weather Desk"}},
			cueRow{73 * time.Second, 78 * time.Second, []string{"End of:", "Weather Report for Tuesday"}},
		},
		{"subsrt_sample.smi", 6, "kr-KR", cueRow{185113 * ms, 187035 * ms, nil}, cueRow{335453 * ms, 342460 * ms, nil}},
		{
			"own_windows_media.smi", 3, "en-US",
			cueRow{0, 3500 * ms, []string{"Announcer", "Good evening. Here is the station news."}},
			cueRow{8 * time.Second, 11 * time.Second, []string{"<b>Bus 7</b> runs every ten minutes."}},
		},
		{
			"own_subtitle_edit_modern.smi", 4, "en-US",
			cueRow{1200 * ms, 3800 * ms, []string{"The winter timetable starts on Monday."}},
			cueRow{10400 * ms, 13900 * ms, []string{`<font color="#ffff00">The last train</font> still leaves at 11:15.`}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s := readFile(t, tt.file, sami.ReadOptions{})
			require.Equal(t, tt.cues, s.Len())
			language, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, tt.language, language)
			rows := cueRows(s)
			first, last := rows[0], rows[len(rows)-1]
			if tt.first.Lines == nil {
				first.Lines, last.Lines = nil, nil
			}
			assert.Equal(t, tt.first, first)
			assert.Equal(t, tt.last, last)
		})
	}
}
