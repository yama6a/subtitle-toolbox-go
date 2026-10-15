package stboxjson_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := `{
    "version": 1,
    "metadata": {"title": "Morning train"},
    "cues": [{"start": 1.0, "end": 3.5, "lines": ["<i>The train leaves at nine.</i>"], "alignment": 8}]
}`
	s, _, err := stboxjson.Read(strings.NewReader(content), model.CommonReadOptions{}, stboxjson.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	title, _ := s.Metadata(model.MetadataTitle)
	c, _ := s.Cue(0)
	fmt.Println(title)
	fmt.Println(c.Start, c.End, c.Alignment, c.Text())

	_, _, err = stboxjson.Read(strings.NewReader(`{"version": 1, "cues": [{"start": "1", "end": 2, "lines": []}]}`),
		model.CommonReadOptions{}, stboxjson.ReadOptions{})
	fmt.Println(err)
	// Output:
	// Morning train
	// 1s 3.5s TopCenter <i>The train leaves at nine.</i>
	// json: The field cues[0].start must be a number.
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataTitle, "Morning train")
	s.AddCue(model.Cue{Start: time.Second, End: 3500 * time.Millisecond, Lines: []string{"The train leaves at nine."}, Forced: true})
	err := stboxjson.Write(os.Stdout, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{PrettyPrint: true, OmitFormatData: true})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// {
	//     "version": 1,
	//     "metadata": {
	//         "title": "Morning train"
	//     },
	//     "comments": [],
	//     "cues": [
	//         {
	//             "start": 1.0,
	//             "end": 3.5,
	//             "lines": [
	//                 "The train leaves at nine."
	//             ],
	//             "identifier": null,
	//             "alignment": null,
	//             "forced": true
	//         }
	//     ]
	// }
}

func ExampleDetectJSON() {
	for _, content := range []string{`{"version": 1, "cues": []}`, `{"version": "1", "cues": []}`} {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal([]byte(content), &keys); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(stboxjson.DetectJSON(keys))
	}
	// Output:
	// true
	// false
}
