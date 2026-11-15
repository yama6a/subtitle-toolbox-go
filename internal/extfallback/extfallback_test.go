package extfallback_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"film.srt", true},
		{"film.vtt", true},
		{"talk.html", true},
		{"talk.HTM", true},
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
		detected model.Format
		path     string
		want     model.Format
	}{
		{model.TTML, "film.itt", model.ITT},
		{model.TTML, "FILM.ITT", model.ITT},
		{model.TTML, "film.ttml", model.TTML},
		{model.TTML, "", model.TTML},
		{model.SubRip, "film.itt", model.SubRip},
	}
	for _, tt := range tests {
		t.Run(string(tt.detected)+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extfallback.Profile(tt.detected, tt.path))
		})
	}
}
