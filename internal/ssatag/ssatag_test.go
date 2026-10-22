package ssatag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/ssatag"
)

// Ports testLegacyAlignmentTagsAreConverted and testInvalidLegacyAlignmentStaysInTheText
// of tests/Parsers/SubRipParserTest.php of subtitle-toolbox-php.
func TestAlignment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag    string
		want   int
		wantOK bool
	}{
		{`\an1`, 1, true},
		{`\an5`, 5, true},
		{`\an9`, 9, true},
		{`\an0`, 0, false},
		{`\an10`, 0, false},
		{`\a1`, 1, true},
		{`\a2`, 2, true},
		{`\a3`, 3, true},
		{`\a5`, 7, true},
		{`\a6`, 8, true},
		{`\a7`, 9, true},
		{`\a9`, 4, true},
		{`\a10`, 5, true},
		{`\a11`, 6, true},
		{`\a4`, 0, false},
		{`\a8`, 0, false},
		{`\a12`, 0, false},
		{`\a012`, 0, false},
		{`\a06`, 8, true},
		{"\\an8\n", 8, true},
		{"\\an8\n\n", 0, false},
		{`\an8 `, 0, false},
		{`\b1`, 0, false},
		{`an8`, 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			got, ok := ssatag.Alignment(tt.tag)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLegacyCode(t *testing.T) {
	t.Parallel()

	for n := 1; n <= 9; n++ {
		got, ok := ssatag.Alignment(`\a` + ssatag.LegacyCode(n))
		assert.True(t, ok, n)
		assert.Equal(t, n, got)
	}
	assert.Equal(t, "6", ssatag.LegacyCode(8))
	assert.Empty(t, ssatag.LegacyCode(0))
	assert.Empty(t, ssatag.LegacyCode(10))
}
