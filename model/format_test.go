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
		{model.ASS, []string{"ass", "ssa"}, false, false, false},
		{model.CSV, []string{"csv"}, false, false, false},
		{model.JSON, []string{"json"}, true, true, false},
		{model.LRC, []string{"lrc"}, false, false, false},
		{model.MicroDVD, []string{"sub"}, false, false, false},
		{model.MPSub, []string{"mpsub"}, false, false, false},
		{model.SAMI, []string{"smi", "sami"}, false, false, false},
		{model.SBV, []string{"sbv"}, false, false, false},
		{model.SubRip, []string{"srt"}, false, false, false},
		{model.SubViewer, []string{"sub"}, false, false, false},
		{model.TTML, []string{"ttml", "dfxp", "xml"}, false, false, false},
		{model.TSV, []string{"tsv"}, false, false, false},
		{model.PlainText, []string{"txt"}, false, false, true},
		{model.MPL2, []string{"txt"}, false, false, false},
		{model.TMPlayer, []string{"txt"}, false, false, false},
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
