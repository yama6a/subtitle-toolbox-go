package model_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// nestedData returns an object with the given number of keys. Each key holds a list nested depth deep.
func nestedData(chains, depth int) *model.Data {
	d := model.NewData()
	for i := range chains {
		var v any = []any{}
		for range depth - 1 {
			v = []any{v}
		}
		d.Set("k"+string(rune('a'+i%26))+string(rune('a'+i/26%26))+string(rune('a'+i/676)), v)
	}
	return d
}

// nestedDataOps are the model calls that walk all of a nested value.
var nestedDataOps = []struct {
	name string
	run  func(tb testing.TB, d *model.Data)
}{
	{"Clone", func(_ testing.TB, d *model.Data) { _ = d.Clone() }},
	{"Subtitle.SetFormatData", func(tb testing.TB, d *model.Data) {
		tb.Helper()

		var s model.Subtitle
		require.NoError(tb, s.SetFormatData("json", d))
		_, _ = s.FormatData("json")
	}},
	{"Cue.SetFormatData", func(tb testing.TB, d *model.Data) {
		tb.Helper()

		var c model.Cue
		require.NoError(tb, c.SetFormatData("json", d))
		_, _ = c.FormatData("json")
	}},
}

// allocatedBytes returns the bytes that one call of run allocates. Callers must not run in parallel.
func allocatedBytes(run func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestNestedDataGrowth checks that the bytes allocated by each operation grow by less than 3 times
// when the nesting depth doubles. Linear growth gives 2 and quadratic growth gives 4.
//
//nolint:paralleltest // the allocation count of a parallel test would include other tests
func TestNestedDataGrowth(t *testing.T) {
	for _, op := range nestedDataOps {
		t.Run(op.name, func(t *testing.T) { //nolint:paralleltest // see above
			small, large := nestedData(20, 250), nestedData(20, 500)
			bytes := func(d *model.Data) uint64 { return allocatedBytes(func() { op.run(t, d) }) }
			assert.Less(t, bytes(large), 3*bytes(small))
		})
	}
}

func BenchmarkNestedData(b *testing.B) {
	// 1000 lists nested 500 deep are 1 MB of JSON.
	d := nestedData(1000, 500)
	for _, op := range nestedDataOps {
		b.Run(op.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				op.run(b, d)
			}
		})
	}
}
