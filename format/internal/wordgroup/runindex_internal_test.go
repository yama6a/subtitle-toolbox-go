package wordgroup

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func scanFirst(text string, source []SourceWord, cands []int, next int) (first, end int, ok bool) {
	for _, i := range cands {
		if i < next {
			continue
		}
		if e, match := runMatch(text, source, i); match {
			return i, e, true
		}
	}
	return 0, 0, false
}

func TestIndexesFindWhatAScanFinds(t *testing.T) {
	t.Parallel()
	texts := []string{"a", "b", "a.", ".", "ab", "", "a!?", "\u3002"}
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed makes the test repeatable
	for range 3000 {
		source := make([]SourceWord, rng.IntN(8))
		for i := range source {
			source[i].Text = texts[rng.IntN(len(texts))]
			if rng.IntN(2) == 0 {
				d := time.Duration(rng.IntN(2)) * time.Second
				source[i].Start = &d
			}
		}
		var timed, untimed []int
		for i, s := range source {
			if s.Start == nil {
				untimed = append(untimed, i)
			} else if *s.Start == 0 {
				timed = append(timed, i)
			}
		}
		parts := make([]string, rng.IntN(5)+1)
		for k := range parts {
			parts[k] = texts[rng.IntN(len(texts))]
		}
		text := strings.Join(parts, " ")
		if rng.IntN(2) == 0 {
			text += ".."
		}
		if strings.Trim(text, " ") == "" {
			continue
		}
		next := rng.IntN(len(source) + 1)

		f1, e1, ok1 := scanFirst(text, source, timed, next)
		f2, e2, ok2 := newRunIndex(source).find(text, 0, source, next)
		assert.Equal(t, []any{f1, e1, ok1}, []any{f2, e2, ok2}, "timed %q %v", text, source)

		f1, e1, ok1 = scanFirst(text, source, untimed, next)
		f2, e2, ok2 = newUntimedIndex(source).find(text, source, next)
		assert.Equal(t, []any{f1, e1, ok1}, []any{f2, e2, ok2}, "untimed %q %v", text, source)
	}
}
