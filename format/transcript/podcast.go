package transcript

import (
	"bytes"
	"encoding/json"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const podcastKey = string(model.PodcastTranscript)

// PodcastWriteOptions holds the settings of the Podcasting 2.0 transcript writer.
type PodcastWriteOptions struct {
	// WordSegments writes one segment per word of Cue.Words, for the word highlight of podcast apps.
	// False writes one segment per cue and speaker.
	WordSegments bool
	// PrettyPrint writes a 4-space indent and a final LF. False writes one compact line.
	PrettyPrint bool
}

// DetectPodcast reports whether content is a JSON object whose segments list starts with an object that has the
// keys startTime and body. stbox.Detect runs the same check before the Whisper check.
func DetectPodcast(content []byte) bool {
	content = bytes.TrimLeft(textenc.TrimUTF8BOMs(content), markupcore.PHPTrimSet)
	root, err := orderedjson.Decode(content)
	if err != nil || root.Kind() != orderedjson.Object {
		return false
	}
	segments, _ := root.Get("segments")
	if segments.Kind() != orderedjson.Array || len(segments.Items()) == 0 {
		return false
	}
	first := segments.Items()[0]
	return first.Kind() == orderedjson.Object && first.Has("startTime") && first.Has("body")
}

// DetectPodcastJSON is JSON check 2 on the top-level keys of an object. It reports whether segments is an array
// whose first item is an object with the keys startTime and body, with any value.
func DetectPodcastJSON(keys map[string]json.RawMessage) bool {
	dec := json.NewDecoder(bytes.NewReader(keys["segments"]))
	if token, err := dec.Token(); err != nil || token != json.Delim('[') || !dec.More() {
		return false
	}
	var first map[string]json.RawMessage
	if err := dec.Decode(&first); err != nil {
		return false
	}
	_, hasStart := first["startTime"]
	_, hasBody := first["body"]
	return hasStart && hasBody
}
