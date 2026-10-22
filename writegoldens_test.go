package stbox_test

import (
	"bytes"
	"flag"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

var update = flag.Bool("update", false, "rewrite the write golden files")

// roundTripFolders holds the folders whose every real file is a round trip case.
func roundTripFolders() []string {
	return []string{"ass", "microdvd", "sami", "srt", "vtt"}
}

// roundTripFile is a round trip case in a folder that roundTripFolders does not hold. Its golden is
// expected/<file>.out.<extension of the file>, as in PHP.
type roundTripFile struct {
	folder, file string
}

func roundTripFiles() []roundTripFile {
	return []roundTripFile{
		{"ttml", "bbc_ebu_tt_d.ttml"},
		{"ttml", "mantas_netflix_ticks.dfxp"},
		{"ttml", "w3c_imsc11_regions.ttml"},
	}
}

// conversion is a write golden case that reads a real file and writes it in another format. A zero class reads
// with the read options of the read golden table.
type conversion struct {
	folder, file string
	to           stbox.Format
	class        string
}

func conversions() []conversion {
	return []conversion{
		{"ass", "own_aegisub.ass", stbox.SubRip, ""},
		{"ass", "own_aegisub.ass", stbox.WebVTT, ""},
		{"ass", "own_signs_crlf.ass", stbox.SubRip, ""},
		{"ass", "own_signs_crlf.ass", stbox.WebVTT, ""},
		{"sami", "multi_language.smi", stbox.SubRip, "ENCC"},
		{"sami", "multi_language.smi", stbox.WebVTT, "ENCC"},
		{"sami", "multi_language.smi", stbox.ASS, "ENCC"},
		{"srt", "own_styled.srt", stbox.SAMI, ""},
		{"srt", "own_styled.srt", stbox.ASS, ""},
		{"srt", "own_styled.srt", stbox.TTML, ""},
		{"srt", "own_styled.srt", stbox.WebVTT, ""},
		{"vtt", "w3c_voices.vtt", stbox.ASS, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.SubRip, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.WebVTT, ""},
		{"ttml", "bbc_ebu_tt_d.ttml", stbox.ASS, ""},
		{"vtt", "w3c_voices.vtt", stbox.SubRip, ""},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.ASS, ""},
		{"vtt", "own_ytdlp_rolling.vtt", stbox.SubRip, ""},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.ASS, ""},
		{"vtt", "own_openai_whisper_highlight_words.vtt", stbox.SubRip, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.SubRip, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.WebVTT, ""},
		{"microdvd", "sub_microdvd_with_styles.sub", stbox.ASS, ""},
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
	for _, r := range roundTripFiles() {
		t.Run(r.folder+"/"+r.file+".out", func(t *testing.T) {
			t.Parallel()

			g := folder(t, r.folder)
			s, _ := g.read(t, r.file)
			golden.Equal(t, testfiles.Path(t, goldenPath(g, r.file, ".out"+path.Ext(r.file))), writeAs(t, s, g.format), *update)
		})
	}
	for _, c := range conversions() {
		t.Run(c.folder+"/"+c.file+".to."+string(c.to), func(t *testing.T) {
			t.Parallel()

			g := folder(t, c.folder)
			opts := g.options[c.file]
			if c.class != "" {
				opts.SAMI.LanguageClass = c.class
			}
			if c.folder == "microdvd" {
				// The PHP write goldens read MicroDVD at 23.976 fps, the read goldens at 25 fps.
				opts.MicroDVD.FrameRate = 23.976
			}
			s, _ := g.readWith(t, c.file, opts)
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
