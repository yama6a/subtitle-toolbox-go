package orderedjson_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

const benchBytes = 1 << 20

func BenchmarkDecode(b *testing.B) {
	inputs := map[string]string{
		"words": "[" + strings.Repeat(`{"word": " The", "start": 0.0, "end": 0.24, "probability": 0.991234540939331},`, benchBytes/80) + "null]",
		"keys":  manyKeys(benchBytes / 12),
		"text":  `"` + strings.Repeat(`a\u00e9\n`, benchBytes/8) + `"`,
	}
	for name, in := range inputs {
		data := []byte(in)
		b.Run("input="+name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := orderedjson.Decode(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func manyKeys(n int) string {
	var s strings.Builder
	s.WriteString("{")
	for i := range n {
		s.WriteString(`"k` + strconv.Itoa(i) + `":1,`)
	}
	s.WriteString(`"end":1}`)
	return s.String()
}

func BenchmarkDuration(b *testing.B) {
	v, err := orderedjson.NumberText("0." + strings.Repeat("1", benchBytes))
	if err != nil {
		b.Fatal(err)
	}
	huge, err := orderedjson.NumberText("1" + strings.Repeat("0", benchBytes))
	if err != nil {
		b.Fatal(err)
	}
	for name, n := range map[string]orderedjson.Value{"digits=1MB": v, "integer digits=1MB": huge} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				n.Duration(time.Second)
			}
		})
	}
}

func BenchmarkUnion(b *testing.B) {
	n := benchBytes / 20
	ms := make([]orderedjson.Member, 0, n)
	for i := range n {
		ms = append(ms, orderedjson.Member{Key: "key" + strconv.Itoa(i)})
	}
	v := orderedjson.ObjectOf(ms...)
	b.Run("members="+strconv.Itoa(n), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			orderedjson.Union(v, v)
		}
	})
}
