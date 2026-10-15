package webvtt_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

func realFiles(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(testfiles.Path(t, filepath.Join(realDir, "*.vtt")))
	require.NoError(t, err)
	require.Len(t, paths, 30)
	rels := make([]string, len(paths))
	for i, p := range paths {
		rels[i] = filepath.Join(realDir, filepath.Base(p))
	}
	return rels
}

func read(t *testing.T, content []byte, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _ := read(t, testfiles.Read(t, rel), false)
	return s
}

// TestReadGoldens compares the parse result of each file in vtt/real with the PHP read golden of the same name,
// as https://github.com/yama6a/subtitle-toolbox-go/issues/9 describes. The runner of #9 takes these cases over.
func TestReadGoldens(t *testing.T) {
	t.Parallel()

	for _, path := range realFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			s, warnings := read(t, testfiles.Read(t, path), false)
			var want any
			require.NoError(t, json.Unmarshal(testfiles.Read(t, filepath.Join(realDir, "expected", filepath.Base(path)+".json")), &want))

			assert.Equal(t, canonical(t, want), canonical(t, map[string]any{
				"warnings": warningsJSON(warnings),
				"subtitle": subtitleJSON(s),
			}))
		})
	}
}

// TestWriteGoldens writes each file in testdata/vtt/real back as WebVTT and compares the output with the input,
// or with its golden file when the output differs from the input.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	for _, path := range realFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			s := readFile(t, path)
			var out bytes.Buffer
			require.NoError(t, webvtt.Write(&out, s, model.CommonWriteOptions{}, webvtt.WriteOptions{}))
			golden.RoundTrip(t, testfiles.Path(t, path), testfiles.Path(t, filepath.Join(realDir, "expected", filepath.Base(path)+".out.vtt")), out.Bytes(), *update)
		})
	}
}

func canonical(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(b, &decoded))
	out, err := json.MarshalIndent(decoded, "", "  ")
	require.NoError(t, err)
	return string(out)
}

func warningsJSON(warnings []model.ParseWarning) []any {
	out := make([]any, 0, len(warnings))
	for _, w := range warnings {
		var line, block any
		if w.Line != model.NoLine {
			line = w.Line
		}
		if w.Block != model.NoBlock {
			block = w.Block
		}
		lines := w.Lines
		if lines == nil {
			lines = []string{}
		}
		out = append(out, map[string]any{"message": w.Message, "lineNumber": line, "blockIndex": block, "block": lines, "action": string(w.Action)})
	}
	return out
}

// subtitleJSON builds the library JSON of version 1 that the read goldens hold.
func subtitleJSON(s *model.Subtitle) map[string]any {
	comments := make([]any, 0, len(s.Comments()))
	for _, c := range s.Comments() {
		comments = append(comments, map[string]any{"text": c.Text, "beforeCueIndex": c.BeforeCue})
	}
	cues := []any{}
	for _, c := range s.All() {
		lines := c.Lines
		if lines == nil {
			lines = []string{}
		}
		var id, alignment any
		if c.ID != "" {
			id = c.ID
		}
		if c.Alignment != model.BottomCenter {
			alignment = c.Alignment.Numpad()
		}
		cues = append(cues, map[string]any{
			"start":      float64(c.Start.Milliseconds()) / 1000,
			"end":        float64(c.End.Milliseconds()) / 1000,
			"lines":      lines,
			"identifier": id,
			"alignment":  alignment,
			"formatData": formatDataJSON(c.FormatDataKeys(), c.FormatData),
		})
	}
	metadata := map[string]any{}
	for k, v := range s.AllMetadata() {
		metadata[k] = v
	}
	return map[string]any{
		"version":    1,
		"metadata":   metadata,
		"comments":   comments,
		"formatData": formatDataJSON(s.FormatDataKeys(), s.FormatData),
		"cues":       cues,
	}
}

func formatDataJSON(keys []string, get func(string) (*model.Data, bool)) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		d, _ := get(k)
		out[k] = valueJSON(d)
	}
	return out
}

func valueJSON(v any) any {
	switch v := v.(type) {
	case *model.Data:
		out := map[string]any{}
		for _, k := range v.Keys() {
			item, _ := v.Get(k)
			out[k] = valueJSON(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = valueJSON(item)
		}
		return out
	default:
		return v
	}
}
