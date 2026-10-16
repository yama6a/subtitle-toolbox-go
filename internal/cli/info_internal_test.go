package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const libraryJSONWithMetadata = `{"version":1,"metadata":{"zulu":"last","title":"A <b> & \"c\"","author":"Anna"},` +
	`"cues":[{"start":1,"end":2,"lines":["Hi"]}]}`

const infoHelp = `Usage: stbox info <input>... [--json] [options]

Print the format, the encoding and the cue count of subtitle files.

Options:
  --json           Print JSON: a list with one object for each input file.
  --from FORMAT    Input format. Default: the format detected from the content.
                   When detection finds no format, the file extension sets it.
  --encoding NAME  Encoding of input that is not UTF-8, for example
                   Windows-1252. It replaces code page detection. A BOM, valid
                   UTF-8 and UTF-16 input win over it.
  --lenient        Skip or repair broken cues and print a warning for each.
  --keep-going     Go on with the next file after a file fails. Default: stop at
                   the first failure.
  -h, --help       Show this help.
`

func TestInfoHelp(t *testing.T) {
	t.Parallel()

	want := result{Stdout: infoHelp}
	assert.Equal(t, want, call{}.run(t, "info", "--help"))
	assert.Equal(t, want, call{}.run(t, "info", "-h"))
	assert.Equal(t, want, call{}.run(t, "help", "info"))
}

func TestInfoText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, result{Stdout: "trip.srt\n  Format:   srt\n  Encoding: UTF-8\n  Cues:     3\n"}, runCLI(t, "info", "trip.srt"))
}

func TestInfoGolden(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		words []string
	}{
		{"info_trip.txt", []string{"info", "trip.srt"}},
		{"info_trip.json", []string{"info", "trip.srt", "--json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := runCLI(t, tt.words...)
			assert.Equal(t, 0, got.Code)
			assert.Empty(t, got.Stderr)
			golden.Equal(t, testfiles.Path(t, "cli/expected/"+tt.name), []byte(got.Stdout), *update)
		})
	}
}

func TestInfoJSON(t *testing.T) {
	t.Parallel()

	want := `[
    {
        "file": "trip.srt",
        "format": "srt",
        "encoding": "UTF-8",
        "metadata": {},
        "warnings": []
    }
]
`
	assert.Equal(t, result{Stdout: want}, runCLI(t, "info", "trip.srt", "--json"))
}

func TestInfoReportsTheSourceEncoding(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	encoding := func(words ...string) any {
		got := call{dir: dir}.run(t, words...)
		require.Equal(t, 0, got.Code, got.Stderr)
		var list []map[string]any
		require.NoError(t, json.Unmarshal([]byte(got.Stdout), &list))
		return list[0]["encoding"]
	}
	assert.Equal(t, "UTF-8", encoding("info", "trip.srt", "--json"))
	assert.Equal(t, "Windows-1252", encoding("info", "latin1.srt", "--json"))
	assert.Equal(t, "Windows-1250", encoding("info", "latin1.srt", "--json", "--encoding", "Windows-1250"))
	assert.Regexp(t, `(?m)^  Encoding: +Windows-1252$`, call{dir: dir}.run(t, "info", "latin1.srt").Stdout)
}

func TestInfoDecodeWarning(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Stdout: "latin1.srt\n  Format:   srt\n  Encoding: Windows-1252\n  Cues:     1\n  Warnings: 1\n",
			Stderr: "latin1.srt: The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong. (repaired)\n" +
				encodingHint + "\n",
		},
		call{dir: dir}.run(t, "info", "latin1.srt", "--lenient"))

	got := call{dir: dir}.run(t, "info", "latin1.srt", "--lenient", "--encoding", "UTF-8")
	assert.Equal(t, 0, got.Code)
	assert.NotContains(t, got.Stderr, encodingHint)

	got = call{dir: dir}.run(t, "info", "latin1.srt")
	assert.Equal(t, result{Stdout: "latin1.srt\n  Format:   srt\n  Encoding: Windows-1252\n  Cues:     1\n"}, got)
}

func TestInfoInvalidUTF8Warning(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	message := "The content is not valid UTF-8. The first bad byte is at offset 35. Set the source encoding of the file."

	got := call{dir: dir}.run(t, "info", "latin1.srt", "--encoding", "UTF-8", "--lenient")
	assert.Equal(t, 0, got.Code)
	assert.Equal(t, "latin1.srt: "+message+" (repaired)\n", got.Stderr)

	got = call{dir: dir}.run(t, "info", "latin1.srt", "--encoding", "UTF-8", "--lenient", "--json")
	assert.Equal(t, 0, got.Code)
	var list []struct {
		Warnings []warningJSON `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal([]byte(got.Stdout), &list))
	require.Len(t, list, 1)
	require.Len(t, list[0].Warnings, 1)
	assert.Equal(t, message, list[0].Warnings[0].Message)
}

func TestInfoParseWarnings(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	message := `Block #1 has no timing line on its second line. The line is "00:00:03,000 => 00:00:04,000".`
	wantJSON := `[
    {
        "file": "broken.srt",
        "format": "srt",
        "encoding": "UTF-8",
        "metadata": {},
        "warnings": [
            {
                "lineNumber": 6,
                "blockIndex": 1,
                "message": "Block #1 has no timing line on its second line. The line is \"00:00:03,000 => 00:00:04,000\".",
                "action": "skipped"
            }
        ]
    }
]
`
	assert.Equal(t, result{Stdout: wantJSON, Stderr: "broken.srt: line 6: " + message + " (skipped)\n"},
		call{dir: dir}.run(t, "info", "broken.srt", "--json", "--lenient"))

	got := call{dir: dir}.run(t, "info", "broken.srt", "--lenient")
	assert.Equal(t, 0, got.Code)
	assert.Regexp(t, `(?m)^  Warnings: +1$`, got.Stdout)
	assert.NotRegexp(t, `Warnings`, runCLI(t, "info", "trip.srt").Stdout)
}

func TestInfoBatchWithAFailure(t *testing.T) {
	t.Parallel()

	got := runCLI(t, "info", "trip.srt", "shop.vtt", "broken.srt", "--json", "--keep-going")
	assert.Equal(t, exitFile, got.Code)
	assert.True(t, strings.HasPrefix(got.Stderr, "broken.srt: "), got.Stderr)
	assert.True(t, strings.HasSuffix(got.Stderr, "3 files: 2 succeeded, 1 failed.\n"), got.Stderr)
	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(got.Stdout), &list))
	require.Len(t, list, 2)
	assert.Equal(t, "trip.srt", list[0]["file"])
	assert.Equal(t, "shop.vtt", list[1]["file"])
	assert.Equal(t, "vtt", list[1]["format"])
}

func TestInfoAllFailed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, result{Code: exitFile, Stdout: "[]\n", Stderr: brokenMessage + "\n" + lenientHint + "\n"},
		runCLI(t, "info", "broken.srt", "--json"))
}

func TestInfoTextBatch(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Code: exitFile,
			Stdout: "trip.srt\n  Format:   srt\n  Encoding: UTF-8\n  Cues:     3\n" +
				"\nstdin\n  Format:   vtt\n  Encoding: UTF-8\n  Cues:     2\n" +
				"3 files: 2 succeeded, 1 failed.\n",
			Stderr: "notes.txt: " + unknownFormatMessage + "\n",
		},
		call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "trip.srt", "notes.txt", "-", "--keep-going"))
	assert.Equal(t,
		result{
			Code:   exitFile,
			Stdout: "trip.srt\n  Format:   srt\n  Encoding: UTF-8\n  Cues:     3\n3 files: 1 succeeded, 1 failed, 1 skipped.\n",
			Stderr: "notes.txt: " + unknownFormatMessage + "\nStopped at the first failure. Pass --keep-going to process the other files.\n",
		},
		call{dir: dir}.run(t, "info", "trip.srt", "notes.txt", "shop.vtt"))
}

func TestInfoReadsStandardInput(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	got := call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "-")
	assert.True(t, strings.HasPrefix(got.Stdout, "stdin\n  Format:   vtt\n"), got.Stdout)
	got = call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "-", "--json")
	assert.True(t, strings.HasPrefix(got.Stdout, "[\n    {\n        \"file\": \"-\",\n        \"format\": \"vtt\","), got.Stdout)
}

func TestInfoReadsWithFrom(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	got := call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "-", "--from", ".VTT")
	assert.Equal(t, 0, got.Code)
	assert.True(t, strings.HasPrefix(got.Stdout, "stdin\n  Format:   vtt\n"), got.Stdout)
}

func TestInfoMetadataRowsAndKeys(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(libraryJSONWithMetadata), 0o600))
	text := call{dir: dir}.run(t, "info", "meta.json")
	assert.Equal(t, 0, text.Code, text.Stderr)
	assert.Contains(t, text.Stdout, "  Metadata author: Anna\n  Metadata title:  A <b> & \"c\"\n  Metadata zulu:   last\n")

	got := call{dir: dir}.run(t, "info", "meta.json", "--json")
	assert.Equal(t, 0, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, `"metadata": {
            "author": "Anna",
            "title": "A <b> & \"c\"",
            "zulu": "last"
        },`)
}

func TestOrderedMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   orderedMetadata
		want string
	}{
		{"nil", nil, `{}`},
		{"empty", orderedMetadata{}, `{}`},
		{"sorted by bytes", orderedMetadata{"b": "2", "B": "1", "a": "3"}, `{"B":"1","a":"3","b":"2"}`},
		{"no HTML escape", orderedMetadata{"k": "<&>"}, `{"k":"<&>"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.in.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

func TestInfoJSONEqualsTheStatisticsFreePHPOutput(t *testing.T) {
	t.Parallel()

	var php, got []map[string]any
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "cli/trip_info.json"), &php))
	require.NoError(t, json.Unmarshal([]byte(runCLI(t, "info", "trip.srt", "--json").Stdout), &got))
	for _, entry := range php {
		delete(entry, "statistics")
		delete(entry, "imageCues")
	}
	assert.Equal(t, php, got)
}

func TestInfoTextStartsLikeThePHPOutput(t *testing.T) {
	t.Parallel()

	php := strings.Fields(strings.Join(strings.Split(string(testfiles.Read(t, "cli/trip_info.txt")), "\n")[:4], " "))
	got := strings.Fields(runCLI(t, "info", "trip.srt").Stdout)
	assert.Equal(t, php, got)
}
