package csv_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func ExampleRead() {
	table := "start,end,speaker,text\n" +
		"00:00:01.000,00:00:04.000,Anna,Where are you going?\n" +
		"00:00:04.500,00:00:06.000,Ben,\"Home.\nNow.\"\n"
	s, warnings, err := csv.Read(strings.NewReader(table), model.CommonReadOptions{}, csv.ReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Lines)
	}
	fmt.Println(len(warnings), "warnings")
	// Output:
	// 1s 4s [<v Anna>Where are you going?]
	// 4.5s 6s [<v Ben>Home. Now.]
	// 0 warnings
}

func ExampleWrite() {
	english, german := &model.Subtitle{}, &model.Subtitle{}
	english.AddCues(
		model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<v Anna>Where are you going?"}},
		model.Cue{Start: 4500 * time.Millisecond, End: 6 * time.Second, Lines: []string{"Home."}},
	)
	german.AddCues(
		model.Cue{Start: 1200 * time.Millisecond, End: 3900 * time.Millisecond, Lines: []string{"Wohin gehst du?"}},
		model.Cue{Start: 4400 * time.Millisecond, End: 6 * time.Second, Lines: []string{"Nach Hause."}},
	)
	opts := csv.WriteOptions{SecondText: german, SecondTextHeader: "text (de)", TimeFormat: csv.TimeSeconds}
	err := csv.Write(os.Stdout, english, model.CommonWriteOptions{BOM: model.BOMRemove}, opts)
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// start,end,speaker,text,text (de)
	// 1,4,Anna,Where are you going?,Wohin gehst du?
	// 4.5,6,,Home.,Nach Hause.
}
