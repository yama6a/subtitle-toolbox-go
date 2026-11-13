package model_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func ExampleCue_ExtractWordTags() {
	c := model.Cue{End: 5 * time.Second, Lines: []string{"<v Anna>Good <00:00:01.000>morning."}}
	c.ExtractWordTags()
	fmt.Println(c.Speaker, c.Lines)
	for _, w := range c.Words {
		fmt.Println(w.Text, w.Start, w.End)
	}
	// Output:
	// Anna [Good morning.]
	// morning. 1s 5s
}

func ExampleCue_TaggedLines() {
	c := model.Cue{
		Lines: []string{"The beach"},
		Words: []model.Word{{Text: "The", End: time.Second}, {Text: "beach", Start: 240 * time.Millisecond, End: time.Second}},
	}
	fmt.Println(c.TaggedLines())
	// Output: [<00:00:00.000>The <00:00:00.240>beach]
}

func ExampleCue_SyncWords() {
	c := model.Cue{
		Lines: []string{"damn it"},
		Words: []model.Word{{Text: "damn", End: time.Second}, {Text: "it", Start: time.Second, End: 2 * time.Second}},
	}
	c.SetText("d*** it")
	fmt.Println(c.SyncWords(), c.Words)
	// Output: 1 [{it 1s 2s <nil>}]
}

func ExampleCue_PlainText() {
	c := model.Cue{Lines: []string{"<i>Fish</i> &amp;", "Chips"}}
	fmt.Println(c.PlainText())
	// Output: Fish & Chips
}

func ExampleCue_ValidateWords() {
	c := model.Cue{Lines: []string{"Hi there"}, Words: []model.Word{{Text: "there"}, {Text: "Hi"}}}
	fmt.Println(c.ValidateWords())
	// Output: stbox: invalid argument: Words[1].Text: The word "Hi" is not in the text of the cue after the word before it.
}
