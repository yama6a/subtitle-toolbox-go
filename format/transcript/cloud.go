package transcript

import (
	"math"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The helpers below are the same for the formats of the cloud speech-to-text services: Amazon Transcribe,
// Deepgram, AssemblyAI and Google Speech-to-Text.

func cloudData(v orderedjson.Value) *model.Data {
	d, _ := jsondata.ToData(v).(*model.Data)
	return d
}

func cloudGet(v orderedjson.Value, key string) orderedjson.Value {
	x, _ := v.Get(key)
	return x
}

// cloudConfidence returns a number from 0 to 1, else nil.
func cloudConfidence(v orderedjson.Value) *float64 {
	f, ok := v.AsFloat64()
	if !ok || math.IsNaN(f) || f < 0 || f > 1 {
		return nil
	}
	return &f
}

// cloudFileData returns the file format data under key as an object, or an empty object.
func cloudFileData(s *model.Subtitle, f model.Format) (orderedjson.Value, error) {
	d, ok := s.FormatData(string(f))
	if !ok {
		return orderedjson.ObjectOf(), nil
	}
	v, err := jsondata.FromData(d)
	if err != nil {
		return v, &model.UnwritableError{Format: f, CueIndex: -1, Reason: "The " + string(f) + " format data is no JSON.", Err: err}
	}
	if v.Kind() != orderedjson.Object {
		return orderedjson.ObjectOf(), nil
	}
	return v, nil
}
