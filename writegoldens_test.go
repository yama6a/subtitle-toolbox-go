package stbox_test

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// roundTripFolders holds the folders whose every real file is a round trip case.
func roundTripFolders() []string {
	return []string{"ass", "srt", "vtt"}
}

// conversion is a write golden case that reads a real file and writes it in another format.
type conversion struct {
	folder, file string
	to           stbox.Format
}

func conversions() []conversion {
	return []conversion{
		{"ass", "own_aegisub.ass", stbox.SubRip},
		{"ass", "own_aegisub.ass", stbox.WebVTT},
		{"ass", "own_signs_crlf.ass", stbox.SubRip},
		{"ass", "own_signs_crlf.ass", stbox.WebVTT},
		{"srt", "own_styled.srt", stbox.ASS},
		{"srt", "own_styled.srt", stbox.WebVTT},
		{"vtt", "w3c_voices.vtt", stbox.ASS},
		{"vtt", "w3c_voices.vtt", stbox.SubRip},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.ASS},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.SubRip},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.ASS},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.SubRip},
	}
}

func folder(t *testing.T, name string) goldenFolder {
	t.Helper()

	for _, g := range goldenFolders() {
		if g.name == name {
			return g
		}
	}
	t.Fatalf("the read golden table has no folder %q", name)
	return goldenFolder{}
}

func writeAs(t *testing.T, s *stbox.Subtitle, f stbox.Format) []byte {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, stbox.Write(&out, s, f, stbox.WriteOptions{}))
	return out.Bytes()
}

// TestWriteGoldens writes each case with the zero write options. A round trip matches the normalised input or its
// golden expected/<file>.out.<extension>. A conversion matches its golden expected/<file>.to.<format>.
func TestWriteGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range roundTripFolders() {
		g := folder(t, name)
		for _, file := range g.realFiles(t) {
			t.Run(g.name+"/"+file+".out", func(t *testing.T) {
				t.Parallel()

				s, _ := g.read(t, file)
				golden.RoundTrip(t, testfiles.Path(t, g.name+"/real/"+file),
					testfiles.Path(t, goldenPath(g, file, ".out."+g.format.Extensions()[0])), writeAs(t, s, g.format), *update)
			})
		}
	}
	for _, c := range conversions() {
		t.Run(c.folder+"/"+c.file+".to."+string(c.to), func(t *testing.T) {
			t.Parallel()

			g := folder(t, c.folder)
			s, _ := g.read(t, c.file)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, c.file, conversionSuffix(c.to))), writeAs(t, s, c.to), *update)
		})
	}
}

// conversionSuffix returns .to.<format>, plus .<extension> when the first extension of f differs from its name,
// for example .to.sami.smi.
func conversionSuffix(f stbox.Format) string {
	suffix := ".to." + string(f)
	if ext := f.Extensions()[0]; ext != string(f) {
		suffix += "." + ext
	}
	return suffix
}
