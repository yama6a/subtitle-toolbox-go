package orderedjson_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

func ExampleDecode() {
	v, err := orderedjson.Decode([]byte(`{"word": " The", "start": 0.0, "end": 0.24}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	start, _ := v.Get("start")
	d, _ := start.Duration(time.Second)
	out, _ := orderedjson.Encode(v, false)
	fmt.Println(d, string(out))
	// Output: 0s {"word":" The","start":0.0,"end":0.24}
}
