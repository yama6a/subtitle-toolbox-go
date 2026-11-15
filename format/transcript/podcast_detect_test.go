package transcript_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
)

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
