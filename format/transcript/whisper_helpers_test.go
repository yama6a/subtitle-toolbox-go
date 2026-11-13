package transcript_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var withWords = transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true}

func readWhisper(t *testing.T, content string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _, err := transcript.ReadWhisper(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s
}

func readFile(t *testing.T, name string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	return readWhisper(t, string(testfiles.Read(t, "whisper/real/"+name)), opts)
}

func writeWhisper(t *testing.T, s *model.Subtitle) orderedjson.Value {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, transcript.WriteWhisper(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}))
	v, err := orderedjson.Decode(out.Bytes())
	require.NoError(t, err)
	return v
}

func parseJSON(t *testing.T, content string) orderedjson.Value {
	t.Helper()

	v, err := orderedjson.Decode([]byte(content))
	require.NoError(t, err)
	return v
}

// editCue applies edit to cue i of s.
func editCue(t *testing.T, s *model.Subtitle, i int, edit func(c *model.Cue)) {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok)
	edit(&c)
	require.NoError(t, s.SetCue(i, c))
}

// compact returns the JSON text of the value at path. A string in path is a key and an int is an index.
func compact(t *testing.T, v orderedjson.Value, path ...any) string {
	t.Helper()

	for _, p := range path {
		switch p := p.(type) {
		case string:
			var ok bool
			v, ok = v.Get(p)
			require.True(t, ok, "no key %q", p)
		case int:
			require.Less(t, p, len(v.Items()))
			v = v.Items()[p]
		}
	}
	return orderedjson.Block(v)
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// jsonOf returns the JSON text of a model.Data value.
func jsonOf(v any) string {
	out, err := jsondata.FromData(v)
	if err != nil {
		return err.Error()
	}
	return orderedjson.Block(out)
}
