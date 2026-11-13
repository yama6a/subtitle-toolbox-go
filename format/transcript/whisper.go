package transcript

import (
	"bytes"
	"encoding/json"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The JSON shapes of the Whisper tools. The reader stores the shape as the format field of the file format data.
const (
	shapeOpenAIWhisper = "openai-whisper"
	shapeOpenAIAPI     = "openai-api"
	shapeWhisperX      = "whisperx"
	shapeWhisperCPP    = "whisper.cpp"
)

const whisperKey = string(model.Whisper)

// DetectWhisper reports whether content is a JSON object with a segments list or a transcription list.
// stbox.Detect runs the same check after the library JSON check.
func DetectWhisper(content []byte) bool {
	content = bytes.TrimLeft(textenc.TrimUTF8BOMs(content), markupcore.PHPTrimSet)
	root, err := orderedjson.Decode(content)
	if err != nil || root.Kind() != orderedjson.Object {
		return false
	}
	return isList(root, "segments") || isList(root, "transcription")
}

// DetectWhisperJSON is JSON check 3 on the top-level keys of an object. It reports whether segments or
// transcription is an array.
func DetectWhisperJSON(keys map[string]json.RawMessage) bool {
	return startsList(keys["segments"]) || startsList(keys["transcription"])
}

func startsList(raw json.RawMessage) bool {
	raw = bytes.TrimLeft(raw, " \t\r\n")
	return len(raw) > 0 && raw[0] == '['
}

func isList(object orderedjson.Value, key string) bool {
	v, _ := object.Get(key)
	return v.Kind() == orderedjson.Array
}

// shapeOf returns the name of the tool whose JSON shape root has.
func shapeOf(root orderedjson.Value) string {
	present := func(key string) bool {
		v, _ := root.Get(key)
		return v.Kind() != orderedjson.Null
	}
	switch {
	case !isList(root, "segments"):
		return shapeWhisperCPP
	case present("word_segments"):
		return shapeWhisperX
	case present("task") || isList(root, "words"):
		return shapeOpenAIAPI
	}
	return shapeOpenAIWhisper
}
