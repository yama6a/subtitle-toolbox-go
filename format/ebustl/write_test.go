package ebustl_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func writeSubtitle(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts ebustl.WriteOptions) ([]byte, ebustl.WriteReport) {
	t.Helper()

	var out bytes.Buffer
	report, err := ebustl.Write(&out, s, common, opts)
	require.NoError(t, err)
	return out.Bytes(), report
}

// newSubtitle returns a subtitle without file data. Each line of lines is one cue of 1 s to 2 s, and a cue holds
// its rows separated by "|".
func newSubtitle(lines ...string) *model.Subtitle {
	s := &model.Subtitle{}
	for i, l := range lines {
		c := model.Cue{Start: time.Duration(i+1) * time.Second, End: time.Duration(i+2) * time.Second}
		c.SetText(strings.ReplaceAll(l, "|", "\n"))
		s.AddCue(c)
	}
	return s
}

func stlFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(testfiles.Path(t, "stl/real"))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".stl" {
			names = append(names, e.Name())
		}
	}
	require.Len(t, names, 17)
	return names
}

func TestWriteRoundTripsRealFiles(t *testing.T) {
	t.Parallel()

	for _, name := range stlFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			content := testfiles.Read(t, "stl/real/"+name)
			s, _ := readBytes(t, content, ebustl.ReadOptions{})
			// The writer puts the sets in cue order, so a file with sets out of time order comes back reordered.
			if name != "shop_comments_out_of_order.stl" {
				got, _ := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
				assert.True(t, bytes.Equal(content, got), "the bytes differ")

				shifted, _ := readBytes(t, content, ebustl.ReadOptions{SubtractStartOfProgramme: true})
				got, _ = writeSubtitle(t, shifted, model.CommonWriteOptions{}, ebustl.WriteOptions{})
				assert.True(t, bytes.Equal(content, got), "the bytes differ after SubtractStartOfProgramme")
			}

			// Without stored blocks the writer encodes every cue again.
			for i, c := range s.All() {
				data, _ := c.FormatData("stl")
				data.Delete("blocks")
				require.NoError(t, c.SetFormatData("stl", data))
				require.NoError(t, s.SetCue(i, c))
			}
			again, _ := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
			back, _ := readBytes(t, again, ebustl.ReadOptions{})
			want, _ := readBytes(t, content, ebustl.ReadOptions{})
			require.Equal(t, want.Len(), back.Len())
			for i, w := range want.All() {
				g, _ := back.Cue(i)
				assert.Equal(t, w.Start, g.Start, "cue %d start", i)
				assert.Equal(t, w.End, g.End, "cue %d end", i)
				assert.Equal(t, w.Text(), g.Text(), "cue %d text", i)
				assert.Equal(t, w.Position.Alignment, g.Position.Alignment, "cue %d alignment", i)
			}
			assert.Equal(t, want.AllMetadata(), back.AllMetadata())
		})
	}
}

func TestWriteTextFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{"italics and underline", `<i>warm</i> and <u>fresh</u>`, "[80]warm[81] and [82]fresh[83]"},
		{"color", `<font color="#FF0000">Fresh</font> rolls`, "[01]Fresh[07]rolls"},
		{"single quotes", `<font color='#FF0000'>Fresh</font> rolls`, "[01]Fresh[07]rolls"},
		{"upper case tag", `<FONT COLOR="#FF0000">Fresh</font> rolls`, "[01]Fresh[07]rolls"},
		{"no quotes", `<font color=#FF0000>Fresh</font> rolls`, "[01]Fresh[07]rolls"},
		{"face before color", `<font face="Arial" color="#FF0000">Fresh</font> rolls`, "[01]Fresh[07]rolls"},
		{"nested colors", `<font color="#00ff00">a <font color="#0000ff">b</font> c</font>`, "[02]a[04]b[02]c"},
		{"font without color inside a color", `<font color="#00ff00">a <font face="Arial">b</font> c</font>`, "[02]a b c"},
		{"color outside the 8", `<font color="#123456">dark</font> text`, "dark text"},
		{"open italics", `<i><b>bold</b> italic`, "[80]bold italic[81]"},
		{"entities", `a &lt; b &amp; c`, "a < b & c"},
		{"angle brackets", `a < b > c`, "a < b > c"},
		{"tag then angle brackets", `<i>a</i>< b > c`, "[80]a[81]< b > c"},
		{"currency and dollar", "$5 or ¤5", "[A4]5 or $5"},
		{"WebVTT color class", `<c.yellow>Mind the gap</c>`, "[03]Mind the gap"},
		{"diacritic", "café", "caf[C2]e"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := writeSubtitle(t, newSubtitle(tt.line), model.CommonWriteOptions{}, ebustl.WriteOptions{})
			assert.Equal(t, tt.want, textField(out[1024+16:1024+128]))
		})
	}
}

// textField returns a text field without the 8Fh padding. ASCII stays as text and every other code is a hex
// number in brackets.
func textField(b []byte) string {
	for len(b) > 0 && b[len(b)-1] == 0x8F {
		b = b[:len(b)-1]
	}
	var out strings.Builder
	for _, c := range b {
		if c >= 0x20 && c < 0x7F {
			out.WriteByte(c)
		} else {
			out.WriteString("[" + strings.ToUpper(hex.EncodeToString([]byte{c})) + "]")
		}
	}
	return out.String()
}

func TestWriteTextFieldRowsAndBlocks(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 111) + "é"
	tests := []struct {
		name   string
		cue    string
		strip  bool
		blocks int
		field  int
		want   string
	}{
		{name: "two rows", cue: "<u>under|line</u>", blocks: 1, want: "[82]under[83][8A][82]line[83]"},
		{name: "strip tags", cue: "<i>a</i> <b>b</b>", strip: true, blocks: 1, want: "a b"},
		{name: "diacritic stays with its letter", cue: long, blocks: 2, field: 1, want: "[C2]e"},
		{name: "112 bytes make an empty second block", cue: strings.Repeat("a", 112), blocks: 2, field: 1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := writeSubtitle(t, newSubtitle(tt.cue), model.CommonWriteOptions{StripTags: tt.strip}, ebustl.WriteOptions{})
			require.Len(t, out, 1024+128*tt.blocks)
			start := 1024 + 128*tt.field
			assert.Equal(t, tt.want, textField(out[start+16:start+128]))
			assert.Equal(t, byte(0xFF), out[1024+128*(tt.blocks-1)+3])
		})
	}
}

func TestWritePositions(t *testing.T) {
	t.Parallel()

	grid := model.Grid{Rows: 15, Columns: 32}
	tests := []struct {
		name   string
		gsi    map[string]string
		a      model.Alignment
		pos    model.Position
		lines  int
		vp, jc byte
	}{
		{"bottom center", nil, model.BottomCenter, model.Position{}, 1, 23, 2},
		{"bottom left", nil, model.BottomLeft, model.Position{}, 1, 23, 1},
		{"middle center", nil, model.MiddleCenter, model.Position{}, 1, 12, 2},
		{"top right", nil, model.TopRight, model.Position{}, 1, 1, 3},
		{"two lines at the bottom", nil, model.BottomCenter, model.Position{}, 2, 22, 2},
		{"open subtitles", map[string]string{"DSC": "0", "MNR": "11"}, model.TopCenter, model.Position{}, 1, 0, 2},
		{"SCC row 2 at the top", nil, model.TopCenter, model.Position{Alignment: model.TopCenter, Row: 2, Grid: grid}, 1, 2, 2},
		{"SCC row 2 with a bottom alignment", nil, model.BottomCenter, model.Position{Alignment: model.BottomCenter, Row: 2, Grid: grid}, 1, 23, 2},
		{"SCC row 15 at the bottom", nil, model.BottomCenter, model.Position{Alignment: model.BottomCenter, Row: 15, Grid: grid}, 1, 23, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Start: time.Second, End: 2 * time.Second, Position: tt.pos}
			c.Position.Alignment = tt.a
			for range tt.lines {
				c.AddLine("x")
			}
			s := &model.Subtitle{}
			s.AddCue(c)
			if tt.gsi != nil {
				data := model.NewData()
				gsi := model.NewData()
				for k, v := range tt.gsi {
					gsi.Set(k, v)
				}
				data.Set("gsi", gsi)
				require.NoError(t, s.SetFormatData("stl", data))
			}
			out, _ := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
			assert.Equal(t, []byte{tt.vp, tt.jc}, out[1024+13:1024+15])
		})
	}
}

func TestWriteKeepsStoredPositionUntilTheRowChanges(t *testing.T) {
	t.Parallel()

	file := append(gsiBlock(nil), ttiBlock(tti{vp: 20, jc: 1})...)
	tests := []struct {
		name   string
		change func(c *model.Cue)
		vp, jc byte
	}{
		{"unchanged", func(*model.Cue) {}, 20, 1},
		{"row changed to 5 at the top", func(c *model.Cue) { c.Position.Alignment, c.Position.Row = model.TopLeft, 5 }, 5, 1},
		{"alignment changed", func(c *model.Cue) { c.Position = model.Position{Alignment: model.TopRight} }, 1, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := readBytes(t, file, ebustl.ReadOptions{})
			c, _ := s.Cue(0)
			tt.change(&c)
			require.NoError(t, s.SetCue(0, c))
			out, _ := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
			assert.Equal(t, []byte{tt.vp, tt.jc}, out[1024+13:1024+15])
		})
	}
}

func TestWriteGSIAndReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		build    func() *model.Subtitle
		opts     ebustl.WriteOptions
		field    func(out []byte) string
		want     string
		table    string
		chosen   bool
		replaced []ebustl.Replacement
	}{
		{
			name: "new file", opts: ebustl.WriteOptions{}, table: "00", chosen: true,
			build: func() *model.Subtitle {
				s := newSubtitle("Hello", "Bye")
				s.SetMetadata(model.MetadataTitle, "Harbour")
				s.SetMetadata(model.MetadataLanguage, "en-GB")
				return s
			},
			field: func(out []byte) string { return string(out[:16]) + string(out[16:23]) + string(out[238:251]) },
			want:  "850STL25.011" + "00" + "09" + "Harbour" + "00002" + "00002" + "001",
		},
		{
			name: "30 fps", opts: ebustl.WriteOptions{FrameRate: model.FrameRate30}, table: "00", chosen: true,
			build: func() *model.Subtitle { return newSubtitle("Hello") },
			field: func(out []byte) string { return string(out[3:11]) },
			want:  "STL30.01",
		},
		{
			name: "longest row sets MNC", table: "00", chosen: true,
			build: func() *model.Subtitle { return newSubtitle(strings.Repeat("é", 41)) },
			field: func(out []byte) string { return string(out[251:253]) },
			want:  "41",
		},
		{
			name: "table with the fewest question marks", table: "01", chosen: true,
			build:    func() *model.Subtitle { return newSubtitle("\u2014 \u0416\u0416\u0416\u0416 \u00ab\u00bb") },
			field:    func(out []byte) string { return textField(out[1024+16 : 1024+128]) },
			want:     "? [B6][B6][B6][B6] ??",
			replaced: []ebustl.Replacement{{CueIndex: 0, Character: '\u2014'}, {CueIndex: 0, Character: '\u00ab'}, {CueIndex: 0, Character: '\u00bb'}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, report := writeSubtitle(t, tt.build(), model.CommonWriteOptions{}, tt.opts)
			require.Len(t, out, 1024+128*tt.build().Len())
			assert.Equal(t, tt.table, report.CharacterCodeTable)
			assert.Equal(t, tt.chosen, report.TableChosen)
			assert.Equal(t, tt.want, tt.field(out))
			assert.Equal(t, tt.replaced, report.Replaced)
		})
	}
}

func TestWriteReportsTheTableAndTheTitle(t *testing.T) {
	t.Parallel()

	stored := model.NewData(model.Field{Key: "gsi", Value: model.NewData(model.Field{Key: "CCT", Value: "03"})})
	s := newSubtitle("Hello")
	require.NoError(t, s.SetFormatData("stl", stored))
	_, report := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
	assert.Equal(t, ebustl.WriteReport{CharacterCodeTable: "03"}, report)

	titled := newSubtitle("Hello")
	titled.SetMetadata(model.MetadataTitle, "Ж")
	out, report := writeSubtitle(t, titled, model.CommonWriteOptions{}, ebustl.WriteOptions{})
	assert.Equal(t, []ebustl.Replacement{{CueIndex: -1, Field: "OPT", Character: 'Ж'}}, report.Replaced)
	assert.Equal(t, "?", string(out[16:17]))
}

func TestWriteComments(t *testing.T) {
	t.Parallel()

	s := newSubtitle("One", "Two")
	require.NoError(t, s.AddComment("Before one", 0))
	require.NoError(t, s.AddComment("After all", 5))
	out, _ := writeSubtitle(t, s, model.CommonWriteOptions{}, ebustl.WriteOptions{})
	require.Len(t, out, 1024+4*128)
	flags := []byte{out[1024+15], out[1024+128+15], out[1024+256+15], out[1024+384+15]}
	assert.Equal(t, []byte{1, 0, 0, 1}, flags)
	assert.Equal(t, []byte{0, 0, 1, 0}, out[1024+5:1024+9], "a comment takes the start of the next cue")
	assert.Equal(t, []byte{0, 0, 3, 0}, out[1024+384+5:1024+384+9], "a comment at the end takes the end of the last cue")
	assert.Equal(t, "00004", string(out[238:243]))
	assert.Equal(t, "00002", string(out[243:248]))
}

func TestWriteInvalidInput(t *testing.T) {
	t.Parallel()

	storedTable := func(cct string) *model.Subtitle {
		s := newSubtitle("a")
		require.NoError(t, s.SetFormatData("stl", model.NewData(model.Field{Key: "gsi", Value: model.NewData(model.Field{Key: "CCT", Value: cct})})))
		return s
	}
	first := newSubtitle("a")
	require.NoError(t, first.SetFormatData("stl", model.NewData(model.Field{Key: "firstSubtitleNumber", Value: int64(65535)})))
	two := newSubtitle("a", "b")
	require.NoError(t, two.SetFormatData("stl", model.NewData(model.Field{Key: "firstSubtitleNumber", Value: int64(65535)})))

	tests := []struct {
		name string
		s    *model.Subtitle
		opts ebustl.WriteOptions
		want string
	}{
		{"frame rate 24", newSubtitle("a"), ebustl.WriteOptions{FrameRate: model.FrameRate{Num: 24, Den: 1}}, "The EBU STL formatter writes 25 or 30 fps, got 24."},
		{"stored table 05", storedTable("05"), ebustl.WriteOptions{}, `The character code table "05" is not 00, 01, 02, 03 or 04.`},
		{"too many blocks", newSubtitle(strings.Repeat("a", 112*242)), ebustl.WriteOptions{}, "The cue text needs more than 241 TTI blocks."},
		{"subtitle number 65536", two, ebustl.WriteOptions{}, "EBU STL allows subtitle numbers up to 65535."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			_, err := ebustl.Write(&out, tt.s, model.CommonWriteOptions{}, tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Empty(t, out.Bytes())
		})
	}
	_, err := ebustl.Write(&bytes.Buffer{}, first, model.CommonWriteOptions{}, ebustl.WriteOptions{})
	assert.NoError(t, err, "65535 is the last subtitle number")
}
