package dcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"Interop after the prolog", "<?xml version=\"1.0\"?>\n<!-- c -->\n<DCSubtitle Version=\"1.1\"></DCSubtitle>", true},
		{"SMPTE without prefix", "<SubtitleReel xmlns=\"x\"/>", true},
		{"SMPTE with prefix", "<dcst:SubtitleReel xmlns:dcst=\"x\"/>", true},
		{"root name continues", "<SubtitleReelX/>", false},
		{"other prefix chars", "<1:SubtitleReel/>", false},
		{"TTML", string(testfiles.Read(t, "ttml/real/bbc_ebu_tt_d.ttml")), false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, dcp.Detect([]byte(tt.content)))
		})
	}
}
