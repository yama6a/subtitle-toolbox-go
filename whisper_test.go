package stbox_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func taggedLines(s *stbox.Subtitle) [][]string {
	var out [][]string
	for _, c := range s.All() {
		out = append(out, c.TaggedLines())
	}
	return out
}

// Ports testWhisperSpeakerVoicesAreOffByDefault, testWhisperCppDiarizeFile and
// testWhisperSpeakerVoicesWithWordTimestampsAndEscaping of tests/Speakers/SpeakerLabelsTest.php of subtitle-toolbox-php.
func TestWhisperSpeakerVoices(t *testing.T) {
	t.Parallel()

	diarize := string(testfiles.Read(t, "speakers/whisper_cpp_diarize.json"))
	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    [][]string
	}{
		{
			name: "off by default", content: diarize,
			want: [][]string{{"Did you lock the back door?"}, {"Yes, and the window."}, {"Both of us checked it twice."}, {"Then we can go."}},
		},
		{
			name: "whisper.cpp with -di", content: diarize, opts: transcript.ReadOptions{SpeakerVoices: true},
			want: [][]string{
				{"<v 0>Did you lock the back door?"},
				{"<v 1>Yes, and the window."},
				{"<v ?>Both of us checked it twice."},
				{"<v 0>Then we can go."},
			},
		},
		{
			name: "escaping, a blank speaker and a number",
			content: `{"segments": [{"start": 0, "end": 2, "text": " Hi.", "speaker": "O'Neil & <Son>",` +
				` "words": [{"word": "Hi.", "start": 0.5, "end": 1}]}, {"start": 2, "end": 3, "text": "Bye.", "speaker": " "},` +
				` {"start": 3, "end": 4, "text": "Yes.", "speaker": 5}]}`,
			opts: transcript.ReadOptions{SpeakerVoices: true, WordTimestamps: true},
			want: [][]string{{"<v O'Neil &amp; &lt;Son&gt;><00:00:00.500>Hi."}, {"Bye."}, {"Yes."}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := parseBytes([]byte(tt.content), stbox.ReadOptions{Format: stbox.Whisper, Transcript: tt.opts})
			require.NoError(t, err)
			assert.Equal(t, tt.want, taggedLines(s))
		})
	}
}

func TestWhisperCppDiarizeWritesWebVTT(t *testing.T) {
	t.Parallel()

	s, _, err := parseBytes(testfiles.Read(t, "speakers/whisper_cpp_diarize.json"),
		stbox.ReadOptions{Transcript: transcript.ReadOptions{SpeakerVoices: true}})
	require.NoError(t, err)
	c, _ := s.Cue(2)
	data, _ := c.FormatData("whisper")
	speaker, _ := data.String("speaker")
	assert.Equal(t, "?", speaker)

	var out bytes.Buffer
	_, err = stbox.Write(&out, s, stbox.WebVTT, stbox.WriteOptions{BOM: stbox.BOMRemove})
	require.NoError(t, err)
	assert.Equal(t, string(testfiles.Read(t, "speakers/whisper_cpp_diarize.vtt")), out.String())
}

// Ports testDetectionDoesNotTakeWhisperJson of tests/Parsers/CloudSpeechParsersTest.php of subtitle-toolbox-php.
func TestDetectFindsEveryWhisperFile(t *testing.T) {
	t.Parallel()

	for _, file := range folder(t, "whisper").realFiles(t) {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			d, ok := stbox.Detect(testfiles.Read(t, "whisper/real/"+file))
			require.True(t, ok)
			assert.Equal(t, stbox.Whisper, d.Format)
		})
	}
}

func TestLibraryJSONWinsOverWhisper(t *testing.T) {
	t.Parallel()

	d, ok := stbox.Detect([]byte(`{"version": 1, "cues": [], "segments": []}`))
	require.True(t, ok)
	assert.Equal(t, stbox.JSON, d.Format)
}
