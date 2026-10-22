package phpkey_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/phpkey"
)

func TestIsIntKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"", false},
		{"0", true},
		{"-0", false},
		{"00", false},
		{"01", false},
		{"-", false},
		{"+1", false},
		{"1", true},
		{"-1", true},
		{"-01", false},
		{"9223372036854775807", true},
		{"9223372036854775808", false},
		{"-9223372036854775808", true},
		{"-9223372036854775809", false},
		{" 1", false},
		{"1 ", false},
		{"abc", false},
		{"1a", false},
		{"1_000", false},
		{"0x10", false},
		{"sectionOrder", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, phpkey.IsIntKey(tt.key))
		})
	}
}

func BenchmarkIsIntKey(b *testing.B) {
	keys := []string{"sectionOrder", "scriptInfo", "12345", "9223372036854775808"}
	for _, key := range keys {
		b.Run("key="+key, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				phpkey.IsIntKey(key)
			}
		})
	}
}
