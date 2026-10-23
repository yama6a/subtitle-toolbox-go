package sami_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	content := `<SAMI>
<HEAD>
<TITLE>Station news</TITLE>
<STYLE TYPE="text/css"><!--
.ENUSCC { Name: English; lang: en-US; }
.FRFRCC { Name: French; lang: fr-FR; }
--></STYLE>
</HEAD>
<BODY>
<SYNC Start=1000><P Class=ENUSCC>The bridge is <b>open</b><br>again.<P Class=FRFRCC>Le pont est ouvert.
<SYNC Start=4000><P Class=ENUSCC>&nbsp;<P Class=FRFRCC>&nbsp;
</BODY>
</SAMI>`
	s, _, err := sami.Read(strings.NewReader(content), model.CommonReadOptions{}, sami.ReadOptions{LanguageClass: "frfrcc"})
	if err != nil {
		fmt.Println(err)
		return
	}
	language, _ := s.Metadata(model.MetadataLanguage)
	fmt.Println(language)
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	// Output:
	// fr-FR
	// 1s 4s [Le pont est ouvert.]
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.SetMetadata(model.MetadataTitle, "Station news")
	s.SetMetadata(model.MetadataLanguage, "en-US")
	s.AddCues(model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"The bridge is <b>open</b>", "again."}})
	if err := sami.Write(os.Stdout, s, model.CommonWriteOptions{}, sami.WriteOptions{}); err != nil {
		fmt.Println(err)
	}
	// Output:
	// <SAMI>
	// <HEAD>
	// <TITLE>Station news</TITLE>
	// <STYLE TYPE="text/css"><!--
	// P { font-family: Arial; text-align: center; }
	// .ENUSCC { Name: en-US; lang: en-US; }
	// --></STYLE>
	// </HEAD>
	// <BODY>
	// <SYNC Start=1000><P Class=ENUSCC>The bridge is <b>open</b><br>again.
	// <SYNC Start=4000><P Class=ENUSCC>&nbsp;
	// </BODY>
	// </SAMI>
}

func ExampleDetect() {
	fmt.Println(sami.Detect([]byte("<SAMI>\n<HEAD>\n")))
	fmt.Println(sami.Detect([]byte("WEBVTT\n")))
	// Output:
	// true
	// false
}

func ExampleReadOptions_Validate() {
	fmt.Println(sami.ReadOptions{LanguageClass: " "}.Validate())
	// Output:
	// stbox: invalid argument: ReadOptions.SAMI.LanguageClass: The language class must not be empty.
}
