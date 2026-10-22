package ass_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// Rows of the ASS signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"ASS", "[Script Info]\nTitle: Bakery\n", true},
		{"SSA", "[Script Info]\nScriptType: v4.00\n", true},
		{"lower case", "[script info]\n", true},
		{"spaces and tabs after the header", "[Script Info] \t\n", true},
		{"header only", "[Script Info]", true},
		{"text after the header", "[Script Info] v4\n", false},
		{"header on the second line", "; made by hand\n[Script Info]\n", false},
		{"events first", "[Events]\nFormat: Start, End, Text\n", false},
		{"space inside the brackets", "[ Script Info]\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ass.Detect([]byte(tt.content)))
		})
	}
}

func TestDetectRealFiles(t *testing.T) {
	t.Parallel()

	assert.True(t, ass.Detect(testfiles.Read(t, realDir+"/own_ffmpeg.ass")))
	assert.False(t, ass.Detect(testfiles.Read(t, "lenient/broken_events.ass")), "a file with a BOM before the header")
}
