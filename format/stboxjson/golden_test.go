package stboxjson_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// realDir is the json folder of the read golden table of https://github.com/yama6a/subtitle-toolbox-go/issues/9.
const realDir = "../../testdata/json/real"

func realFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(realDir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			names = append(names, e.Name())
		}
	}
	require.Len(t, names, 6)
	return names
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func readBytes(t *testing.T, content []byte) *model.Subtitle {
	t.Helper()

	s, warnings, err := stboxjson.Read(bytes.NewReader(content), model.CommonReadOptions{}, stboxjson.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

func write(t *testing.T, s *model.Subtitle, opts stboxjson.WriteOptions) []byte {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, stboxjson.Write(&out, s, model.CommonWriteOptions{}, opts))
	return out.Bytes()
}

// canonical decodes JSON into any and indents it with sorted object keys, as the read golden runner of
// https://github.com/yama6a/subtitle-toolbox-go/issues/9 compares.
func canonical(t *testing.T, v any) string {
	t.Helper()

	b, err := json.MarshalIndent(v, "", "    ")
	require.NoError(t, err)
	return string(b)
}

// TestReadGoldens runs the json row of the read golden table. Each real file must read to its PHP golden.
func TestReadGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := stboxjson.Read(bytes.NewReader(readFile(t, filepath.Join(realDir, name))), model.CommonReadOptions{}, stboxjson.ReadOptions{})
			require.NoError(t, err)
			var subtitle any
			require.NoError(t, json.Unmarshal(write(t, s, stboxjson.WriteOptions{}), &subtitle))
			got := map[string]any{"warnings": goldenWarnings(warnings), "subtitle": subtitle}

			var want any
			require.NoError(t, json.Unmarshal(readFile(t, filepath.Join(realDir, "expected", name+".json")), &want))
			assert.Equal(t, canonical(t, want), canonical(t, got))
		})
	}
}

// goldenWarnings returns warnings in the JSON shape of the read goldens, with null for NoLine and NoBlock.
func goldenWarnings(warnings []model.ParseWarning) []any {
	out := make([]any, 0, len(warnings))
	for _, w := range warnings {
		var line, block any
		if w.Line != model.NoLine {
			line = float64(w.Line)
		}
		if w.Block != model.NoBlock {
			block = float64(w.Block)
		}
		lines := make([]any, len(w.Lines))
		for i, l := range w.Lines {
			lines[i] = l
		}
		out = append(out, map[string]any{"message": w.Message, "lineNumber": line, "blockIndex": block, "block": lines, "action": string(w.Action)})
	}
	return out
}

// TestWriteGoldens compares the output of each real file with its Go golden: compact as the round trip with the zero
// options, pretty against the input, and one file without format data.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	type golden struct {
		file, suffix string
		opts         stboxjson.WriteOptions
		roundTrip    bool
	}
	names := realFiles(t)
	cases := make([]golden, 0, 2*len(names)+1)
	for _, name := range names {
		cases = append(cases,
			golden{name, ".out.json", stboxjson.WriteOptions{}, false},
			golden{name, ".pretty.out.json", stboxjson.WriteOptions{PrettyPrint: true}, true})
	}
	cases = append(cases, golden{"own_aegisub.json", ".omit-format-data.out.json", stboxjson.WriteOptions{PrettyPrint: true, OmitFormatData: true}, false})

	for _, c := range cases {
		t.Run(c.file+c.suffix, func(t *testing.T) {
			t.Parallel()

			input := filepath.Join(realDir, c.file)
			got := write(t, readBytes(t, readFile(t, input)), c.opts)
			path := filepath.Join(realDir, "expected", c.file+c.suffix)
			if c.roundTrip {
				roundTripGolden(t, input, path, got)
				return
			}
			equalGolden(t, path, got)
		})
	}
}

// equalGolden compares got with the golden file at path. With -update, it writes got to the file first.
func equalGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	assert.Equal(t, string(readFile(t, path)), string(got))
}

// roundTripGolden compares got with the golden file when it exists, and with the input otherwise.
// With -update, it writes the golden file when got differs from the input, and deletes it when not.
func roundTripGolden(t *testing.T, input, path string, got []byte) {
	t.Helper()

	in := normalize(readFile(t, input))
	if *update {
		if bytes.Equal(in, got) {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				require.NoError(t, err)
			}
		} else {
			require.NoError(t, os.WriteFile(path, got, 0o600))
		}
	}
	want, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		want = in
	} else {
		require.NoError(t, err)
	}
	assert.Equal(t, string(want), string(got))
}

func normalize(b []byte) []byte {
	b = bytes.TrimPrefix(b, []byte("\ufeff"))
	return []byte(strings.ReplaceAll(strings.ReplaceAll(string(b), "\r\n", "\n"), "\r", "\n"))
}

// Ports testEveryRealFileSurvivesTheArrayAndJsonRoundTrip of tests/ArrayConversionTest.php and
// testRealFileSurvivesAByteForByteRoundTrip of tests/Parsers/JsonRealFilesTest.php of subtitle-toolbox-php.
func TestRealFilesSurviveTheRoundTrip(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		for _, opts := range []stboxjson.WriteOptions{{}, {PrettyPrint: true}} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				s := readBytes(t, readFile(t, filepath.Join(realDir, name)))
				out := write(t, s, opts)
				again := readBytes(t, out)
				assert.Equal(t, s, again)
				assert.Equal(t, string(out), string(write(t, again, opts)))
			})
		}
	}
}

// Ports testReadsTheCuesInStartOrderWithTheirComments of tests/Parsers/JsonParserTest.php of subtitle-toolbox-php.
func TestReadsReversedCuesInStartOrderWithTheirComments(t *testing.T) {
	t.Parallel()

	content := readFile(t, filepath.Join(realDir, "own_aegisub.json"))
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(content, &doc))
	var cues []json.RawMessage
	require.NoError(t, json.Unmarshal(doc["cues"], &cues))
	var comments []struct {
		Text           string `json:"text"`
		BeforeCueIndex int    `json:"beforeCueIndex"`
	}
	require.NoError(t, json.Unmarshal(doc["comments"], &comments))

	slices.Reverse(cues)
	for i, c := range comments {
		if c.BeforeCueIndex < len(cues) {
			comments[i].BeforeCueIndex = len(cues) - 1 - c.BeforeCueIndex
		}
	}
	var err error
	doc["cues"], err = json.Marshal(cues)
	require.NoError(t, err)
	doc["comments"], err = json.Marshal(comments)
	require.NoError(t, err)
	reversed, err := json.Marshal(doc)
	require.NoError(t, err)

	assert.Equal(t, readBytes(t, content), readBytes(t, reversed))
}

// Ports testRealFileParses and testImageCueFileHoldsThePngBytes of tests/Parsers/JsonRealFilesTest.php of subtitle-toolbox-php.
func TestRealFileParses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		cues        int
		first, last timedText
	}{
		{
			"own_aegisub.json", 7,
			timedText{ms(1000), ms(3500), "<v Conductor>Good morning, everyone."},
			timedText{ms(16_100), ms(18_000), "<v Passenger><u>Thank you</u>, and <s>good night</s> good day."},
		},
		{
			"own_alignment_and_coordinates.json", 10,
			timedText{ms(1000), ms(3000), "Plain first cue"},
			timedText{ms(19_500), ms(21_000), `Keep this: {\some_unknown_tag} and {normal text}`},
		},
		{"own_image_cues.json", 2, timedText{ms(1000), ms(2500), ""}, timedText{ms(3000), ms(4250), "Platform four"}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			got := cueTexts(readBytes(t, readFile(t, filepath.Join(realDir, tt.file))))
			require.Len(t, got, tt.cues)
			assert.Equal(t, tt.first, got[0])
			assert.Equal(t, tt.last, got[len(got)-1])
		})
	}

	s := readBytes(t, readFile(t, filepath.Join(realDir, "own_image_cues.json")))
	c, _ := s.Cue(0)
	image, ok := c.FormatData(model.ImageKey)
	require.True(t, ok)
	png, _ := image.String("png")
	assert.True(t, strings.HasPrefix(png, "\x89PNG\r\n\x1a\n"))
	keys := []string{"x", "y", "width", "height", "screenWidth", "screenHeight", "forced"}
	fields := make([]any, 0, len(keys))
	for _, key := range keys {
		v, _ := image.Get(key)
		fields = append(fields, v)
	}
	assert.Equal(t, []any{int64(640), int64(940), int64(2), int64(1), int64(1920), int64(1080), true}, fields)
}

func TestReadFailsForADropFrameString(t *testing.T) {
	t.Parallel()

	_, _, err := stboxjson.Read(bytes.NewReader(readFile(t, "../../testdata/json/own_scc_drop_frame_string.json")),
		model.CommonReadOptions{}, stboxjson.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The field formatData.scc.dropFrame must be a boolean.", perr.Msg)
}
