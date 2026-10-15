package stbox_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// Ports testRealFilesGiveTheCuesOf1x of tests/ReadOptionsTest.php of subtitle-toolbox-php.
// Each golden holds [start, end, lines] rows that PHP 1.x read from the file with the options.
func TestReadOptionGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		golden string
		file   string
		opts   stbox.ReadOptions
		needs  string
	}{
		{
			golden: "french_windows_1252",
			file:   "encoding/french-windows-1252.srt",
			opts:   stbox.ReadOptions{Format: stbox.SubRip, Encoding: "Windows-1252"},
			needs:  "the Windows-1252 decoder and its fixture of #16 and the SubRip reader of #20",
		},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join("testdata", filepath.FromSlash(tt.file))
			if _, err := os.Stat(path); err != nil || !tt.opts.Format.CanRead() {
				t.Skip("needs " + tt.needs)
			}
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
