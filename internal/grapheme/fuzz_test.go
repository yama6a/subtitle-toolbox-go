package grapheme_test

import (
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
)

func FuzzCount(f *testing.F) {
	f.Add("Cafe\u0301")
	f.Add("\U0001f44d\U0001f3fd \U0001f468\u200d\U0001f469\u200d\U0001f467")
	f.Add("\u200e\u200f\u202a\u202d\ufeff\u00ad")
	f.Add("Caf\xe9")

	f.Fuzz(func(t *testing.T, s string) {
		n := grapheme.Count(s)
		if n < 0 || n > len(s) {
			t.Fatalf("Count(%q) = %d, want 0 to %d", s, n, len(s))
		}
		var joined strings.Builder
		for c := range grapheme.Clusters(s) {
			joined.WriteString(c)
		}
		if joined.String() != s {
			t.Fatalf("Clusters(%q) joins to %q", s, joined.String())
		}
	})
}
