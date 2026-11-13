package arib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDigestVariations pins the hashes of the scaled variations of a 16 by 18 glyph with 2 levels to the values of
// the PHP reader.
func TestDigestVariations(t *testing.T) {
	t.Parallel()

	pixels := make([]byte, 36)
	for i := range pixels {
		pixels[i] = byte(i*37 + 11)
	}
	tests := []struct {
		variation int
		hash      string
		ok        bool
	}{
		{1, "e63e087a2adf04ffbb87af6abb9a35bd", true},
		{2, "73cca6257df9e84f6faf0ab3eabfadec", true},
		{3, "2d32dc21f53c2884be77d895f42e00ef", true},
		{4, "", false},
	}
	for _, tt := range tests {
		hash, ok := digest(pixels, 16, 18, 2, tt.variation)
		assert.Equal(t, tt.ok, ok, "variation %d", tt.variation)
		assert.Equal(t, tt.hash, hash, "variation %d", tt.variation)
	}
}
