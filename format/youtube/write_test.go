package youtube_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Ports the round trips of tests/Formatters/YouTubeTimedTextFormatterTest.php of subtitle-toolbox-php. An XML
// variant gives the source bytes, with and without word times.
func TestWriteRoundTripGivesTheSourceBytes(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"transcript.en.srv1", "missing-dur.en.srv1", "missing-d.en.srv2", "missing-d.en.srv3", "auto.en.srv3", "styled.en.srv3", "blank-lines.en.srv3",
	} {
		for name, opts := range map[string]youtube.ReadOptions{"default": {}, "words": withWords} {
			t.Run(file+"/"+name, func(t *testing.T) {
				t.Parallel()

				s, src := readFile(t, file, opts)
				assert.Equal(t, strings.TrimLeft(src, " \n"), write(t, s))
			})
		}
	}
}

func TestWriteSrv1AndSrv2AfterEdits(t *testing.T) {
	t.Parallel()

	const (
		srv1 = `<transcript><text start="1" dur="2">a</text><text start="4">b &amp;#39;x</text></transcript>`
		srv2 = "<timedtext>\n<text t=\"1000\" d=\"2000\">a</text>\n<text t=\"4000\">b</text>\n</timedtext>\n"
		decl = `<?xml version="1.0" encoding="utf-8" ?>`
	)
	tests := []struct {
		name, source string
		edit         func(c *model.Cue)
		cue          int
		want         string
	}{
		{
			name: "no edit", source: srv1, cue: 0, edit: func(*model.Cue) {},
			want: decl + `<transcript><text start="1" dur="2">a</text><text start="4">b &amp;#39;x</text></transcript>`,
		},
		{
			name: "a move keeps the stored duration", source: srv1, cue: 0,
			edit: func(c *model.Cue) { c.Start += ms(400); c.End += ms(400) },
			want: decl + `<transcript><text start="1.4" dur="2">a</text><text start="4">b &amp;#39;x</text></transcript>`,
		},
		{
			name: "a longer cue writes the new duration", source: srv1, cue: 0,
			edit: func(c *model.Cue) { c.End += ms(1250) },
			want: decl + `<transcript><text start="1" dur="3.25">a</text><text start="4">b &amp;#39;x</text></transcript>`,
		},
		{
			name: "a new text escapes once and drops the stored text", source: srv1, cue: 1,
			edit: func(c *model.Cue) { c.SetText("Tom &amp; Jerry <b>x</b>") },
			want: decl + `<transcript><text start="1" dur="2">a</text><text start="4">Tom &amp; Jerry x</text></transcript>`,
		},
		{
			name: "srv2 in milliseconds", source: srv2, cue: 0,
			edit: func(c *model.Cue) { c.Start += ms(1) },
			want: decl + "\n<timedtext>\n<text t=\"1001\" d=\"1999\">a</text>\n<text t=\"4000\">b</text>\n</timedtext>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := read(t, tt.source, youtube.ReadOptions{})
			editCue(t, s, tt.cue, tt.edit)
			assert.Equal(t, tt.want, write(t, s))
		})
	}
}

func TestWriteRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	s := read(t, `<transcript><text start="1" dur="2">a</text></transcript>`, youtube.ReadOptions{})
	editCue(t, s, 0, func(c *model.Cue) { c.Lines = []string{"caf\xe9"} })
	err := youtube.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, youtube.WriteOptions{})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	require.ErrorIs(t, err, model.ErrInvalidUTF8)
	assert.Equal(t, "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.", uerr.Reason)
}

// Each source stays as it is after a read and a write without an edit.
func TestWriteKeepsTheSourceOfOddFiles(t *testing.T) {
	t.Parallel()

	const decl = `<?xml version="1.0" encoding="utf-8" ?>`
	tests := []struct{ name, source string }{
		{
			name:   "json3 without a duration",
			source: `{"wireMagic":"pb3","pens":[{}],"wsWinStyles":[{}],"wpWinPositions":[{}],"events":[{"tStartMs":0,"segs":[{"utf8":"a"}]}]}`,
		},
		{
			name:   "json3 with a null duration",
			source: `{"wireMagic":"pb3","pens":[{}],"wsWinStyles":[{}],"wpWinPositions":[{}],"events":[{"tStartMs":0,"dDurationMs":null,"segs":[{"utf8":"a"}]}]}`,
		},
		{
			name: "a span with the pen 0 takes the pen of its paragraph",
			source: decl + "<timedtext format=\"3\">\n<head>\n<pen id=\"1\" b=\"1\"/>\n</head>\n<body>\n" +
				"<p t=\"0\" d=\"1000\" p=\"1\"><s p=\"0\">Hi</s></p>\n</body>\n</timedtext>\n",
		},
		{
			name: "a window position with a fraction in ap counts as bottom center",
			source: decl + "<timedtext format=\"3\">\n<head>\n<wp id=\"1\" ap=\"4.5\"/>\n</head>\n<body>\n" +
				"<p t=\"0\" d=\"1000\" wp=\"1\">Hi</p>\n</body>\n</timedtext>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := youtube.Read(strings.NewReader(tt.source), model.CommonReadOptions{Lenient: true}, withWords)
			require.NoError(t, err)
			assert.Equal(t, tt.source, write(t, s))
		})
	}
}
