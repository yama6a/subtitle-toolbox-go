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

func ExampleApply_splits() {
	at := func(start, end float64, text string) model.Cue {
		return model.Cue{
			Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)),
			Lines: []string{text},
		}
	}
	var englishTV, german model.Subtitle
	englishTV.AddCues(
		at(10, 12.5, "Where is the station?"),
		at(20, 23, "Turn left at the bakery."),
		at(31, 33, "Thank you."),
		at(45, 47.5, "The last train leaves at ten."),
		at(210, 212.5, "Is this seat free?"),
		at(221, 224, "Yes, please sit down."),
		at(232, 234, "Where are you going?"),
		at(246, 248.5, "To my sister in the north."),
	)
	german.AddCues(
		at(12, 14.5, "Wo ist der Bahnhof?"),
		at(22, 25, "Links an der Ecke."),
		at(33, 35, "Danke."),
		at(47, 49.5, "Der letzte Zug geht um zehn."),
		at(62, 64.5, "Ist dieser Platz frei?"),
		at(73, 76, "Ja, bitte setzen Sie sich."),
		at(84, 86, "Wohin fahren Sie?"),
		at(98, 100.5, "Zu meiner Schwester im Norden."),
	)

	r, err := refsync.Apply(&german, refsync.Options{
		Reference: &englishTV,
		MinOffset: new(-180 * time.Second),
		MaxOffset: new(180 * time.Second), // the part after the break needs 148 s
		MaxSplits: 2,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, seg := range r.Segments {
		to := "nil"
		if seg.To != nil {
			to = seg.To.String()
		}
		fmt.Printf("from %s, to %s, scale %.5f, offset %s\n", seg.From, to, seg.Scale, seg.Offset)
	}
	fmt.Printf("offset %s, score %.2f\n", r.Offset, r.Score)
	// Output:
	// from 0s, to 1m2s, scale 1.00000, offset -2s
	// from 1m2s, to nil, scale 1.00000, offset 2m28s
	// offset -2s, score 1.00
}
