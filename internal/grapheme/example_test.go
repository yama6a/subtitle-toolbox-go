package grapheme_test

import (
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
)

func ExampleCount() {
	fmt.Println(grapheme.Count("Cafe\u0301"), grapheme.Count("\U0001f44d\U0001f3fd"), grapheme.Count("\u200e\ufeff"))
	// Output: 4 1 0
}

func ExampleClusters() {
	for c := range grapheme.Clusters("e\u0301a") {
		fmt.Printf("%U ", []rune(c))
	}
	// Output: [U+0065 U+0301] [U+0061]
}
