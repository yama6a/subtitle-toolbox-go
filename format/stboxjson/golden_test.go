package stboxjson_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// realDir is the json folder of the read golden table of https://github.com/yama6a/subtitle-toolbox-go/issues/9.
const realDir = "json/real"

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
	require.Len(t, names, 6)
	return names
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

// TestWriteGoldens compares the output of each real file with its Go golden: compact as the round trip with the zero
// options, pretty against the input, and one file without format data.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	type goldenCase struct {
		file, suffix string
		opts         stboxjson.WriteOptions
		roundTrip    bool
	}
	names := realFiles(t)
	cases := make([]goldenCase, 0, 2*len(names)+1)
	for _, name := range names {
		cases = append(cases,
			goldenCase{name, ".out.json", stboxjson.WriteOptions{}, false},
			goldenCase{name, ".pretty.out.json", stboxjson.WriteOptions{PrettyPrint: true}, true})
	}
	cases = append(cases, goldenCase{"own_aegisub.json", ".omit-format-data.out.json", stboxjson.WriteOptions{PrettyPrint: true, OmitFormatData: true}, false})

	for _, c := range cases {
		t.Run(c.file+c.suffix, func(t *testing.T) {
			t.Parallel()

			input := testfiles.Path(t, realDir+"/"+c.file)
			got := write(t, readBytes(t, testfiles.Read(t, realDir+"/"+c.file)), c.opts)
			path := testfiles.Path(t, realDir+"/expected/"+c.file+c.suffix)
			if c.roundTrip {
				golden.RoundTrip(t, input, path, got, *update)
				return
			}
			golden.Equal(t, path, got, *update)
		})
	}
}

// Ports testEveryRealFileSurvivesTheArrayAndJsonRoundTrip of tests/ArrayConversionTest.php and
// testRealFileSurvivesAByteForByteRoundTrip of tests/Parsers/JsonRealFilesTest.php of subtitle-toolbox-php.
func TestRealFilesSurviveTheRoundTrip(t *testing.T) {
	t.Parallel()

	for _, name := range realFiles(t) {
		for _, opts := range []stboxjson.WriteOptions{{}, {PrettyPrint: true}} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				s := readBytes(t, testfiles.Read(t, realDir+"/"+name))
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

	content := testfiles.Read(t, realDir+"/own_aegisub.json")
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

			got := cueTexts(readBytes(t, testfiles.Read(t, realDir+"/"+tt.file)))
			require.Len(t, got, tt.cues)
			assert.Equal(t, tt.first, got[0])
			assert.Equal(t, tt.last, got[len(got)-1])
		})
	}

	s := readBytes(t, testfiles.Read(t, realDir+"/own_image_cues.json"))
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

	_, _, err := stboxjson.Read(bytes.NewReader(testfiles.Read(t, "json/own_scc_drop_frame_string.json")),
		model.CommonReadOptions{}, stboxjson.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The field formatData.scc.dropFrame must be a boolean.", perr.Msg)
}
