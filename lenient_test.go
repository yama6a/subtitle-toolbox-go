package stbox_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// lenientRow is the expected result of a lenient and of a strict read of one fixture in testdata/lenient.
type lenientRow struct {
	Name   string       `json:"name"`
	Format stbox.Format `json:"format"`
	Strict struct {
		Error string `json:"error"`
		Line  *int   `json:"line"`
		Cues  *int   `json:"cues"`
	} `json:"strict"`
	Cues []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"cues"`
	Warnings []struct {
		Line    *int                `json:"line"`
		Block   *int                `json:"block"`
		Action  stbox.WarningAction `json:"action"`
		Message string              `json:"message"`
	} `json:"warnings"`
}

type cueRow struct {
	Start, End time.Duration
	Text       string
}

type warningRow struct {
	Line, Block int
	Action      stbox.WarningAction
	Message     string
}

// Ports the damagedFiles contract of tests/Parsers/LenientParsingTest.php of subtitle-toolbox-php.
// Each file in testdata/lenient/expected holds the result of the fixture of the same name.
func TestLenientFixtures(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("testdata", "lenient", "expected", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	for _, path := range paths {
		file := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			row := readLenientRow(t, path)
			content, err := os.ReadFile(filepath.Join("testdata", "lenient", file))
			require.NoError(t, err)
			if !row.Format.CanRead() {
				t.Skipf("needs the %s reader", row.Format)
			}

			t.Run("lenient", func(t *testing.T) {
				t.Parallel()

				s, info, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: row.Format, Lenient: true})
				require.NoError(t, err)
				assert.Equal(t, row.wantCues(), cueRows(s))
				assert.Equal(t, row.wantWarnings(), warningRows(info.Warnings))
			})
			t.Run("strict", func(t *testing.T) {
				t.Parallel()

				s, info, err := stbox.Parse(bytes.NewReader(content), stbox.ReadOptions{Format: row.Format})
				if row.Strict.Cues != nil {
					require.NoError(t, err)
					assert.Equal(t, *row.Strict.Cues, s.Len())
					assert.Empty(t, info.Warnings)
					return
				}
				var perr *stbox.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, row.Strict.Error, perr.Msg)
				assert.Equal(t, row.strictLine(), perr.Line)
				assert.Equal(t, row.Format, perr.Format)
			})
		})
	}
}

func readLenientRow(t *testing.T, path string) lenientRow {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var row lenientRow
	require.NoError(t, dec.Decode(&row))
	require.NotEmpty(t, row.Name)
	require.NotEqual(t, row.Strict.Error == "", row.Strict.Cues == nil, "strict holds an error or a cue count")
	return row
}

func (r lenientRow) strictLine() int {
	if r.Strict.Line == nil {
		return 0
	}
	return *r.Strict.Line
}

func (r lenientRow) wantCues() []cueRow {
	out := make([]cueRow, 0, len(r.Cues))
	for _, c := range r.Cues {
		out = append(out, cueRow{Start: seconds(c.Start), End: seconds(c.End), Text: c.Text})
	}
	return out
}

func (r lenientRow) wantWarnings() []warningRow {
	out := make([]warningRow, 0, len(r.Warnings))
	for _, w := range r.Warnings {
		row := warningRow{Line: stbox.NoLine, Block: stbox.NoBlock, Action: w.Action, Message: w.Message}
		if w.Line != nil {
			row.Line = *w.Line
		}
		if w.Block != nil {
			row.Block = *w.Block
		}
		out = append(out, row)
	}
	return out
}

func seconds(s float64) time.Duration {
	return time.Duration(math.Round(s*1000)) * time.Millisecond
}

func cueRows(s *stbox.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{Start: c.Start, End: c.End, Text: c.Text()})
	}
	return out
}

func warningRows(warnings []stbox.ParseWarning) []warningRow {
	out := make([]warningRow, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, warningRow{Line: w.Line, Block: w.Block, Action: w.Action, Message: w.Message})
	}
	return out
}
