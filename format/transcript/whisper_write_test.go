package transcript_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

var realFiles = []string{
	"openai_whisper_word_timestamps.json", "openai_whisper_german.json", "openai_api_verbose_json_words.json",
	"openai_api_verbose_json_french.json", "faster_whisper_ctranslate2_words.json", "faster_whisper_ctranslate2_segments.json",
	"whisperx_diarize.json", "whisper_cpp_oj.json", "whisper_cpp_ojf.json", "whisper_cpp_ojf_split_utf8.json",
}

// Ports testARoundTripKeepsTheDataOfTheSource of tests/Formatters/WhisperJsonFormatterTest.php of subtitle-toolbox-php.
// The reader makes no cue of a segment without text, so the writer drops such a segment.
func TestWriteWhisperRoundTrip(t *testing.T) {
	t.Parallel()

	for _, file := range realFiles {
		for name, opts := range map[string]transcript.ReadOptions{"default": {}, "words": withWords} {
			t.Run(file+"/"+name, func(t *testing.T) {
				t.Parallel()

				source := parseJSON(t, string(testfiles.Read(t, "whisper/real/"+file)))
				for _, key := range []string{"segments", "transcription"} {
					list, ok := source.Get(key)
					if !ok {
						continue
					}
					var kept []orderedjson.Value
					for _, seg := range list.Items() {
						if text, _ := seg.Get("text"); strings.TrimSpace(textOf(text)) != "" {
							kept = append(kept, seg)
						}
					}
					source = source.With(key, orderedjson.ArrayOf(kept...))
				}
				got := writeWhisper(t, readFile(t, file, opts))
				assert.Equal(t, orderedjson.Block(orderedjson.Canonical(source)), orderedjson.Block(orderedjson.Canonical(got)))
			})
		}
	}
}

func textOf(v orderedjson.Value) string {
	s, _ := v.AsString()
	return s
}
