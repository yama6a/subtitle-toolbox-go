package transcript_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/transcript"
)

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
