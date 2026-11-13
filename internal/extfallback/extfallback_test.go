package extfallback_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/extfallback"
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

func TestProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		detected, path, want string
	}{
		{"ttml", "film.itt", "itt"},
		{"ttml", "FILM.ITT", "itt"},
		{"ttml", "film.ttml", "ttml"},
		{"ttml", "", "ttml"},
		{"srt", "film.itt", "srt"},
	}
	for _, tt := range tests {
		t.Run(tt.detected+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extfallback.Profile(tt.detected, tt.path))
		})
	}
}
