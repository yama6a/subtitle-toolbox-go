package stboxjson_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
)

func TestDetectJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"version and cues", `{"version": 1, "cues": []}`, true},
		{"cues first", `{"cues": [], "metadata": {"title": "version"}, "version": 1}`, true},
		{"spaces around the values", "{\n  \"version\" : 1 ,\n  \"cues\" : [ ]\n}\n", true},
		{"float version", `{"version": 1.5, "cues": []}`, true},
		{"negative version", `{"version": -1, "cues": []}`, true},
		{"version 0 with an exponent", `{"version": 0e3, "cues": [{"start": 1}]}`, true},
		{"segments in format data", `{"version": 1, "formatData": {"x": {"segments": [1]}}, "cues": []}`, true},
		{"version as a string", `{"version": "1", "cues": []}`, false},
		{"version null", `{"version": null, "cues": []}`, false},
		{"version true", `{"version": true, "cues": []}`, false},
		{"cues as an object", `{"version": 1, "cues": {}}`, false},
		{"cues as a string", `{"version": 1, "cues": "[]"}`, false},
		{"cues null", `{"version": 1, "cues": null}`, false},
		{"no version", `{"cues": [{"start": 1, "end": 2, "text": "Hello"}]}`, false},
		{"no cues", `{"version": 1}`, false},
		{"cues only inside a string", `{"version": 1, "text": "\"cues\": ["}`, false},
		{"Whisper", `{"text": " Hello", "segments": [], "language": "en"}`, false},
		{"empty object", `{}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var keys map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(tt.content), &keys))
			assert.Equal(t, tt.want, stboxjson.DetectJSON(keys))
		})
	}
}

func TestDetectJSONWithoutKeys(t *testing.T) {
	t.Parallel()

	assert.False(t, stboxjson.DetectJSON(nil))
}
