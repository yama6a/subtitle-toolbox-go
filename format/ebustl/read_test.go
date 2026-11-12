package ebustl_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestReadHeaderFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		gsi         map[string]string
		block       tti
		opts        ebustl.ReadOptions
		title, lang string
		start, end  int64
	}{
		{name: "code page 865", gsi: map[string]string{"CPN": "865", "OPT": "A\x9E"}, title: "A\u20a7", lang: "en", start: 1200, end: 2400},
		{name: "unknown code page reads as 850", gsi: map[string]string{"CPN": "999", "OPT": "A\x9E"}, title: "A\u00d7", lang: "en", start: 1200, end: 2400},
		{name: "language in lower case", gsi: map[string]string{"LC": "0f"}, lang: "fr", start: 1200, end: 2400},
		{name: "unknown language", gsi: map[string]string{"LC": "ZZ"}, start: 1200, end: 2400},
		{name: "title with trailing NUL bytes", gsi: map[string]string{"OPT": "Bakery\x00\x00", "LC": "00"}, title: "Bakery", start: 1200, end: 2400},
		{name: "30 fps", gsi: map[string]string{"DFC": "STL30.01", "LC": "00"}, start: 1167, end: 2333},
		{
			name: "start of programme before a time after it", gsi: map[string]string{"TCP": "10000000", "LC": "00"},
			block: tti{tci: [4]byte{10, 0, 1, 5}, tco: [4]byte{10, 0, 2, 10}},
			opts:  ebustl.ReadOptions{SubtractStartOfProgramme: true}, start: 1200, end: 2400,
		},
		{
			name: "start of programme after a time before it", gsi: map[string]string{"TCP": "00000200", "LC": "00"},
			opts: ebustl.ReadOptions{SubtractStartOfProgramme: true}, start: 0, end: 400,
		},
		{
			name: "start of programme that is not 8 digits", gsi: map[string]string{"TCP": "1000000", "LC": "00"},
			opts: ebustl.ReadOptions{SubtractStartOfProgramme: true}, start: 1200, end: 2400,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.block.text = "Text"
			s, warnings := readBytes(t, append(gsiBlock(tt.gsi), ttiBlock(tt.block)...), tt.opts)
			assert.Empty(t, warnings)
			title, _ := s.Metadata(model.MetadataTitle)
			lang, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, tt.title, title)
			assert.Equal(t, tt.lang, lang)
			require.Equal(t, 1, s.Len())
			c := s.Cues()[0]
			assert.Equal(t, ms(tt.start), c.Start)
			assert.Equal(t, ms(tt.end), c.End)
		})
	}
}

func TestReadTextFields(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 111)
	tests := []struct {
		name   string
		blocks [][]byte
		lines  []string
		count  int
	}{
		{"example", [][]byte{ttiBlock(tti{text: "\x80Caf\xC2e\x81\x8A"})}, []string{"<i>Café</i>"}, 1},
		{"color resets on a new row", [][]byte{ttiBlock(tti{text: "\x01red\x8Awhite"})}, []string{`<font color="#ff0000">red</font>`, "white"}, 1},
		{"italics last over a new row", [][]byte{ttiBlock(tti{text: "\x80one\x8Atwo\x81 three"})}, []string{"<i>one</i>", "<i>two</i> three"}, 1},
		{"nested styles", [][]byte{ttiBlock(tti{text: "\x80\x82both\x81 under\x83"})}, []string{"<i><u>both</u></i> <u>under</u>"}, 1},
		{"white has no tag", [][]byte{ttiBlock(tti{text: "\x07plain"})}, []string{"plain"}, 1},
		{"color between words", [][]byte{ttiBlock(tti{text: "a\x03b"})}, []string{`a <font color="#ffff00">b</font>`}, 1},
		{"boxing codes", [][]byte{ttiBlock(tti{text: "\x84boxed\x85"})}, []string{"boxed"}, 1},
		{"double new row", [][]byte{ttiBlock(tti{text: "one\x8A\x8Atwo"})}, []string{"one", "two"}, 1},
		{"markup characters", [][]byte{ttiBlock(tti{text: "a < b & c"})}, []string{"a &lt; b &amp; c"}, 1},
		{"undefined bytes", [][]byte{ttiBlock(tti{text: "a\xA6\xC9b\x7F"})}, []string{"ab"}, 1},
		{
			"diacritic joins across blocks and user data is skipped",
			[][]byte{
				ttiBlock(tti{ebn: 0x01, text: long + "\xC2"}),
				ttiBlock(tti{ebn: 0xFE, text: "e and more"}),
				ttiBlock(tti{ebn: 0x02, text: "e end"}),
				ttiBlock(tti{ebn: 0xFF}),
			},
			[]string{long + "é end"},
			4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := readBytes(t, append(gsiBlock(nil), bytes.Join(tt.blocks, nil)...), ebustl.ReadOptions{})
			require.Equal(t, 1, s.Len())
			c := s.Cues()[0]
			assert.Equal(t, tt.lines, c.Lines)
			data, _ := c.FormatData("stl")
			blocks, _ := data.Strings("blocks")
			assert.Len(t, blocks, tt.count)
		})
	}
}

func TestReadComments(t *testing.T) {
	t.Parallel()

	content := append(gsiBlock(nil), bytes.Join([][]byte{
		ttiBlock(tti{sn: 1, text: "First"}),
		ttiBlock(tti{sn: 2, cf: 1, text: "\x80Note & more\x81\x8A  second row  \x8A\x8A"}),
		ttiBlock(tti{sn: 3, text: "Second"}),
	}, nil)...)
	s, _ := readBytes(t, content, ebustl.ReadOptions{})
	assert.Equal(t, 2, s.Len())
	assert.Equal(t, []model.Comment{{Text: "Note & more\nsecond row", BeforeCue: 1}}, s.Comments())
	data, _ := s.FormatData("stl")
	list, _ := data.List("comments")
	assert.Len(t, list, 1)
	first, _ := data.Int("firstSubtitleNumber")
	assert.EqualValues(t, 1, first)
}

func TestReadPositions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		gsi  map[string]string
		vp   byte
		jc   byte
		want model.Position
	}{
		{"teletext top left", nil, 1, 1, model.Position{Alignment: model.TopLeft, Row: 1, Grid: model.Grid{Rows: 23, Columns: 40}}},
		{"teletext middle right", nil, 12, 3, model.Position{Alignment: model.MiddleRight, Row: 12, Grid: model.Grid{Rows: 23, Columns: 40}}},
		{"teletext bottom center", nil, 22, 2, model.Position{Alignment: model.BottomCenter, Row: 22, Grid: model.Grid{Rows: 23, Columns: 40}}},
		{"teletext row 0 is off the grid", map[string]string{"DSC": "2"}, 0, 2, model.Position{Alignment: model.TopCenter}},
		{"open top", map[string]string{"DSC": "0", "MNR": "23"}, 0, 2, model.Position{Alignment: model.TopCenter, Row: 1, Grid: model.Grid{Rows: 24, Columns: 40}}},
		{"open bottom", map[string]string{"DSC": "0", "MNR": "23"}, 20, 2, model.Position{Alignment: model.BottomCenter, Row: 21, Grid: model.Grid{Rows: 24, Columns: 40}}},
		{"open below the grid", map[string]string{"DSC": "0", "MNR": "23"}, 30, 2, model.Position{Alignment: model.BottomCenter}},
		{"open without MNR", map[string]string{"DSC": "0", "MNR": "", "MNC": "37"}, 20, 2, model.Position{Alignment: model.BottomCenter, Row: 21, Grid: model.Grid{Rows: 24, Columns: 37}}},
		{"MNC read like a PHP integer", map[string]string{"DSC": "0", "MNR": "15", "MNC": "2x"}, 1, 0, model.Position{Alignment: model.TopCenter, Row: 2, Grid: model.Grid{Rows: 16, Columns: 2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := readBytes(t, append(gsiBlock(tt.gsi), ttiBlock(tti{text: "Text", vp: tt.vp, jc: tt.jc, exactPos: true})...), ebustl.ReadOptions{})
			require.Equal(t, 1, s.Len())
			assert.Equal(t, tt.want, s.Cues()[0].Position)
			assert.NoError(t, s.Cues()[0].Position.Validate())
		})
	}
}

func TestReadInvalidFiles(t *testing.T) {
	t.Parallel()

	const (
		tooShort  = "An EBU STL file starts with a GSI block of 1024 bytes."
		blockSize = "The TTI blocks of an EBU STL file must have 128 bytes each."
	)
	partial := bytes.Repeat([]byte{0x8F}, 100)
	badCode := ttiBlock(tti{tco: [4]byte{0, 0, 2, 25}, tci: [4]byte{0, 0, 1, 5}})
	tests := []struct {
		name    string
		content []byte
		// strictError is the message of the strict read, "" when it succeeds with strictCues cues.
		strictError string
		strictCues  int
		// lenientError is the message of the lenient read, "" when it succeeds with the warnings.
		lenientError string
		warnings     []model.ParseWarning
	}{
		{name: "too short", content: bytes.Repeat([]byte{' '}, 1023), strictError: tooShort, lenientError: tooShort},
		{
			name: "partial TTI block", content: append(gsiBlock(nil), partial...), strictError: blockSize,
			warnings: []model.ParseWarning{{
				Message: blockSize, Line: model.NoLine, Block: 0, Lines: []string{hex.EncodeToString(partial)}, Action: model.Skipped,
			}},
		},
		{
			name: "disk format code", content: gsiBlock(map[string]string{"DFC": "STL24.01"}),
			strictError:  `The disk format code "STL24.01" is not STL25.01 or STL30.01.`,
			lenientError: `The disk format code "STL24.01" is not STL25.01 or STL30.01.`,
		},
		{
			name: "character code table", content: gsiBlock(map[string]string{"CCT": "05"}),
			strictError:  `The character code table "05" is not 00, 01, 02, 03 or 04.`,
			lenientError: `The character code table "05" is not 00, 01, 02, 03 or 04.`,
		},
		{
			name: "frame above the frame rate", content: append(gsiBlock(nil), badCode...), strictCues: 1,
			warnings: []model.ParseWarning{{
				Message: "Subtitle number 1 has a time code that is not valid: 00000105 to 00000225.", Line: model.NoLine, Block: 0,
				Lines: []string{hex.EncodeToString(badCode)}, Action: model.Skipped,
			}},
		},
		{name: "no TTI blocks", content: gsiBlock(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, lenient := range []bool{false, true} {
				s, warnings, err := ebustl.Read(bytes.NewReader(tt.content), model.CommonReadOptions{Lenient: lenient}, ebustl.ReadOptions{})
				wantError, wantCues, wantWarnings := tt.strictError, tt.strictCues, []model.ParseWarning(nil)
				if lenient {
					wantError, wantCues, wantWarnings = tt.lenientError, 0, tt.warnings
				}
				if wantError != "" {
					var perr *model.ParseError
					require.ErrorAs(t, err, &perr)
					assert.Equal(t, wantError, perr.Msg)
					assert.Equal(t, model.EBUSTL, perr.Format)
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, wantCues, s.Len())
				assert.Equal(t, wantWarnings, warnings)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"25 fps", "850STL25.01", true},
		{"30 fps", "437STL30.01 and more", true},
		{"24 fps", "850STL24.01", false},
		{"letter in the code page", "85xSTL25.01", false},
		{"too short", "850STL25.0", false},
		{"BOM first", "\xEF\xBB\xBF850STL25.01", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ebustl.Detect([]byte(tt.content)))
		})
	}
}

func TestStrictReadKeepsCuesWithABadTimeCode(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "lenient/bad_time_code.stl")[:1024+3*128]
	s, warnings := readBytes(t, content, ebustl.ReadOptions{})
	assert.Equal(t, 3, s.Len())
	assert.Empty(t, warnings)
}
