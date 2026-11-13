package transcript_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
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
