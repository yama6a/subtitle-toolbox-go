package mcc_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func ExampleRead() {
	content := "File Format=MacCaption_MCC V2.0\nTime Code Rate=25\n\n" +
		"00:00:00:16\tT55S553F43ZZ72F8FC94AEROOJ74ZZ95B7\n" +
		"00:00:00:17\tT55S553F43Z0172F8FC94AEROOJ74Z0193B7\n" +
		"00:00:00:18\tT55S553F43Z0272F8FC9420FC9420ROOI74Z0269B7\n" +
		"00:00:00:19\tT55S553F43Z0372F8FC9476ROOJ74Z03C7B7\n" +
		"00:00:00:20\tT55S553F43Z0472F8FC9476ROOJ74Z04C5B7\n" +
		"00:00:00:21\tT55S553F43Z0572F8FC97A1ROOJ74Z0595B7\n" +
		"00:00:00:22\tT55S553F43Z0672F8FC97A1ROOJ74Z0693B7\n" +
		"00:00:00:23\tT55S553F43Z0772F8FCC8E5FCECECROOI74Z0742B7\n" +
		"00:00:00:24\tT55S553F43Z0872F8FCEFA1RFF103EFE8C01FE9818FEDA32FE7005FE0997FEC0ZFE0EZFE92ZFEZ90FE01ZFE913FFEZZFE4865FE6C6CFE6F21L74Z0874B7\n" +
		"00:00:01:00\tT55S553F43Z0972F8FC942FRFF4222FE8901OOH74Z090BB7\n" +
		"00:00:01:01\tT55S553F43Z0A72F8FC942FROOJ74Z0AZB7\n" +
		"00:00:03:00\tT55S553F43Z0B72F8FC942CRFFC222FE8C01OOH74Z0B87B7\n" +
		"00:00:03:01\tT55S553F43Z0C72F8FC942CROOJ74Z0CFFB7\n" +
		""
	s, warnings, err := mcc.Read(strings.NewReader(content), model.CommonReadOptions{}, mcc.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.Cues() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1s 3s [Hello!]
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"Hello!"}})
	var out strings.Builder
	report, err := mcc.Write(&out, s, model.CommonWriteOptions{}, mcc.WriteOptions{FrameRate: model.FrameRate25})
	if err != nil {
		fmt.Println(err)
		return
	}
	lines := strings.Split(out.String(), "\n")
	fmt.Println(lines[0])
	fmt.Println(strings.Contains(out.String(), "\nTime Code Rate=25\n"))
	fmt.Println(len(report.Changes), "changes")
	// Output:
	// File Format=MacCaption_MCC V2.0
	// true
	// 0 changes
}

func ExampleDetect() {
	fmt.Println(mcc.Detect([]byte("File Format=MacCaption_MCC V2.0\n")))
	fmt.Println(mcc.Detect([]byte("WEBVTT\n")))
	// Output:
	// true
	// false
}
