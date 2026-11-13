package transcript_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
)

func TestDetectWhisper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"segments", `{"segments": []}`, true},
		{"transcription", `{"transcription": []}`, true},
		{"a BOM, a CR LF and a tab first", "\xEF\xBB\xBF\r\n\t{\"segments\": []}", true},
		{"segments with a cues key", `{"version": "1", "cues": [], "segments": []}`, true},
		{"segments as an object", `{"segments": {}}`, false},
		{"segments as text", `{"segments": "[]"}`, false},
		{"text only", `{"text": "Hi"}`, false},
		{"words only", `{"words": []}`, false},
		{"a list", `[{"segments": []}]`, false},
		{"invalid JSON", `{"segments": [`, false},
		{"empty", ``, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transcript.DetectWhisper([]byte(tt.content)))
		})
	}
}

func TestDetectWhisperJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keys map[string]string
		want bool
	}{
		{"segments", map[string]string{"segments": "[]"}, true},
		{"transcription with white space", map[string]string{"transcription": " \n[1]"}, true},
		{"segments as an object", map[string]string{"segments": "{}"}, false},
		{"no keys", nil, false},
		{"another key", map[string]string{"words": "[]"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			keys := map[string]json.RawMessage{}
			for k, v := range tt.keys {
				keys[k] = json.RawMessage(v)
			}
			assert.Equal(t, tt.want, transcript.DetectWhisperJSON(keys))
		})
	}
}
