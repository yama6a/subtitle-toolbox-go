package itt_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ExampleWrite() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataLanguage, "en")
	s.AddCue(model.Cue{Start: 1500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Hello", "<i>world</i>"}})
	if _, err := itt.Write(os.Stdout, s, model.CommonWriteOptions{}, itt.WriteOptions{FrameRate: model.FrameRate25}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <?xml version="1.0" encoding="UTF-8"?>
	// <tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en" ttp:timeBase="smpte" ttp:frameRate="25" ttp:frameRateMultiplier="1 1" ttp:dropMode="nonDrop">
	//   <head>
	//     <styling>
	//       <style xml:id="normal" tts:fontFamily="sansSerif" tts:fontWeight="normal" tts:fontStyle="normal" tts:color="white" tts:fontSize="100%"/>
	//     </styling>
	//     <layout>
	//       <region xml:id="top" tts:origin="0% 0%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="before"/>
	//       <region xml:id="bottom" tts:origin="0% 85%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="after"/>
	//     </layout>
	//   </head>
	//   <body style="normal">
	//     <div>
	//       <p begin="00:00:01:13" end="00:00:04:00" region="bottom">Hello<br/><span tts:fontStyle="italic">world</span></p>
	//     </div>
	//   </body>
	// </tt>
}

func ExampleRead() {
	content := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="999 1000">
  <body><div><p begin="00:00:01:12" end="00:00:04:00">Hello</p></div></body>
</tt>`
	s, _, err := itt.Read(strings.NewReader(content), model.CommonReadOptions{}, itt.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	data, _ := s.FormatData("itt")
	frameRate, _ := data.String("frameRate")
	multiplier, _ := data.String("frameRateMultiplier")
	fmt.Println(frameRate, multiplier)
	// Output:
	// 1.502s 4.004s [Hello]
	// 24 999 1000
}
