package stbox_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/format/sami"
)

// Ports testRealFilesGiveTheCuesOf1x of tests/ReadOptionsTest.php of subtitle-toolbox-php.
// Each golden holds [start, end, lines] rows that PHP 1.x read from the file with the options.
func TestReadOptionGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		golden string
		file   string
		opts   stbox.ReadOptions
	}{
		{
			golden: "french_windows_1252",
			file:   "encoding/french-windows-1252.srt",
			opts:   stbox.ReadOptions{Format: stbox.SubRip, Encoding: "Windows-1252"},
		},
		{
			golden: "sami_multi_language_frcc",
			file:   "sami/real/multi_language.smi",
			opts: stbox.ReadOptions{
				Format: stbox.SAMI, LastCueDuration: new(10 * time.Second), SAMI: sami.ReadOptions{LanguageClass: "FRCC"},
			},
		},
		{
			golden: "microdvd_subsrt_sample",
			file:   "microdvd/real/subsrt_sample.sub",
			opts:   stbox.ReadOptions{Format: stbox.MicroDVD, MicroDVD: microdvd.ReadOptions{FrameRate: 23.976}},
		},
		{
			golden: "csv_excel_de_semicolon",
			file:   "csv/real/excel_de_semicolon.csv",
			opts: stbox.ReadOptions{
				Format: stbox.CSV, LastCueDuration: new(10 * time.Second), CSV: csv.ReadOptions{Delimiter: csv.Semicolon},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", filepath.FromSlash(tt.file))
			data, err := os.ReadFile(filepath.Join("testdata", "read-options", tt.golden+".json"))
			require.NoError(t, err)
			var want any
			require.NoError(t, json.Unmarshal(data, &want))

			s, _, err := stbox.Load(path, tt.opts)
			require.NoError(t, err)
			var rows []any
			for _, c := range s.All() {
				lines := make([]any, len(c.Lines))
				for i, line := range c.Lines {
					lines[i] = line
				}
				rows = append(rows, []any{c.Start.Seconds(), c.End.Seconds(), lines})
			}
			assert.Equal(t, want, any(rows))
		})
	}
}

// Ports testALastCueWithoutEndLastsFiveSecondsByDefault and testALastCueDurationOf0KeepsTheLastCue of
// tests/ReadOptionsTest.php of subtitle-toolbox-php. Each reader that uses the default adds its row.
func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		opts  stbox.ReadOptions
		start time.Duration
	}{
		{"LRC", "lrc/real/handwritten-core.lrc", stbox.ReadOptions{Format: stbox.LRC}, 32800 * time.Millisecond},
		{"TMPlayer", "tmplayer/real/tmplayer_crlf.txt", stbox.ReadOptions{Format: stbox.TMPlayer}, 3600 * time.Second},
		{"SAMI", "sami/real/mantas_smi_formatted.smi", stbox.ReadOptions{Format: stbox.SAMI}, 17350 * time.Millisecond},
		{"CSV", "csv/real/dubbing_script.csv", stbox.ReadOptions{Format: stbox.CSV, CSV: dubbingScript}, 36008200 * time.Millisecond},
		{"SubViewer 1", "subviewer/real/subviewer1_delay.sub", stbox.ReadOptions{Format: stbox.SubViewer}, 17 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", filepath.FromSlash(tt.file))
			for _, c := range []struct {
				last *time.Duration
				want time.Duration
			}{{nil, 5 * time.Second}, {new(7 * time.Second), 7 * time.Second}, {new(time.Duration(0)), 0}} {
				opts := tt.opts
				opts.LastCueDuration = c.last
				s, _, err := stbox.Load(path, opts)
				require.NoError(t, err)
				last, _ := s.Cue(s.Len() - 1)
				assert.Equal(t, [2]time.Duration{tt.start, tt.start + c.want}, [2]time.Duration{last.Start, last.End})
			}
		})
	}
}
