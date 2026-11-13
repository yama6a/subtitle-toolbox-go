package transcript

import (
	"errors"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The helpers below are the same for the formats of the cloud speech-to-text services: Amazon Transcribe,
// Deepgram, AssemblyAI and Google Speech-to-Text.

// cloudFail passes a field error to State.Fail. Strict mode returns it, lenient mode skips the item with a warning.
func cloudFail(st *parsing.State, err error, block int, item orderedjson.Value) error {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return err
	}
	return st.Fail(perr, model.NoLine, block, []string{orderedjson.Block(item)})
}

func cloudData(v orderedjson.Value) *model.Data {
	d, _ := jsondata.ToData(v).(*model.Data)
	return d
}

func cloudGet(v orderedjson.Value, key string) orderedjson.Value {
	x, _ := v.Get(key)
	return x
}

// cloudStoredTime returns a number or a numeric string of seconds as whole milliseconds, or nil.
func cloudStoredTime(v orderedjson.Value) *time.Duration {
	if k := v.Kind(); k != orderedjson.Number && k != orderedjson.String {
		return nil
	}
	d, ok := v.Duration(time.Second)
	if !ok {
		return nil
	}
	return &d
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
