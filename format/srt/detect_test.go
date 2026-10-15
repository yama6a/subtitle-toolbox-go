package srt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
)

// Rows of the SubRip signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"cue number and timing line", "1\n00:00:01,000 --> 00:00:04,000\nHello\n", true},
		{"spaces and a dot", "1 \n 0:00:01.5\t-->", true},
		{"no fraction after a cue number", "1\n00:00:01 --> 00:00:04\n", true},
		{"5 lines of text first", "a\nb\nc\nd\ne\n1\n00:00:01,000 --> 00:00:04,000\nHello\n", true},
		{"6 lines of text first", "a\nb\nc\nd\ne\nf\n1\n00:00:01,000 --> 00:00:04,000\nHello\n", false},
		{"a timing line alone", "00:00:01,000 --> 00:00:04,000\nHello\n", true},
		{"a timing line with 1 hour digit alone", "0:00:01,000 --> 0:00:04,000\n", false},
		{"headerless WebVTT", "00:00:01.000 --> 00:00:04.000\nHello\n", false},
		{"SBV", "0:00:01.500,0:00:04.000\nHello\n", false},
		{"a cue number without a timing line", "1\nHello\n", false},
		{"WebVTT", "WEBVTT\n\n00:00:01.000 --> 00:00:04.000\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, srt.Detect([]byte(tt.content)))
		})
	}
}
