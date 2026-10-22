package microdvd_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// Rows of the MicroDVD signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"cue", "{24}{72}Hello\n", true},
		{"frame rate line", "{1}{1}25\n{24}{72}Hello\n", true},
		{"empty end frame", "{24}{}Hello\n", true},
		{"pipe lines", "{10}{25}Hello|world\n", true},
		{"MPL2 brackets", "[10][25]Hello\n", false},
		{"letter as frame", "{x}{72}Hello\n", false},
		{"no second frame", "{24}Hello\n", false},
		{"text before the cue", "Movie\n{24}{72}Hello\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, microdvd.Detect([]byte(tt.content)))
		})
	}
}

func TestDetectRealFiles(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(testfiles.Path(t, realDir))
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "SOURCES.md" {
			assert.True(t, microdvd.Detect(testfiles.Read(t, realDir+"/"+e.Name())), e.Name())
		}
	}
}
