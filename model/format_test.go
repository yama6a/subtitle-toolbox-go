package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestFormatFacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format       model.Format
		extensions   []string
		canRead      bool
		canWrite     bool
		autoDetected bool
	}{
		{model.ASS, []string{"ass", "ssa"}, true, true, true},
		{model.CSV, []string{"csv"}, true, true, false},
		{model.JSON, []string{"json"}, true, true, true},
		{model.LRC, []string{"lrc"}, true, true, true},
		{model.MicroDVD, []string{"sub"}, true, true, true},
		{model.MPSub, []string{"mpsub"}, true, true, true},
		{model.SAMI, []string{"smi", "sami"}, true, true, true},
		{model.SBV, []string{"sbv"}, true, true, true},
		{model.SubRip, []string{"srt"}, true, true, true},
		{model.SubViewer, []string{"sub"}, true, true, true},
		{model.TTML, []string{"ttml", "dfxp", "xml"}, true, true, true},
		{model.TSV, []string{"tsv"}, true, true, false},
		{model.PlainText, []string{"txt"}, false, true, true},
		{model.MPL2, []string{"txt"}, true, true, true},
		{model.TMPlayer, []string{"txt"}, true, true, true},
		{model.QuickTimeText, []string{"txt"}, false, false, false},
		{model.WebVTT, []string{"vtt"}, true, true, true},
		{"foo", nil, false, false, false},
		{"", nil, false, false, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.extensions, tt.format.Extensions())
			assert.Equal(t, tt.canRead, tt.format.CanRead())
			assert.Equal(t, tt.canWrite, tt.format.CanWrite())
			assert.Equal(t, tt.autoDetected, tt.format.IsAutoDetected())
		})
	}
}

func TestExtensionsReturnsACopy(t *testing.T) {
	t.Parallel()

	model.TTML.Extensions()[0] = "changed"
	assert.Equal(t, []string{"ttml", "dfxp", "xml"}, model.TTML.Extensions())
}
