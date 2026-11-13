package transcript_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleReadDeepgram() {
	content := `{"results": {"channels": [{"detected_language": "de", "alternatives": [{"words": [
		{"word": "der", "start": 0.4, "end": 0.82, "confidence": 0.9087655, "punctuated_word": "Der"},
		{"word": "zug", "start": 0.82, "end": 1.24, "confidence": 0.9187655, "punctuated_word": "Zug."}]}]}]}}`
	opts := transcript.ReadOptions{WordTimestamps: true}
	s, _, err := transcript.ReadDeepgram(strings.NewReader(content), model.CommonReadOptions{}, opts)
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
	// de 400ms 1.24s [Der Zug.]
	// Der 400ms 820ms 0.9087655
	// Zug. 820ms 1.24s 0.9187655
}

func ExampleWriteDeepgram() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Guten", "Morgen."}, Speaker: "1"})
	if err := transcript.WriteDeepgram(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {"results":{"channels":[{"alternatives":[{"transcript":"Guten Morgen.","words":[]}]}],"utterances":[{"start":1.5,"end":3.0,"confidence":1.0,"channel":0,"transcript":"Guten Morgen.","words":[],"speaker":1}]}}
}
