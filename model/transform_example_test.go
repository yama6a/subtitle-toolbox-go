package model_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleSubtitle_ChangeCase() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1001 * time.Millisecond, End: 3336 * time.Millisecond, Lines: []string{">> GOOD MORNING. THE 8:15 TRAIN", "TO MAIN STREET IS LATE."}},
		model.Cue{Start: 3503 * time.Millisecond, End: 5505 * time.Millisecond, Lines: []string{"<i>I THINK WE MISSED IT.</i>"}},
	)

	if err := s.ChangeCase(model.CaseSentence, "en"); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Text())
	}
	// Output:
	// >> Good morning. The 8:15 train
	// to main street is late.
	// <i>I think we missed it.</i>
}

func ExampleSubtitle_ReplaceText() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"[BELL RINGS]"}},
		model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"<i>Wait.....</i> [DOOR] Tom &amp; Jerry"}},
	)

	if err := s.ReplaceText(`\s*\[[^\]]*\]`, "", model.ReplaceOptions{Regex: true}); err != nil {
		fmt.Println(err)
		return
	}
	if err := s.ReplaceText(`\.{4,}`, "...", model.ReplaceOptions{Regex: true}); err != nil {
		fmt.Println(err)
		return
	}
	if err := s.ReplaceText("tom & jerry", "Tom & Jerry's", model.ReplaceOptions{IgnoreCase: true}); err != nil {
		fmt.Println(err)
		return
	}

	for _, c := range s.All() {
		fmt.Println(c.Start, c.Text())
	}
	// Output:
	// 4s <i>Wait...</i> Tom &amp; Jerry's
}

func ExampleSubtitle_StripFormatting() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{`<v Fred><b><i>Run</i></b>, <font color="#ff0000">now</font> <00:00:02.000>&amp; hide!`}})

	s.StripFormatting(model.StripOptions{KeepTags: []string{"i"}})

	fmt.Println(s.Cues()[0].Text())
	// Output:
	// <i>Run</i>, now <00:00:02.000>&amp; hide!
}

func ExampleSubtitle_MapText() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"<i>Fish &amp; chips</i> for two"}})

	s.MapText(func(text string, _ model.Cue) string {
		return strings.ReplaceAll(text, "&", "and")
	})

	fmt.Println(s.Cues()[0].Text())
	// Output:
	// <i>Fish and chips</i> for two
}

func ExampleSubtitle_MapLines() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"Hello", "<i>world</i>"}})

	s.MapLines(func(line string, _ model.Cue) string {
		return `<font color="#ffff00">` + line + "</font>"
	})

	fmt.Println(s.Cues()[0].Text())
	// Output:
	// <font color="#ffff00">Hello</font>
	// <font color="#ffff00"><i>world</i></font>
}

func ExampleSubtitle_FixRightToLeft() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"<i>!\u05e9\u05dc\u05d5\u05dd</i>", "Hello!"}})

	s.FixRightToLeft()
	fmt.Printf("%+q\n", s.Cues()[0].Lines)

	s.RemoveBidiControls()
	fmt.Printf("%+q\n", s.Cues()[0].Lines)
	// Output:
	// ["<i>\u202b\u05e9\u05dc\u05d5\u05dd!\u202c</i>" "Hello!"]
	// ["<i>\u05e9\u05dc\u05d5\u05dd!</i>" "Hello!"]
}

func ExampleSubtitle_EditLines() {
	var s model.Subtitle
	s.AddCues(
		model.Cue{Start: 1 * time.Second, End: 2 * time.Second, Lines: []string{"Hi"}},
		model.Cue{Start: 2 * time.Second, End: 3 * time.Second, Lines: []string{"(MUSIC)"}},
		model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"Bye"}},
	)
	_ = s.AddComment("Scene 2", 1)

	removed := s.EditLines(func(_ int, c model.Cue) ([]string, bool) {
		if strings.HasPrefix(c.Text(), "(") {
			return nil, true
		}
		return nil, false
	}, nil)

	fmt.Println(removed, s.Len(), s.Comments())
	// Output:
	// [1] 2 [{Scene 2 1}]
}
