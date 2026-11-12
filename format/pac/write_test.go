package pac_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const header = "0100000000000000000000000000000000000000"

// defaultFooter is the hex of the footer of a new file.
const defaultFooter = "ff0000000000000000000000110064756d6d7920656e64206f662066696c65"

func cue(start, end time.Duration, p model.Position, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Position: p, Lines: lines}
}

func subtitle(t *testing.T, fileData *model.Data, cues ...model.Cue) *model.Subtitle {
	t.Helper()

	s := &model.Subtitle{}
	s.AddCues(cues...)
	if fileData != nil {
		require.NoError(t, s.SetFormatData("pac", fileData))
	}
	return s
}

func TestWriteNewFile(t *testing.T) {
	t.Parallel()

	long := cue(time.Second, 3480*time.Millisecond, model.Position{}, "Fresh rolls are <i>warm</i> today.", "<i>Caf\u00e9 cr\u00e8me</i>")
	tests := []struct {
		name    string
		cue     model.Cue
		common  model.CommonWriteOptions
		records string
	}{
		{
			name: "two lines", cue: long,
			records: "000000" + "60" + "00006400" + "00003801" + "2f00" + "0a" +
				"fe0203" + hexOf([]byte("Fresh rolls are<warm>today.")) + "fe0203" + "3c436166e26520" + "6372e3656d65",
		},
		{
			name: "strip tags", cue: long, common: model.CommonWriteOptions{StripTags: true},
			records: "000000" + "60" + "00006400" + "00003801" + "2e00" + "0a" +
				"fe0203" + hexOf([]byte("Fresh rolls are warm today.")) + "fe0203" + "436166e2652063" + "72e3656d65",
		},
		{
			name: "text that Latin cannot hold becomes PAC Unicode", cue: cue(time.Second, 2*time.Second, model.Position{}, "\u039a\u03b1."),
			records: "000000" + "60" + "00006400" + "0000c800" + "1100" + "0b" +
				"808080" + "fe0203" + "1fefbbbf" + "ce9aceb1ff" + "2e",
		},
		{name: "top center, 3 lines", cue: cue(0, time.Second, model.Position{Alignment: model.TopCenter}, "A", "B", "C"), records: "000000" + "60" + "00000000" + "00006400" + "0d00" + "01" + "fe020341fe020342fe020343"},
		{name: "middle right", cue: cue(0, time.Second, model.Position{Alignment: model.MiddleRight}, "A"), records: "000000" + "60" + "00000000" + "00006400" + "0500" + "06" + "fe000341"},
		{name: "bottom left", cue: cue(0, time.Second, model.Position{Alignment: model.BottomLeft}, "A"), records: "000000" + "60" + "00000000" + "00006400" + "0500" + "0b" + "fe010341"},
		{
			name: "row of another grid", cue: cue(0, time.Second, model.Position{Alignment: model.TopCenter, Row: 2, Grid: model.Grid{Rows: 15}}, "A"),
			records: "000000" + "60" + "00000000" + "00006400" + "0500" + "01" + "fe020341",
		},
		{
			name: "hours stop at 654", cue: cue(700*time.Hour, 700*time.Hour+time.Second, model.Position{}, "A"),
			records: "000000" + "60" + "78ff0000" + "78ff6400" + "0500" + "0b" + "fe020341",
		},
		{name: "negative start counts as 0", cue: cue(-time.Second, time.Second, model.Position{}, "A"), records: "000000" + "60" + "00000000" + "00006400" + "0500" + "0b" + "fe020341"},
		{name: "literal markers become question marks", cue: cue(0, time.Second, model.Position{}, "a < b > c"), records: "000000" + "60" + "00000000" + "00006400" + "0d00" + "0b" + "fe020361203f2062203f2063"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := write(t, subtitle(t, nil, tt.cue), tt.common)
			assert.Equal(t, header+tt.records+defaultFooter, hexOf(got))
		})
	}
}

func TestWriteKeepsStoredData(t *testing.T) {
	t.Parallel()

	greek := testfiles.Read(t, "pac/real/weather_greek.pac")
	s, _ := read(t, greek, false, pac.ReadOptions{})
	cues := s.Cues()
	changed := cues[0]
	changed.Lines = []string{"\u039a\u03b1\u03bb\u03b7\u03bc\u03ad\u03c1\u03b1, " + string(rune(0x4e2d)) + string(rune(0x6587))}
	require.NoError(t, s.SetCue(0, changed))
	out := write(t, s, model.CommonWriteOptions{})
	back, _ := read(t, out, false, pac.ReadOptions{})
	data, _ := back.FormatData("pac")
	stored, _ := data.String("codePage")
	assert.Equal(t, "greek", stored)
	assert.Equal(t, "\u039a\u03b1\u03bb\u03b7\u03bc\u03ad\u03c1\u03b1, ??", back.Cues()[0].Text())
	assert.Equal(t, cues[1].Text(), back.Cues()[1].Text())
	assert.Equal(t, cues[1].Position, back.Cues()[1].Position)
}

func TestWriteDoubleByte(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		codePage pac.CodePage
		text     string
		want     string // the hex of the bytes after FEh 02h 03h
	}{
		{name: "Big5 takes A55Dh, not the Hong Kong code", codePage: pac.ChineseTraditional, text: "\u5305", want: "1f573136" + "2e" + "a55d"},
		{name: "Big5 trail byte FEh reads back in its pair", codePage: pac.ChineseTraditional, text: "\u5168\u5305", want: "1f573136" + "2e" + "a5fe" + "a55d"},
		{name: "GBK trail byte FEh reads back in its pair", codePage: pac.ChineseSimplified, text: "\u4e8c\u5305", want: "1f573136" + "2e" + "b6fe" + "b0fc"},
		{name: "GBK character with the lead byte FEh is written as a question mark", codePage: pac.ChineseSimplified, text: "\u2e81\u5305", want: "1f573136" + "2e" + "003f" + "b0fc"},
		{name: "Shift_JIS wave dash is a one-way code", codePage: pac.Japanese, text: "\u301c", want: "1f573136" + "2e" + "8160"},
		{name: "Shift_JIS prefers the IBM extension", codePage: pac.Japanese, text: "\u2170", want: "1f573136" + "2e" + "fa40"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fileData := model.NewData(model.Field{Key: "codePage", Value: string(tt.codePage)})
			out := write(t, subtitle(t, fileData, cue(0, time.Second, model.Position{}, tt.text)), model.CommonWriteOptions{})
			assert.Contains(t, hexOf(out), "fe0203"+tt.want)
			back, _ := read(t, out, false, pac.ReadOptions{CodePage: tt.codePage})
			assert.Len(t, back.Cues()[0].Lines, 1)
		})
	}
}

func TestWriteRejects(t *testing.T) {
	t.Parallel()

	fileData := func(first int64) *model.Data {
		return model.NewData(model.Field{Key: "firstNumber", Value: first})
	}
	tests := []struct {
		name   string
		s      *model.Subtitle
		reason string
		index  int
	}{
		{
			name: "number 65536", index: 1, reason: "PAC allows subtitle numbers up to 65535.",
			s: subtitle(t, fileData(65535), cue(0, time.Second, model.Position{}, "A"), cue(time.Second, 2*time.Second, model.Position{}, "B")),
		},
		{
			name: "text of 65535 bytes", index: 0, reason: "PAC allows cue text of up to 65534 bytes.",
			s: subtitle(t, nil, cue(0, time.Second, model.Position{}, string(bytes.Repeat([]byte("a"), 65535)))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := pac.Write(&bytes.Buffer{}, tt.s, model.CommonWriteOptions{}, pac.WriteOptions{})
			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, model.PAC, uerr.Format)
			assert.Equal(t, tt.index, uerr.CueIndex)
			assert.Equal(t, tt.reason, uerr.Reason)
		})
	}
}

func TestWriteIgnoresBadStoredData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data *model.Data
	}{
		{name: "code page", data: model.NewData(model.Field{Key: "codePage", Value: "klingon"})},
		{name: "frame rate", data: model.NewData(model.Field{Key: "frameRate", Value: int64(24)})},
		{name: "first number", data: model.NewData(model.Field{Key: "firstNumber", Value: int64(-5)})},
		{name: "header that does not decode", data: model.NewData(model.Field{Key: "header", Value: "zz"})},
		{name: "header without the 20 header bytes", data: model.NewData(model.Field{Key: "header", Value: "0102"})},
		{name: "footer that does not start with FFh", data: model.NewData(model.Field{Key: "footer", Value: "0011"})},
		{name: "header with a broken story record", data: model.NewData(model.Field{Key: "header", Value: header + "0001"})},
		{name: "footer that does not decode", data: model.NewData(model.Field{Key: "footer", Value: "f"})},
	}
	want := write(t, subtitle(t, nil, cue(0, time.Second, model.Position{}, "A")), model.CommonWriteOptions{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, write(t, subtitle(t, tt.data, cue(0, time.Second, model.Position{}, "A")), model.CommonWriteOptions{}))
		})
	}
}

func TestWriteFailsOnAWriterError(t *testing.T) {
	t.Parallel()

	_, err := pac.Write(failWriter{}, subtitle(t, nil, cue(0, time.Second, model.Position{}, "A")), model.CommonWriteOptions{}, pac.WriteOptions{})
	require.ErrorIs(t, err, errFail)
}

var errFail = errors.New("disk full")

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errFail }

func TestRealFilesRoundTrip(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "pac/real/*.[pf]*c"))
	require.NoError(t, err)
	require.Len(t, files, 13)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()

			content, err := os.ReadFile(f)
			require.NoError(t, err)
			require.True(t, pac.Detect(content))
			s, warnings := read(t, content, false, pac.ReadOptions{})
			assert.Empty(t, warnings)
			assert.Equal(t, content, write(t, s, model.CommonWriteOptions{}))

			// Without the cue format data, the encoder must give the bytes of the decoder back.
			for i, c := range s.Cues() {
				c.DeleteFormatData("pac")
				require.NoError(t, s.SetCue(i, c))
			}
			assert.Equal(t, content, write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestReadWithAnotherCodePageWritesTheSameBytes(t *testing.T) {
	t.Parallel()

	content := testfiles.Read(t, "pac/real/weather_greek.pac")
	greek, _ := read(t, content, false, pac.ReadOptions{})
	cyrillic, _ := read(t, content, false, pac.ReadOptions{CodePage: pac.Cyrillic})
	assert.NotEqual(t, greek.Cues()[0].Text(), cyrillic.Cues()[0].Text())
	assert.Equal(t, content, write(t, cyrillic, model.CommonWriteOptions{}))
}
