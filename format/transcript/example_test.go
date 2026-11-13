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

func ExampleReadPodcast() {
	content := `{"version": "1.0.0", "segments": [
		{"speaker": "Anna", "startTime": 0.5, "endTime": 0.75, "body": "The"},
		{"speaker": "Anna", "startTime": 1, "endTime": 1.25, "body": "bakery"},
		{"speaker": "Anna", "startTime": 1.5, "endTime": 2, "body": "opens."},
		{"speaker": "Ben", "startTime": 3, "body": "Good news"}]}`
	s, _, err := transcript.ReadPodcast(strings.NewReader(content), model.CommonReadOptions{}, transcript.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.Cues() {
		fmt.Println(c.Start, c.End, c.Speaker, c.Lines)
	}
	// Output:
	// 500ms 2s Anna [The bakery opens.]
	// 3s 8s Ben [Good news]
}

func ExampleWritePodcast() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Guten", "Morgen."}, Speaker: "Anna"})
	opts := transcript.WriteOptions{Podcast: transcript.PodcastWriteOptions{PrettyPrint: true}}
	if err := transcript.WritePodcast(os.Stdout, s, model.CommonWriteOptions{}, opts); err != nil {
		fmt.Println(err)
	}
	// Output:
	// {
	//     "version": "1.0.0",
	//     "segments": [
	//         {
	//             "speaker": "Anna",
	//             "startTime": 1.5,
	//             "endTime": 3.0,
	//             "body": "Guten Morgen."
	//         }
	//     ]
	// }
}

func ExampleDetectPodcast() {
	fmt.Println(transcript.DetectPodcast([]byte(`{"segments": [{"startTime": 0, "body": "Hi"}]}`)))
	fmt.Println(transcript.DetectPodcast([]byte(`{"segments": [{"start": 0, "text": "Hi"}]}`)))
	// Output:
	// true
	// false
}

func ExampleDetectPodcastJSON() {
	keys := map[string]json.RawMessage{"segments": json.RawMessage(`[{"startTime": 0, "body": "Hi"}]`)}
	fmt.Println(transcript.DetectPodcastJSON(keys))
	fmt.Println(transcript.DetectPodcastJSON(map[string]json.RawMessage{"segments": json.RawMessage(`[]`)}))
	// Output:
	// true
	// false
}

func ExampleReadHTML() {
	content := `<cite>Anna:</cite>
<time>0:05</time>
<p>Hello<br>there.</p>
<time>1:02</time>
<p>Bye.</p>`
	s, _, err := transcript.ReadHTML(strings.NewReader(content), model.CommonReadOptions{}, transcript.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.Cues() {
		fmt.Printf("%v %v %q %v\n", c.Start, c.End, c.Speaker, c.Lines)
	}
	// Output:
	// 5s 1m2s "Anna" [Hello there.]
	// 1m2s 1m7s "" [Bye.]
}

func ExampleWriteHTML() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Hello", "there."}, Speaker: "Anna"})
	s.AddCue(model.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}, Speaker: "Anna"})
	s.AddCue(model.Cue{Start: 3725 * time.Second, End: 3727 * time.Second, Lines: []string{"Fish & chips."}})
	if err := transcript.WriteHTML(os.Stdout, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <cite>Anna:</cite>
	// <time>0:01</time>
	// <p>Hello there. Where are you going?</p>
	// <time>1:02:05</time>
	// <p>Fish &amp; chips.</p>
}

func ExampleDetectHTML() {
	fmt.Println(transcript.DetectHTML([]byte("<cite>Anna:</cite>\n<time>0:05</time>\n<p>Hi</p>")))
	fmt.Println(transcript.DetectHTML([]byte("<time>0:05</time>\n<p>Hi</p>")))
	// Output:
	// true
	// false
}
