package stbox_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/imaging"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
)

// imageGolden is a row of the pixel golden table. The golden JSON and the PNG of its first cue lie next to the fixture
// in an expected folder. The PHP goldens are the oracle, so no tool rewrites them.
type imageGolden struct {
	// fixture is the path under testdata that Load reads.
	fixture string
	options stbox.ReadOptions
	// golden is the path under testdata of the golden files without their extension, such as "pgs/expected/shapes_1080p.sup".
	golden string
}

// imageGoldens joins the rows of the readers. Each reader has its own slice, so parallel changes do not touch the same line.
func imageGoldens() []imageGolden {
	return slices.Concat(pgsImageGoldens, vobsubImageGoldens, dvbImageGoldens)
}

// pgsImageGoldens lists one row for each PGS fixture.
var pgsImageGoldens = []imageGolden{
	{fixture: "pgs/shapes_1080p.sup", golden: "pgs/expected/shapes_1080p.sup"},
	{fixture: "pgs/shapes_576p.sup", golden: "pgs/expected/shapes_576p.sup"},
	{fixture: "pgs/text_1080p.sup", golden: "pgs/expected/text_1080p.sup"},
	{fixture: "pgs/text_cyrillic_1080p.sup", golden: "pgs/expected/text_cyrillic_1080p.sup"},
	{fixture: "pgs/authoring_layout_1080p.sup", golden: "pgs/expected/authoring_layout_1080p.sup"},
	{fixture: "pgs/fade_layout_1080p.sup", golden: "pgs/expected/fade_layout_1080p.sup"},
}

// vobsubImageGoldens lists one row for each VobSub track.
var vobsubImageGoldens = []imageGolden{}

// dvbImageGoldens lists one row for each DVB subtitle track of a transport stream or an MKV file.
var dvbImageGoldens = []imageGolden{
	{fixture: "mpegts/dvb_subtitles.ts", options: stbox.ReadOptions{Track: 257}, golden: "mpegts/expected/dvb_subtitles.ts.257"},
	{fixture: "mpegts/dvb_subtitles.ts", options: stbox.ReadOptions{Track: 258}, golden: "mpegts/expected/dvb_subtitles.ts.258"},
	{fixture: "mkv/dvbsub.mkv", options: stbox.ReadOptions{Track: 2}, golden: "mkv/expected/dvbsub.mkv.2"},
	{fixture: "mkv/dvbsub.mkv", options: stbox.ReadOptions{Track: 3}, golden: "mkv/expected/dvbsub.mkv.3"},
}

// imageGoldenDirs are the folders under testdata whose expected subfolder holds pixel goldens.
var imageGoldenDirs = []string{"pgs", "vobsub", "mpegts", "mkv"}

// imageGoldenCue is one cue of a pixel golden. The field order is the key order of the PHP goldens.
type imageGoldenCue struct {
	Start        float64 `json:"start"`
	End          float64 `json:"end"`
	X            int     `json:"x"`
	Y            int     `json:"y"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	ScreenWidth  int     `json:"screenWidth"`
	ScreenHeight int     `json:"screenHeight"`
	Forced       bool    `json:"forced"`
	Alignment    string  `json:"alignment"`
	RGBASha256   string  `json:"rgbaSha256"`
}

// imageGoldenCues returns the golden entry of each cue of s.
func imageGoldenCues(t *testing.T, s *stbox.Subtitle) []imageGoldenCue {
	t.Helper()

	cues := make([]imageGoldenCue, 0, s.Len())
	for _, c := range s.All() {
		require.NotNil(t, c.Image)
		img, err := imaging.Decode(*c.Image)
		require.NoError(t, err)
		cues = append(cues, imageGoldenCue{
			Start:        timecode.FloatSeconds(c.Start),
			End:          timecode.FloatSeconds(c.End),
			X:            c.Image.X,
			Y:            c.Image.Y,
			Width:        c.Image.Width,
			Height:       c.Image.Height,
			ScreenWidth:  c.Image.ScreenWidth,
			ScreenHeight: c.Image.ScreenHeight,
			Forced:       c.Forced,
			Alignment:    c.Position.Alignment.String(),
			RGBASha256:   rgbaHash(img),
		})
	}
	return cues
}

// rgbaHash returns the hex SHA-256 of the pixels of img, 4 bytes R, G, B, A each, row by row.
func rgbaHash(img *image.NRGBA) string {
	h := sha256.New()
	for y := range img.Rect.Dy() {
		h.Write(img.Pix[y*img.Stride : y*img.Stride+4*img.Rect.Dx()])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g imageGolden) expected(t *testing.T) []imageGoldenCue {
	t.Helper()

	var cues []imageGoldenCue
	require.NoError(t, json.Unmarshal(testfiles.Read(t, g.golden+".json"), &cues))
	require.NotEmpty(t, cues)
	return cues
}

func TestImageGoldens(t *testing.T) {
	t.Parallel()

	for _, g := range imageGoldens() {
		t.Run(g.golden, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, g.fixture), g.options)
			require.NoError(t, err)
			assert.Equal(t, canonicalJSON(t, g.expected(t)), canonicalJSON(t, imageGoldenCues(t, s)))
		})
	}
}

func TestImageGoldenPNGShowsThePixelsOfTheFirstCue(t *testing.T) {
	t.Parallel()

	for _, g := range imageGoldens() {
		t.Run(g.golden, func(t *testing.T) {
			t.Parallel()

			first := g.expected(t)[0]
			img, err := imaging.DecodePNG(testfiles.Read(t, g.golden+".png"))
			require.NoError(t, err)
			assert.Equal(t, [3]any{first.Width, first.Height, first.RGBASha256}, [3]any{img.Rect.Dx(), img.Rect.Dy(), rgbaHash(img)})
		})
	}
}

func TestEveryImageGoldenFileHasACase(t *testing.T) {
	t.Parallel()

	allowed := map[string]bool{}
	for _, g := range imageGoldens() {
		allowed[g.golden+".json"], allowed[g.golden+".png"] = true, true
		if strings.HasPrefix(g.golden, "vobsub/") {
			allowed[g.golden+".to.sup"] = true
		}
	}
	for _, dir := range imageGoldenDirs {
		files, err := filepath.Glob(testfiles.Path(t, dir+"/expected/*"))
		require.NoError(t, err)
		for _, f := range files {
			rel := dir + "/expected/" + filepath.Base(f)
			if ext := path.Ext(rel); (dir == "mpegts" || dir == "mkv") && ext != ".json" && ext != ".png" {
				continue
			}
			assert.True(t, allowed[rel], "%s has no row in imageGoldens", rel)
		}
	}
}

func TestImageGoldenCuesDescribeABuiltImageCue(t *testing.T) {
	t.Parallel()

	im, err := imaging.NewImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)), 10, 20, 1920, 1080)
	require.NoError(t, err)
	s := &stbox.Subtitle{}
	s.AddCue(stbox.Cue{Start: 1500 * time.Millisecond, End: 2 * time.Second, Image: &im, Forced: true, Position: stbox.Position{Alignment: stbox.TopCenter}})

	assert.Equal(t, []imageGoldenCue{{
		Start: 1.5, End: 2, X: 10, Y: 20, Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080, Forced: true, Alignment: "TopCenter",
		RGBASha256: "df3f619804a92fdb4057192dc43dd748ea778adc52bc498ce80524c014b81119",
	}}, imageGoldenCues(t, s))
}
