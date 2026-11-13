package transcript_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
)

func TestDetectPodcast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"start and body", `{"segments": [{"startTime": 0, "body": "Hi"}]}`, true},
		{"body first, values null", `{"segments": [{"body": null, "startTime": null}]}`, true},
		{"a BOM and white space first", "\xEF\xBB\xBF\r\n\t" + `{"segments": [{"startTime": 0, "body": "Hi"}]}`, true},
		{"only the first item counts", `{"segments": [{"startTime": 0, "body": "Hi"}, 5]}`, true},
		{"no body", `{"segments": [{"startTime": 0}]}`, false},
		{"Whisper segment", `{"segments": [{"start": 0, "end": 1, "text": "Hi"}]}`, false},
		{"first item no object", `{"segments": [5, {"startTime": 0, "body": "Hi"}]}`, false},
		{"empty list", `{"segments": []}`, false},
		{"an object as list", `{"segments": {"0": {"startTime": 0, "body": "Hi"}}}`, false},
		{"a list as root", `[{"segments": []}]`, false},
		{"invalid JSON", `{"segments": [`, false},
		{"empty", ``, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transcript.DetectPodcast([]byte(tt.content)))
		})
	}
}

func TestDetectPodcastJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		keys map[string]string
		want bool
	}{
		{"start and body", map[string]string{"segments": `[{"startTime": 0, "body": "Hi"}]`}, true},
		{"white space and null values", map[string]string{"segments": " \n[ {\"body\": null, \"startTime\": null} ]"}, true},
		{"no body", map[string]string{"segments": `[{"startTime": 0}]`}, false},
		{"first item null", map[string]string{"segments": `[null]`}, false},
		{"first item a list", map[string]string{"segments": `[[1]]`}, false},
		{"empty list", map[string]string{"segments": `[]`}, false},
		{"an object", map[string]string{"segments": `{"startTime": 0, "body": ""}`}, false},
		{"no keys", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			keys := map[string]json.RawMessage{}
			for k, v := range tt.keys {
				keys[k] = json.RawMessage(v)
			}
			assert.Equal(t, tt.want, transcript.DetectPodcastJSON(keys))
		})
	}
}
