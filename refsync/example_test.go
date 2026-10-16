package refsync_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/refsync"
)

func ExampleApply() {
	at := func(start, end float64, text string) model.Cue {
		return model.Cue{
			Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)),
			Lines: []string{text},
		}
	}
	var english, german model.Subtitle
	english.AddCues(
		at(10, 12.5, "Where is the station?"),
		at(20, 23, "Turn left at the bakery."),
		at(31, 33, "Thank you."),
		at(45, 47.5, "The last train leaves at ten."),
	)
	german.AddCues(
		at(10.79, 13.188, "Wo ist der Bahnhof?"),
		at(20.381, 23.258, "Links an der Ecke."),
		at(30.93, 32.848, "Danke."),
		at(44.357, 46.755, "Der letzte Zug geht um zehn."),
	)

	r, err := refsync.Apply(&german, refsync.Options{Reference: &english})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("scale %.5f, offset %s, score %.2f\n", r.Scale, r.Offset, r.Score)
	for _, c := range german.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// scale 1.04271, offset -1.25s, score 1.00
	// 10.001s 12.501s Wo ist der Bahnhof?
	// 20.001s 23.001s Links an der Ecke.
	// 31.001s 33.001s Danke.
	// 45.001s 47.502s Der letzte Zug geht um zehn.
}
