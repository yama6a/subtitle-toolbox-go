package transcript_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleReadAWSTranscribe() {
	content := `{"jobName": "lecture-12", "results": {"language_code": "en-US", "items": [
		{"type": "pronunciation", "start_time": "0.04", "end_time": "0.51", "alternatives": [{"confidence": "0.99", "content": "Hello"}]},
		{"type": "pronunciation", "start_time": "0.51", "end_time": "0.98", "alternatives": [{"confidence": "0.98", "content": "world"}]},
		{"type": "punctuation", "alternatives": [{"confidence": "0.0", "content": "."}]}]}}`
	opts := transcript.ReadOptions{WordTimestamps: true}
	s, _, err := transcript.ReadAWSTranscribe(strings.NewReader(content), model.CommonReadOptions{}, opts)
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
	// en-US 40ms 980ms [Hello world.]
	// Hello 40ms 510ms 0.99
	// world. 510ms 980ms 0.98
}

func ExampleWriteAWSTranscribe() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{
		Start: 4 * time.Second, End: 5 * time.Second, Lines: []string{"See you."}, Speaker: "Ben",
		Words: []model.Word{{Text: "See", Start: 4 * time.Second, End: 4500 * time.Millisecond}, {Text: "you.", Start: 4500 * time.Millisecond, End: 5 * time.Second}},
	})
	if err := transcript.WriteAWSTranscribe(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {"results":{"transcripts":[{"transcript":"See you."}],"speaker_labels":{"speakers":1,"segments":[{"start_time":"4.0","speaker_label":"Ben","end_time":"5.0","items":[{"start_time":"4.0","speaker_label":"Ben","end_time":"4.5"},{"start_time":"4.5","speaker_label":"Ben","end_time":"5.0"}]}]},"items":[{"id":0,"start_time":"4.0","speaker_label":"Ben","end_time":"4.5","alternatives":[{"confidence":"1.0","content":"See"}],"type":"pronunciation"},{"id":1,"start_time":"4.5","speaker_label":"Ben","end_time":"5.0","alternatives":[{"confidence":"1.0","content":"you"}],"type":"pronunciation"},{"id":2,"speaker_label":"Ben","alternatives":[{"confidence":"0.0","content":"."}],"type":"punctuation"}]}}
}
