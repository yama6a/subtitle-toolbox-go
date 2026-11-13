package transcript_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleReadWhisper() {
	content := `{"text": " The bakery opens at seven.", "language": "english", "segments": [
		{"id": 0, "start": 0.0, "end": 2.2, "text": " The bakery opens at seven.", "words": [
			{"word": " The", "start": 0.0, "end": 0.24, "probability": 0.99},
			{"word": " bakery", "start": 0.24, "end": 0.7, "probability": 0.95}]}]}`
	opts := transcript.ReadOptions{WordTimestamps: true}
	s, _, err := transcript.ReadWhisper(strings.NewReader(content), model.CommonReadOptions{}, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	c, _ := s.Cue(0)
	fmt.Println(language, c.Start, c.End, c.Lines)
	for _, w := range c.Words {
		fmt.Println(w.Text, w.Start, w.End, *w.Confidence)
	}
	// Output:
	// en 0s 2.2s [The bakery opens at seven.]
	// The 0s 240ms 0.99
	// bakery 240ms 700ms 0.95
}

func ExampleWriteWhisper() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "de")
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Guten", "Morgen."}, Speaker: "Anna"})
	if err := transcript.WriteWhisper(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {"text":" Guten Morgen.","segments":[{"id":0,"start":1.5,"end":3.0,"text":" Guten Morgen.","speaker":"Anna"}],"language":"de"}
}

func ExampleDetectWhisper() {
	fmt.Println(transcript.DetectWhisper([]byte(`{"text": "Hi", "segments": []}`)))
	fmt.Println(transcript.DetectWhisper([]byte(`{"systeminfo": "", "transcription": []}`)))
	fmt.Println(transcript.DetectWhisper([]byte(`{"text": "Hi"}`)))
	// Output:
	// true
	// true
	// false
}

func ExampleDetectWhisperJSON() {
	keys := map[string]json.RawMessage{"text": json.RawMessage(`"Hi"`), "segments": json.RawMessage(`[]`)}
	fmt.Println(transcript.DetectWhisperJSON(keys))
	fmt.Println(transcript.DetectWhisperJSON(map[string]json.RawMessage{"segments": json.RawMessage(`{}`)}))
	// Output:
	// true
	// false
}
