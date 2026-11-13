package transcript_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleReadAssemblyAI() {
	content := `{"language_code": "en_us", "words": [
		{"text": "Where", "start": 260, "end": 560, "confidence": 0.94655, "speaker": "A"},
		{"text": "to?", "start": 560, "end": 900, "confidence": 0.91655, "speaker": "A"}]}`
	opts := transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true}
	s, _, err := transcript.ReadAssemblyAI(strings.NewReader(content), model.CommonReadOptions{}, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	c, _ := s.Cue(0)
	fmt.Println(language, c.Start, c.End, c.Speaker, c.Lines)
	for _, w := range c.Words {
		fmt.Println(w.Text, w.Start, w.End, *w.Confidence)
	}
	// Output:
	// en-US 260ms 900ms A [Where to?]
	// Where 260ms 560ms 0.94655
	// to? 560ms 900ms 0.91655
}

func ExampleWriteAssemblyAI() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "en-US")
	s.AddCue(model.Cue{
		Start: 4 * time.Second, End: 5 * time.Second, Lines: []string{"See you."},
		Words: []model.Word{{Text: "See", Start: 4 * time.Second, End: 4500 * time.Millisecond}, {Text: "you.", Start: 4500 * time.Millisecond, End: 5 * time.Second}},
	})
	if err := transcript.WriteAssemblyAI(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {"language_code":"en_us","text":"See you.","words":[{"text":"See","start":4000,"end":4500,"confidence":1.0,"speaker":null},{"text":"you.","start":4500,"end":5000,"confidence":1.0,"speaker":null}],"utterances":null}
}
