package profanity_test

import (
	"errors"
	"fmt"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/profanity"
)

func ExampleApply() {
	const content = "WEBVTT\n\n00:01:02.000 --> 00:01:04.000\n" +
		"<00:01:02.000>What <00:01:02.300>the <00:01:02.480>hell <00:01:02.800>is this?\n"
	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	report, err := profanity.Apply(s, profanity.Options{
		Words:   []string{"hell", "damn*"},
		Mask:    profanity.MaskFirstLetter,
		Padding: 100 * time.Millisecond,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Text())
	}
	fmt.Print(profanity.EDL(report.MuteRanges))
	fmt.Println(profanity.FFmpegVolumeFilter(report.MuteRanges))
	// Output:
	// <00:01:02.000>What <00:01:02.300>the <00:01:02.480>h*** <00:01:02.800>is this?
	// 62.380 62.900 1
	// volume=enable='between(t,62.380,62.900)':volume=0
}

func ExampleEDL() {
	ranges := []profanity.MuteRange{
		{Start: 3400 * time.Millisecond, End: 5 * time.Second},
		{Start: 8 * time.Second, End: 9100 * time.Millisecond},
	}
	fmt.Print(profanity.EDL(ranges))
	// Output:
	// 3.400 5.000 1
	// 8.000 9.100 1
}

func ExampleFFmpegVolumeFilter() {
	ranges := []profanity.MuteRange{
		{Start: 1500 * time.Millisecond, End: 2100 * time.Millisecond},
		{Start: 6200 * time.Millisecond, End: 6800 * time.Millisecond},
	}
	fmt.Println(profanity.FFmpegVolumeFilter(ranges))
	// Output:
	// volume=enable='between(t,1.500,2.100)+between(t,6.200,6.800)':volume=0
}

func ExampleOptions_Validate() {
	err := profanity.Options{Words: []string{"f*ck"}}.Validate()
	fmt.Println(errors.Is(err, model.ErrInvalidArgument))
	fmt.Println(err)
	// Output:
	// true
	// stbox: invalid argument: profanity.Options.Words: A profanity word must be a non-empty UTF-8 string with a * only at the end, got 'f*ck'.
}
