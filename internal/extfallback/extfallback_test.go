package extfallback_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/extfallback"
)

func TestAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"film.srt", true},
		{"film.vtt", true},
		{"noext", true},
		{"dir.json/film.srt", true},
		{"call.json", false},
		{"CALL.JSON", false},
		{"notes.txt", false},
		{"book.ffmeta", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extfallback.Allowed(tt.path))
		})
	}
}
