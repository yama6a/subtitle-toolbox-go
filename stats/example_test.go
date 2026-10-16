package stats_test

import (
	"fmt"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/stats"
)

func ExampleOf() {
	const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
		"2\n00:00:02,500 --> 00:00:05,000\n[BELL RINGS]\nANNA: We need two tickets for the long ride to the coast.\n\n" +
		"3\n00:00:06,000 --> 00:00:06,400\n(sighs) Too late.\n"
	s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	st := stats.Of(s)
	fmt.Println(st.CueCount, "cues,", st.WordCount, "words,", st.CharacterCount, "characters")
	fmt.Printf("characters per second: min %.1f, max %.1f\n", st.CharactersPerSecond.Min, st.CharactersPerSecond.Max)
	fmt.Println("gaps:", st.Gaps.Min, st.Gaps.Max)
	fmt.Println("most used:", st.MostUsedWords[0].Word, st.MostUsedWords[0].Count)
	fmt.Println("span:", *st.Span)
	// Output:
	// 3 cues, 22 words, 111 characters
	// characters per second: min 12.5, max 42.5
	// gaps: -500ms 1s
	// most used: the 3
	// span: 5.4s
}
