package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestWarningActionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action model.WarningAction
		want   string
	}{
		{model.Skipped, "skipped"},
		{model.Repaired, "repaired"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.action.String())
		})
	}
}
