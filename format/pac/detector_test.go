package pac_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// TestDetectsTheCodePageThatWroteTheText writes each sample with its code page, then reads the file without a code
// page. The writer takes the code page from the stored format data.
func TestDetectsTheCodePageThatWroteTheText(t *testing.T) {
	t.Parallel()

	var samples []struct {
		Name     string   `json:"name"`
		CodePage string   `json:"codePage"`
		Lines    []string `json:"lines"`
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "pac-detector/detector_samples.json"), &samples))
	require.Len(t, samples, 21)
	for _, tt := range samples {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			fileData := model.NewData(model.Field{Key: "codePage", Value: tt.CodePage})
			s := subtitle(t, fileData, cue(0, time.Second, model.Position{}, tt.Lines...))
			back, warnings := read(t, write(t, s, model.CommonWriteOptions{}), false, pac.ReadOptions{})
			assert.Empty(t, warnings)
			data, _ := back.FormatData("pac")
			stored, _ := data.String("codePage")
			assert.Equal(t, tt.CodePage, stored)
		})
	}
}
