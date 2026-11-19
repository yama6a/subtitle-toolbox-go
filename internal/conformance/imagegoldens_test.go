package conformance_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
)

// imageGolden is a row of the pixel golden table. The golden JSON and the PNG of its first cue lie next to the fixture
// in an expected folder. No tool rewrites them.
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
var vobsubImageGoldens = []imageGolden{
	vobsubGolden("two-tracks-pal", 0),
	vobsubGolden("two-tracks-pal", 1),
	vobsubGolden("custom-colors-ntsc", 0),
	vobsubGolden("custom-colors-ntsc", 2),
	vobsubGolden("text-pal", 0),
	vobsubGolden("one-packet-pal", 0),
}

// vobsubGolden returns the row of the track with the index: value index of the pair name. Load reads the .sub file
// next to the .idx file.
func vobsubGolden(name string, index int) imageGolden {
	return imageGolden{
		fixture: "vobsub/" + name + ".idx",
		options: stbox.ReadOptions{VobSub: vobsub.ReadOptions{Index: &index}},
		golden:  "vobsub/expected/" + name + ".idx." + strconv.Itoa(index),
	}
}

// dvbImageGoldens lists one row for each DVB subtitle track of a transport stream or an MKV file.
var dvbImageGoldens = []imageGolden{
	{fixture: "mpegts/dvb_subtitles.ts", options: stbox.ReadOptions{Track: 257}, golden: "mpegts/expected/dvb_subtitles.ts.257"},
	{fixture: "mpegts/dvb_subtitles.ts", options: stbox.ReadOptions{Track: 258}, golden: "mpegts/expected/dvb_subtitles.ts.258"},
	{fixture: "mkv/dvbsub.mkv", options: stbox.ReadOptions{Track: 2}, golden: "mkv/expected/dvbsub.mkv.2"},
	{fixture: "mkv/dvbsub.mkv", options: stbox.ReadOptions{Track: 3}, golden: "mkv/expected/dvbsub.mkv.3"},
}

// imageGoldenDirs are the folders under testdata whose expected subfolder holds pixel goldens.
var imageGoldenDirs = []string{"pgs", "vobsub", "mpegts", "mkv"}

// imageGoldenCue is one cue of a pixel golden.
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
		img, err := stbox.DecodeImage(*c.Image)
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
			img, err := stbox.DecodePNG(testfiles.Read(t, g.golden+".png"))
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

	im, err := stbox.NewImage(image.NewNRGBA(image.Rect(0, 0, 1, 1)), 10, 20, 1920, 1080)
	require.NoError(t, err)
	s := &stbox.Subtitle{}
	require.NoError(t, s.AddCue(stbox.Cue{Start: 1500 * time.Millisecond, End: 2 * time.Second, Image: &im, Forced: true, Position: stbox.Position{Alignment: stbox.TopCenter}}))

	assert.Equal(t, []imageGoldenCue{{
		Start: 1.5, End: 2, X: 10, Y: 20, Width: 1, Height: 1, ScreenWidth: 1920, ScreenHeight: 1080, Forced: true, Alignment: "TopCenter",
		RGBASha256: "df3f619804a92fdb4057192dc43dd748ea778adc52bc498ce80524c014b81119",
	}}, imageGoldenCues(t, s))
}

// TestVobSubToPGS writes each VobSub track as PGS and compares the file with its golden.
// Reading the result gives the same cues. Limited range YCrCb has no code for some RGB colours,
// so a colour channel can differ by 1. The alpha does not change.
func TestVobSubToPGS(t *testing.T) {
	t.Parallel()

	for _, g := range vobsubImageGoldens {
		t.Run(g.golden, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, g.fixture), g.options)
			require.NoError(t, err)
			var out bytes.Buffer
			_, err = stbox.Write(&out, s, stbox.PGS, stbox.WriteOptions{})
			require.NoError(t, err)
			testfiles.MatchGolden(t, testfiles.Path(t, g.golden+".to.sup"), out.Bytes(), *update)

			back, _, err := stbox.Parse(bytes.NewReader(out.Bytes()), int64(out.Len()), stbox.ReadOptions{Format: stbox.PGS})
			require.NoError(t, err)
			require.Equal(t, s.Len(), back.Len())
			for i, c := range s.Cues() {
				d, _ := back.Cue(i)
				assert.Equal(t, c.Start, d.Start, "start of cue %d", i)
				assert.Equal(t, c.End, d.End, "end of cue %d", i)
				assert.Equal(t, [6]int{c.Image.X, c.Image.Y, c.Image.Width, c.Image.Height, c.Image.ScreenWidth, c.Image.ScreenHeight},
					[6]int{d.Image.X, d.Image.Y, d.Image.Width, d.Image.Height, d.Image.ScreenWidth, d.Image.ScreenHeight}, "place of cue %d", i)
				want, err := stbox.DecodeImage(*c.Image)
				require.NoError(t, err)
				got, err := stbox.DecodeImage(*d.Image)
				require.NoError(t, err)
				for p := range want.Pix {
					delta := 1.0
					if p%4 == 3 {
						delta = 0
					}
					assert.InDelta(t, int(want.Pix[p]), int(got.Pix[p]), delta, "cue %d byte %d", i, p)
				}
			}
		})
	}
}

func imageCue(start, end time.Duration, lines ...string) stbox.Cue {
	c := stbox.Cue{Start: start, End: end, Lines: lines}
	c.Image = &stbox.Image{PNG: []byte("\x89PNG"), X: 1, Y: 2, Width: 3, Height: 4, ScreenWidth: 720, ScreenHeight: 576}
	return c
}

func writeBytes(t *testing.T, s *stbox.Subtitle, f stbox.Format, opts stbox.WriteOptions) ([]byte, error) {
	t.Helper()

	var b bytes.Buffer
	_, err := stbox.Write(&b, s, f, opts)
	return b.Bytes(), err
}

func subtitleOfCues(t *testing.T, comment int, cues ...stbox.Cue) *stbox.Subtitle {
	t.Helper()

	s := &stbox.Subtitle{}
	require.NoError(t, s.AddCues(cues...))
	require.NoError(t, s.AddComment("note", comment))
	// DCP writes the clock as IssueDate when none is set, so 2 writes that span a second boundary differ.
	header := stbox.NewData(stbox.Field{Key: "IssueDate", Value: "2026-10-12T00:00:00"})
	require.NoError(t, s.SetFormatData("dcp", stbox.NewData(stbox.Field{Key: "header", Value: header})))
	return s
}

func TestWritersHandleImageCues(t *testing.T) {
	t.Parallel()

	text := func(start, end time.Duration, line string) stbox.Cue {
		return stbox.Cue{Start: start, End: end, Lines: []string{line}}
	}
	one, two, three := text(time.Second, 2*time.Second, "One"), text(5*time.Second, 6*time.Second, "Two"), text(7*time.Second, 8*time.Second, "Three")
	withImages := subtitleOfCues(t, 1, one, imageCue(3*time.Second, 4*time.Second), two, imageCue(7*time.Second, 8*time.Second, "Three"))
	// The comment before the dropped image cue moves to the next kept cue.
	skipped := subtitleOfCues(t, 1, one, two, three)

	for _, f := range stbox.Formats() {
		if !f.CanWrite() || f == stbox.PGS {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			got, err := writeBytes(t, withImages, f, stbox.WriteOptions{})
			if f == stbox.JSON {
				require.NoError(t, err)
				assert.Contains(t, string(got), `"image"`)
				return
			}
			var uerr *stbox.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, 1, uerr.CueIndex)
			assert.Equal(t, f, uerr.Format)
			assert.Equal(t, "cue #1 holds an image but no text, and "+string(f)+" needs text", uerr.Reason)

			got, err = writeBytes(t, withImages, f, stbox.WriteOptions{SkipImageCues: true})
			want, wantErr := writeBytes(t, skipped, f, stbox.WriteOptions{})
			assert.Equal(t, wantErr, err)
			assert.Equal(t, want, got)
		})
	}
}
