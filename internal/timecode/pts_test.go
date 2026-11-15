package timecode_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
)

func TestPTSMilliseconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ticks int64
		want  int64
	}{
		{0, 0},
		{44, 0},
		{45, 1},
		{135, 2},
		{-1, 0},
		{math.MaxInt64, math.MaxInt64 / 90},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, timecode.PTSMilliseconds(tt.ticks), "ticks %d", tt.ticks)
	}
}
