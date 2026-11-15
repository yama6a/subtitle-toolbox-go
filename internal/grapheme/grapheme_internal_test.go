package grapheme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInvisible(t *testing.T) {
	t.Parallel()

	invisible := []rune{0xAD, 0x200B, 0x200E, 0x200F, 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069, 0xFEFF}
	visible := []rune{'a', ' ', 0x00A0, 0x200C, 0x200D, 0x2029, 0x202F, 0x2065, 0x206A, 0xFEFE, 0xFFFD}
	for _, r := range invisible {
		assert.True(t, isInvisible(r), "%U", r)
	}
	for _, r := range visible {
		assert.False(t, isInvisible(r), "%U", r)
	}
}
