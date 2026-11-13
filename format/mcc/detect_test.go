package mcc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"version 2.0", "File Format=MacCaption_MCC V2.0\n\n", true},
		{"any version", "File Format=MacCaption_MCC V9.12 \t\n", true},
		{"header only", "File Format=MacCaption_MCC V1.0", true},
		{"no minor version", "File Format=MacCaption_MCC V2\n", false},
		{"text after the version", "File Format=MacCaption_MCC V2.0 x\n", false},
		{"header on the second line", "\nFile Format=MacCaption_MCC V2.0\n", false},
		{"SCC", "Scenarist_SCC V1.0\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mcc.Detect([]byte(tt.content)))
		})
	}
}
