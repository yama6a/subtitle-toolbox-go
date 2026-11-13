package stbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func cloudTaggedLines(s *stbox.Subtitle) [][]string {
	var out [][]string
	for _, c := range s.All() {
		out = append(out, c.TaggedLines())
	}
	return out
}

// testCloudWriteGoldens writes each real file of the cloud speech-to-text format f back, converts speakerFile to
// SubRip and WebVTT, and converts a SubRip and a WebVTT file to f. The write goldens are Go's own.
func testCloudWriteGoldens(t *testing.T, name string, f stbox.Format, speakerFile string) {
	t.Helper()

	g := folder(t, name)
	for _, file := range g.realFiles(t) {
		t.Run(file+".out", func(t *testing.T) {
			t.Parallel()

			s, _ := g.read(t, file)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, file, ".out.json")), writeAs(t, s, f), *update)
		})
	}
	for _, to := range []stbox.Format{stbox.SubRip, stbox.WebVTT} {
		t.Run("to "+string(to), func(t *testing.T) {
			t.Parallel()

			s, _ := g.readWith(t, speakerFile, stbox.ReadOptions{})
			golden.Equal(t, testfiles.Path(t, goldenPath(g, speakerFile, conversionSuffix(to))), writeAs(t, s, to), *update)
		})
	}
	for _, c := range []struct{ folder, file string }{{"srt", "own_styled.srt"}, {"vtt", "w3c_timestamps.vtt"}} {
		t.Run(c.file+" to "+string(f), func(t *testing.T) {
			t.Parallel()

			from := folder(t, c.folder)
			s, _ := from.readWith(t, c.file, transcriptGolden)
			golden.Equal(t, testfiles.Path(t, goldenPath(from, c.file, conversionSuffix(f))), writeAs(t, s, f), *update)
		})
	}
}

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
			assert.Equal(t, cloudTaggedLines(s), cloudTaggedLines(vtt))
			assert.Contains(t, cloudTaggedLines(s)[0][0], "<00:00:0")
		})
	}
}
