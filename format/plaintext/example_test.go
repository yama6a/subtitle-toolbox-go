package plaintext_test

import (
	"fmt"
	"os"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Hello world."}},
		model.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}},
		model.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"<i>Home</i> &amp; bed."}},
	)
	if err := plaintext.Write(os.Stdout, s, model.CommonWriteOptions{}, plaintext.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// Hello world. Where are you going?
	//
	// Home & bed.
}
