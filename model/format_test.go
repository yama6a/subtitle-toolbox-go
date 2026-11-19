package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestFormatFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format     model.Format
		extensions []string
		canRead    bool
		canWrite   bool
	}{
		{model.ASS, []string{"ass", "ssa"}, true, true},
		{model.CSV, []string{"csv"}, true, true},
		{model.JSON, []string{"json"}, true, true},
		{model.LRC, []string{"lrc"}, true, true},
		{model.MicroDVD, []string{"sub"}, true, true},
		{model.MPSub, []string{"mpsub"}, true, true},
		{model.SAMI, []string{"smi", "sami"}, true, true},
		{model.SBV, []string{"sbv"}, true, true},
		{model.SubRip, []string{"srt"}, true, true},
		{model.SubViewer, []string{"sub"}, true, true},
		{model.TTML, []string{"ttml", "dfxp", "xml"}, true, true},
		{model.TSV, []string{"tsv"}, true, true},
		{model.PlainText, []string{"txt"}, false, true},
		{model.MPL2, []string{"txt"}, true, true},
		{model.TMPlayer, []string{"txt"}, true, true},
		{model.QuickTimeText, []string{"txt"}, true, true},
		{model.WebVTT, []string{"vtt"}, true, true},
		{"foo", nil, false, false},
		{"", nil, false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.extensions, tt.format.Extensions())
			assert.Equal(t, tt.canRead, tt.format.CanRead())
			assert.Equal(t, tt.canWrite, tt.format.CanWrite())
		})
	}
}

func TestExtensionsReturnsACopy(t *testing.T) {
	t.Parallel()

	model.TTML.Extensions()[0] = "changed"
	assert.Equal(t, []string{"ttml", "dfxp", "xml"}, model.TTML.Extensions())
}
