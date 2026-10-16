package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportCommandJSON(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Code: exitFile,
			Stdout: `[
    {
        "file": "trip.srt",
        "format": "srt",
        "warnings": []
    },
    {
        "file": "shop.vtt",
        "format": "vtt",
        "warnings": []
    }
]
`,
			Stderr: brokenMessage + "\n" + lenientHint + "\n3 files: 2 succeeded, 1 failed.\n",
		},
		call{dir: dir}.run(t, "info", "trip.srt", "shop.vtt", "broken.srt", "--json", "--keep-going"))
}

func TestReportCommandJSONWarnings(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Stdout: `[
    {
        "file": "broken.srt",
        "format": "srt",
        "warnings": [
            {
                "lineNumber": 6,
                "blockIndex": 1,
                "message": "Block #1 has no timing line on its second line. The line is \"00:00:03,000 => 00:00:04,000\".",
                "action": "skipped"
            }
        ]
    },
    {
        "file": "-",
        "format": "srt",
        "warnings": [
            {
                "lineNumber": null,
                "blockIndex": null,
                "message": "The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong.",
                "action": "repaired"
            }
        ]
    }
]
`,
			Stderr: brokenMessage + " (skipped)\n" +
				"stdin: The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong. (repaired)\n" +
				encodingHint + "\n2 files: 2 succeeded, 0 failed.\n",
		},
		call{dir: dir, stdin: readFile(t, dir, "latin1.srt")}.run(t, "info", "broken.srt", "-", "--json", "--lenient"))
}

func TestReportCommandText(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t,
		result{
			Code:   exitFile,
			Stdout: "trip.srt\n  Format: srt\nstdin\n  Format: vtt\n3 files: 2 succeeded, 1 failed.\n",
			Stderr: "notes.txt: " + unknownFormatMessage + "\n",
		},
		call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "trip.srt", "notes.txt", "-", "--keep-going"))
}

func TestReportCommandAllFailed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, result{Code: exitFile, Stdout: "[]\n", Stderr: "missing.srt: " + missingFileMessage + "\n"},
		runCLI(t, "info", "missing.srt", "--json"))
}

func TestReportCommandKeepsTheJSONText(t *testing.T) {
	t.Parallel()

	entries := []any{map[string]string{"text": "<b>a & b</b> \xff"}}
	r := &runner{c: &console{}}
	var out stringWriter
	r.c.stdout = &out
	o := &reportOutput{json: true, entries: entries}
	require.NoError(t, o.finish(r))
	assert.Equal(t, "[\n    {\n        \"text\": \"<b>a & b</b> \ufffd\"\n    }\n]\n", out.s) //nolint:testifylint // the test checks the bytes of the encoding
}

type stringWriter struct{ s string }

func (w *stringWriter) Write(p []byte) (int, error) {
	w.s += string(p)
	return len(p), nil
}

func TestReportCommandReadsWithFrom(t *testing.T) {
	t.Parallel()

	dir := newDir(t)
	assert.Equal(t, result{Stdout: "stdin\n  Format: vtt\n"},
		call{dir: dir, stdin: readFile(t, dir, "shop.vtt")}.run(t, "info", "-", "--from", ".VTT"))
}
