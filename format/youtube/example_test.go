package youtube_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleRead() {
	content := `<transcript><text start="1.2" dur="2.3">Hello &amp;amp; welcome</text><text start="5">Next</text></transcript>`
	s, _, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{}, youtube.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.Cues() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// 1.2s 3.5s [Hello &amp; welcome]
	// 5s 10s [Next]
}

func ExampleWrite() {
	content := `<transcript><text start="1.2" dur="2.3">Hello</text></transcript>`
	s, _, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{}, youtube.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	c, _ := s.Cue(0)
	c.SetText("Hello & welcome")
	if err := s.SetCue(0, c); err != nil {
		fmt.Println(err)
		return
	}
	if err := youtube.Write(os.Stdout, s, model.CommonWriteOptions{}, youtube.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <?xml version="1.0" encoding="utf-8" ?><transcript><text start="1.2" dur="2.3">Hello &amp; welcome</text></transcript>
}

func ExampleDetect() {
	fmt.Println(youtube.Detect([]byte(`<?xml version="1.0" encoding="utf-8" ?><timedtext format="3"></timedtext>`)))
	fmt.Println(youtube.Detect([]byte(`<tt xmlns="http://www.w3.org/ns/ttml"/>`)))
	// Output:
	// true
	// false
}

func ExampleDetectJSON() {
	keys := map[string]json.RawMessage{"events": json.RawMessage(`[{"tStartMs": 0}]`)}
	fmt.Println(youtube.DetectJSON(keys))
	fmt.Println(youtube.DetectJSON(map[string]json.RawMessage{"events": json.RawMessage(`[]`)}))
	// Output:
	// true
	// false
}
