package srt_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// realDir is the srt folder of the read golden table of https://github.com/yama6a/subtitle-toolbox-go/issues/9.
const realDir = "srt/real"

func realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, realDir))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	require.Len(t, names, 19)
	return names
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := srt.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

// TestReadGoldens runs the srt row of the read golden table: each real file must read to its PHP golden.
// The read golden runner of https://github.com/yama6a/subtitle-toolbox-go/issues/9 takes it over with the
// library JSON writer. Until then, libraryJSON builds the same JSON from the fields that SubRip reads.
func TestReadGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			content := testfiles.Read(t, realDir+"/"+name)
			s, warnings, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
			require.NoError(t, err)
			got := map[string]any{"warnings": goldenWarnings(warnings), "subtitle": libraryJSON(s)}

			var want any
			require.NoError(t, json.Unmarshal(testfiles.Read(t, realDir+"/expected/"+name+".json"), &want))
			assert.Equal(t, canonical(t, want), canonical(t, got))
		})
	}
}

func TestEveryReadGoldenHasARealFile(t *testing.T) {
	t.Parallel()

	goldens, err := filepath.Glob(testfiles.Path(t, realDir+"/expected/*.json"))
	require.NoError(t, err)
	var got []string
	for _, g := range goldens {
		base := filepath.Base(g)
		if !strings.Contains(base, ".out.") && !strings.Contains(base, ".to.") {
			got = append(got, strings.TrimSuffix(base, ".json"))
		}
	}
	assert.ElementsMatch(t, realFiles(t), got)
}

// TestWriteGoldens writes each real file back as SubRip with the zero options. The output equals the normalised
// input, or the Go golden expected/<file>.out.srt.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			require.NoError(t, srt.Write(&out, readFile(t, realDir+"/"+name), model.CommonWriteOptions{}, srt.WriteOptions{}))
			golden.RoundTrip(t, testfiles.Path(t, realDir+"/"+name), testfiles.Path(t, realDir+"/expected/"+name+".out.srt"), out.Bytes(), *update)
		})
	}
}

func canonical(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(b, &decoded))
	b, err = json.MarshalIndent(decoded, "", "    ")
	require.NoError(t, err)
	return string(b)
}

// goldenWarnings returns warnings in the JSON shape of the read goldens, with null for NoLine and NoBlock.
func goldenWarnings(warnings []model.ParseWarning) []any {
	out := make([]any, 0, len(warnings))
	for _, w := range warnings {
		var line, block any
		if w.Line != model.NoLine {
			line = w.Line
		}
		if w.Block != model.NoBlock {
			block = w.Block
		}
		out = append(out, map[string]any{"message": w.Message, "lineNumber": line, "blockIndex": block, "block": append([]string{}, w.Lines...), "action": w.Action})
	}
	return out
}

// libraryJSON returns s in the shape of the library JSON version 1, for a subtitle without comments, metadata,
// file format data, identifiers and forced cues, as SubRip reads it.
func libraryJSON(s *model.Subtitle) map[string]any {
	cues := []any{}
	for _, c := range s.All() {
		var alignment any
		if c.Alignment != model.BottomCenter {
			alignment = c.Alignment.Numpad()
		}
		formatData := map[string]any{}
		for _, k := range c.FormatDataKeys() {
			d, _ := c.FormatData(k)
			formatData[k] = dataJSON(d)
		}
		cues = append(cues, map[string]any{
			"start":      float64(c.Start.Milliseconds()) / 1000,
			"end":        float64(c.End.Milliseconds()) / 1000,
			"lines":      append([]string{}, c.Lines...),
			"identifier": nil,
			"alignment":  alignment,
			"formatData": formatData,
		})
	}
	return map[string]any{
		"version":    1,
		"metadata":   map[string]any{},
		"comments":   []any{},
		"formatData": map[string]any{},
		"cues":       cues,
	}
}

func dataJSON(d *model.Data) map[string]any {
	out := map[string]any{}
	for _, k := range d.Keys() {
		v, _ := d.Get(k)
		if o, ok := v.(*model.Data); ok {
			v = dataJSON(o)
		}
		out[k] = v
	}
	return out
}
