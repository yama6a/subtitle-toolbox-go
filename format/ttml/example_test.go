package ttml_test

import (
	"fmt"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en">
  <head><layout><region xml:id="top" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/></layout></head>
  <body><div><p begin="00:00:01.000" end="00:00:03.000" region="top">Hello<br/><span tts:fontStyle="italic">world</span></p></div></body>
</tt>`
	s, warnings, err := ttml.Read(strings.NewReader(content), model.CommonReadOptions{}, ttml.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines, c.Alignment)
	}
	lang, _ := s.Metadata(model.MetadataLanguage)
	data, _ := s.FormatData("ttml")
	head, _ := data.String("head")
	fmt.Println(lang, len(warnings), "warnings")
	fmt.Println(head)
	// Output:
	// 1s 3s [Hello <i>world</i>] TopCenter
	// en 0 warnings
	// <head><layout><region xml:id="top" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/></layout></head>
}

func ExampleDetect() {
	fmt.Println(ttml.Detect([]byte("<?xml version=\"1.0\"?>\n<tt xmlns=\"http://www.w3.org/ns/ttml\">")))
	fmt.Println(ttml.Detect([]byte("<html><body>")))
	// Output:
	// true
	// false
}
