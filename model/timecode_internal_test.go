package model

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The int64 path of Frames and Duration must give the result of the big integer path, also near the overflow.
func TestFrameRateInt64PathMatchesBigPath(t *testing.T) {
	t.Parallel()

	rates := []FrameRate{FrameRate23976, FrameRate25, FrameRate2997, FrameRate5994}
	values := []int64{0, 1, 500, 1001, 1 << 30, 1 << 40, 1 << 50, 1 << 55, math.MaxInt64 / 1001, math.MaxInt64 / 1000, math.MaxInt64}
	for _, r := range rates {
		for _, v := range values {
			wantFrame := roundDiv(product(v, r.Num), product(r.Den, int64(time.Second)))
			assert.Equal(t, wantFrame, r.Frames(time.Duration(v)), "Frames %v %d", r, v)
			wantTime := time.Duration(roundDiv(product(v, r.Den, int64(time.Second)), product(r.Num)))
			assert.Equal(t, wantTime, r.Duration(v), "Duration %v %d", r, v)
		}
	}
}
