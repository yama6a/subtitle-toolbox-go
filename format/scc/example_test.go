package scc_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func ExampleRead() {
	content := "Scenarist_SCC V1.0\n\n" +
		"00:00:01;12\t9420 9420 94ae 94ae 9452 9452 97a1 97a1 c8e5 ecec efa1 942f 942f\n\n" +
		"00:00:04;00\t942c 942c\n"
	s, warnings, err := scc.Read(strings.NewReader(content), model.CommonReadOptions{}, scc.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.Cues() {
		fmt.Println(c.Start, c.End, c.Lines, c.Position.Alignment, c.Position.Row, c.Position.Column)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1.768s 4.004s [Hello!] BottomCenter 14 6
	// 0 warnings
}

func ExampleWrite() {
	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"Hello!"}})
	report, err := scc.Write(os.Stdout, s, model.CommonWriteOptions{}, scc.WriteOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(report.Changes), "changes")
	// Output:
	// Scenarist_SCC V1.0
	//
	// 00:00:00;19	94ae 94ae 9420 9420 9476 9476 97a1 97a1 c8e5 ecec efa1 942f 942f
	//
	// 00:00:03;00	942c 942c
	// 0 changes
}

func ExampleDetect() {
	fmt.Println(scc.Detect([]byte("Scenarist_SCC V1.0\n\n00:00:01:00\t942c 942c\n")))
	fmt.Println(scc.Detect([]byte("WEBVTT\n")))
	// Output:
	// true
	// false
}

func ExampleReadOptions_Validate() {
	fmt.Println(scc.ReadOptions{Channel: 3}.Validate())
	// Output:
	// stbox: invalid argument: ReadOptions.SCC.Channel: The SCC data channel must be 1 or 2, got 3.
}
