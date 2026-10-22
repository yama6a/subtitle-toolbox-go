package stbox_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// goldenFolder is a row of the read golden table of https://github.com/yama6a/subtitle-toolbox-go/issues/9.
type goldenFolder struct {
	name   string
	format stbox.Format
	files  int
	// options holds the read options of single files by file name. Every other file reads with the zero options.
	options map[string]stbox.ReadOptions
}

// microDVDAt25 is the read option of the MicroDVD read goldens, as tests/files/generate-read-goldens.php sets it.
var microDVDAt25 = stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: 25}}

func goldenFolders() []goldenFolder {
	return []goldenFolder{
		{name: "ass", format: stbox.ASS, files: 7},
		{name: "json", format: stbox.JSON, files: 6},
		{name: "microdvd", format: stbox.MicroDVD, files: 4, options: map[string]stbox.ReadOptions{
			"own_max_frames.sub":           microDVDAt25,
			"sub_microdvd.sub":             microDVDAt25,
			"sub_microdvd_with_styles.sub": microDVDAt25,
			"subsrt_sample.sub":            microDVDAt25,
		}},
		{
			name: "sami", format: stbox.SAMI, files: 11,
			options: map[string]stbox.ReadOptions{"own_korean_cp949.smi": {Encoding: "CP949"}},
		},
		{name: "srt", format: stbox.SubRip, files: 19},
		{name: "ttml", format: stbox.TTML, files: 29},
		{name: "vtt", format: stbox.WebVTT, files: 30},
	}
}

// realFiles returns the file names under <folder>/real, without SOURCES.md and the expected folder.
func (g goldenFolder) realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, g.name+"/real"))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	require.Len(t, names, g.files)
	return names
}

func (g goldenFolder) read(t *testing.T, file string) (*stbox.Subtitle, stbox.ReadInfo) {
	t.Helper()

	return g.readWith(t, file, g.options[file])
}

func (g goldenFolder) readWith(t *testing.T, file string, opts stbox.ReadOptions) (*stbox.Subtitle, stbox.ReadInfo) {
	t.Helper()

	opts.Format = g.format
	s, info, err := stbox.Parse(bytes.NewReader(testfiles.Read(t, g.name+"/real/"+file)), opts)
	require.NoError(t, err)
	return s, info
}

// TestReadGoldens reads each real file and compares the warnings and the library JSON of the result with the PHP
// read golden expected/<file>.json. It never writes a golden file, because the PHP goldens are the oracle.
func TestReadGoldens(t *testing.T) {
	t.Parallel()

	for _, g := range goldenFolders() {
		for _, file := range g.realFiles(t) {
			t.Run(g.name+"/"+file, func(t *testing.T) {
				t.Parallel()

				s, info := g.read(t, file)
				var library bytes.Buffer
				require.NoError(t, stbox.Write(&library, s, stbox.JSON, stbox.WriteOptions{}))
				var subtitle any
				require.NoError(t, json.Unmarshal(library.Bytes(), &subtitle))
				got := map[string]any{"warnings": goldenWarnings(info.Warnings), "subtitle": subtitle}

				var want any
				require.NoError(t, json.Unmarshal(testfiles.Read(t, g.name+"/real/expected/"+file+".json"), &want))
				assert.Equal(t, canonicalJSON(t, want), canonicalJSON(t, got))
			})
		}
	}
}

func TestEveryReadGoldenHasARealFile(t *testing.T) {
	t.Parallel()

	for _, g := range goldenFolders() {
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()

			paths, err := filepath.Glob(testfiles.Path(t, g.name+"/real/expected/*.json"))
			require.NoError(t, err)
			var got []string
			for _, p := range paths {
				name := filepath.Base(p)
				if !strings.Contains(name, ".out.") && !strings.Contains(name, ".to.") {
					got = append(got, name)
				}
			}
			files := g.realFiles(t)
			want := make([]string, len(files))
			for i, file := range files {
				want[i] = file + ".json"
			}
			assert.ElementsMatch(t, want, got)
		})
	}
}

// goldenWarnings returns warnings in the JSON shape of the read goldens, with null for NoLine and NoBlock.
func goldenWarnings(warnings []stbox.ParseWarning) []any {
	out := make([]any, 0, len(warnings))
	for _, w := range warnings {
		var line, block any
		if w.Line != stbox.NoLine {
			line = w.Line
		}
		if w.Block != stbox.NoBlock {
			block = w.Block
		}
		out = append(out, map[string]any{
			"message":    w.Message,
			"lineNumber": line,
			"blockIndex": block,
			"block":      append([]string{}, w.Lines...),
			"action":     string(w.Action),
		})
	}
	return out
}

// canonicalJSON encodes v with sorted object keys and one line per value, after a JSON round trip that turns every
// number into a float64. So key order and number spelling such as 4.0 against 4 do not count.
func canonicalJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(b, &decoded))
	b, err = json.MarshalIndent(decoded, "", "    ")
	require.NoError(t, err)
	return string(b)
}

func goldenPath(g goldenFolder, file, suffix string) string {
	return path.Join(g.name, "real", "expected", file+suffix)
}
