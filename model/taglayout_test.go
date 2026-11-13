package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestTagLayout(t *testing.T) {
	t.Parallel()

	const moved = "the<00:00:01.000><c> train</c><c.red><00:00:02.000><c> leaves</c></c>"
	const rule = "<00:00:01.000>The <00:00:02.000>beach"
	tests := []struct {
		name      string
		lines     []string
		restore   []string
		wantOK    bool
		wantAfter []string
	}{
		{name: "tags where the rule does not put them", lines: []string{moved}, restore: []string{moved}, wantOK: true, wantAfter: []string{moved}},
		{name: "tags where the rule puts them", lines: []string{rule}, restore: []string{rule}, wantAfter: []string{rule}},
		{name: "layout of other words is ignored", lines: []string{moved}, restore: []string{rule}, wantOK: true, wantAfter: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{End: 5 * time.Second, Lines: tt.lines}
			c.ExtractWordTags()
			layout, ok := c.TagLayout()
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.lines, layout)
			}

			fresh := model.Cue{End: c.End, Lines: c.Lines, Words: c.Words}
			ruleLines := fresh.TaggedLines()
			fresh.SetTagLayout(tt.restore)
			if tt.wantAfter == nil {
				tt.wantAfter = ruleLines
			}
			assert.Equal(t, tt.wantAfter, fresh.TaggedLines())
		})
	}
}
