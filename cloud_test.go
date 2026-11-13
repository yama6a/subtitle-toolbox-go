package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// testCloudRealFiles ports testRealFileParses and testRealFileKeepsItsWordTimestampsAndSpeakersThroughWebVtt of
// tests/Parsers/CloudSpeechRealFilesTest.php of subtitle-toolbox-php. The read goldens hold the cues.
func testCloudRealFiles(t *testing.T, name string) {
	t.Helper()

	g := folder(t, name)
	for _, file := range g.realFiles(t) {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			_, ok := stbox.Detect(testfiles.Read(t, name+"/real/"+file))
			assert.False(t, ok, "detection never finds this format")

			s, _ := g.read(t, file)
			vtt, _, err := parseBytes(writeAs(t, s, stbox.WebVTT), stbox.ReadOptions{Format: stbox.WebVTT})
			require.NoError(t, err)
			require.Equal(t, s.Len(), vtt.Len())
			assert.Equal(t, taggedLines(s), taggedLines(vtt))
			assert.Contains(t, taggedLines(s)[0][0], "<00:00:0")
		})
	}
}
