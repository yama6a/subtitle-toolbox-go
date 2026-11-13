package transcript_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleReadGoogleSpeech() {
	content := `{"results": [{"alternatives": [{"transcript": "okay the bus", "confidence": 0.96, "words": [
		{"startTime": "0.200s", "endTime": "0.660s", "word": "okay"},
		{"startTime": "0.660s", "endTime": "1.080s", "word": "the"},
		{"startTime": "1.080s", "endTime": "1.500s", "word": "bus"}]}], "resultEndTime": "1.500s", "languageCode": "en-us"}]}`
	opts := transcript.ReadOptions{WordTimestamps: true}
	s, _, err := transcript.ReadGoogleSpeech(strings.NewReader(content), model.CommonReadOptions{}, opts)
	if err != nil {
		fmt.Println(err)
		return
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	c, _ := s.Cue(0)
	fmt.Println(language, c.Start, c.End, c.Lines)
	for _, w := range c.Words {
		fmt.Println(w.Text, w.Start, w.End)
	}
	// Output:
	// en-us 200ms 1.5s [okay the bus]
	// okay 200ms 660ms
	// the 660ms 1.08s
	// bus 1.08s 1.5s
}

func ExampleWriteGoogleSpeech() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "en-US")
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Good", "morning."}})
	if err := transcript.WriteGoogleSpeech(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {"results":[{"alternatives":[{"transcript":"Good morning."}],"resultEndTime":"3s","languageCode":"en-US"}]}
}
