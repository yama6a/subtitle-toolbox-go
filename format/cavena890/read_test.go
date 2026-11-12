package cavena890_test

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestReadFields(t *testing.T) {
	t.Parallel()

	danish := header(t, map[int]string{146: "\x07"})
	chinese := header(t, map[int]string{146: "\x90"})
	tests := []struct {
		name    string
		file    []byte
		variant string
		text    string
		align   model.Alignment
	}{
		{
			name: "dash, colors, fillers and a zero byte before a table code", variant: "latin", align: model.BottomCenter,
			file: append(header(t, nil), block(16, 0x54, "", "\xBE Red \xF1fox\xF7 \x00\x1D")...), text: "- Red fox å",
		},
		{
			name: "italics, an accent code and the brackets of the Latin table", variant: "latin", align: model.BottomLeft,
			file: append(header(t, nil), block(16, 0x50, "", "\x88Gr\x86u\xDFe\x98 \xE5x\xE6")...), text: "<i>Grüße</i> [x]",
		},
		{
			name: "Danish keeps E5h and E6h as Windows-1252", variant: "scandinavian", align: model.BottomRight,
			file: append(danish, block(16, 0x52, "", "\xE5\xE6\x5B")...), text: "åæÆ",
		},
		{
			name: "bytes that Windows-1252 leaves undefined give nothing", variant: "latin", align: model.BottomCenter,
			file: append(header(t, nil), block(16, 0x99, "", "a\x8Db\x80")...), text: "ab€",
		},
		{
			name: "upper and lower line", variant: "latin", align: model.BottomCenter,
			file: append(header(t, nil), block(16, 0x54, "One", "Two")...), text: "One\nTwo",
		},
		{
			name: "2 blocks with the same number form one cue", variant: "latin", align: model.BottomCenter,
			file: append(append(header(t, nil), block(16, 0x54, "One", "Two")...), block(16, 0x50, "Three", "")...), text: "One\nTwo\nThree",
		},
		{
			name: "Chinese reads a last character with the low byte 7Fh", variant: "chinese", align: model.BottomCenter,
			file: append(chinese, block(16, 0x54, "", "\x5C\x9B\x5C\x7F")...), text: string([]rune{0x5c9b, 0x5c7f}),
		},
		{
			name: "Chinese joins a surrogate pair and drops a lone surrogate", variant: "chinese", align: model.BottomCenter,
			file: append(chinese, block(16, 0x54, "\x00a\xD8\x3D\xDE\x00\xD8\x3D\x00b", "")...), text: "a\U0001F600b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := read(t, tt.file, false)
			require.NoError(t, err)
			assert.Empty(t, warnings)
			require.Equal(t, 1, s.Len())
			c, _ := s.Cue(0)
			assert.Equal(t, tt.text, c.Text())
			assert.Equal(t, tt.align, c.Position.Alignment)
			assert.Equal(t, [2]time.Duration{second, 4 * second}, [2]time.Duration{c.Start, c.End})
			data, _ := s.FormatData("cavena890")
			variant, _ := data.String("variant")
			assert.Equal(t, tt.variant, variant)
		})
	}
}

func TestHeaderNamesTheVariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		edits   map[int]string
		variant string
	}{
		{"Danish code", map[int]string{146: "\x07"}, "scandinavian"},
		{"Hebrew font", map[int]string{246: "HEBNOA"}, "hebrew"},
		{"Greek code", map[int]string{146: "\x70"}, "greek"},
		{"Arabic line 2", map[int]string{147: "\x80"}, "arabic"},
		{"Russian font", map[int]string{187: "KYRIL4"}, "russian"},
		{"Chinese code", map[int]string{146: "\x90"}, "chinese"},
		{"Hebrew before Arabic", map[int]string{147: "\x80", 146: "\x8F"}, "hebrew"},
		{"English", nil, "latin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := read(t, header(t, tt.edits), false)
			require.NoError(t, err)
			data, _ := s.FormatData("cavena890")
			variant, _ := data.String("variant")
			assert.Equal(t, tt.variant, variant)
		})
	}
}

func TestReadInvalidFiles(t *testing.T) {
	t.Parallel()

	partial := append(append(header(t, nil), block(16, 0x54, "", "One")...), block(32, 0x54, "", "Two")[:50]...)
	tests := []struct {
		name    string
		file    []byte
		lenient bool
		wantErr string
		cues    []string
		rest    string
	}{
		{name: "short header", file: make([]byte, 100), wantErr: "A Cavena 890 file starts with a header of 388 bytes."},
		{name: "short header in lenient mode", file: make([]byte, 100), lenient: true, wantErr: "A Cavena 890 file starts with a header of 388 bytes."},
		{name: "incomplete block", file: partial, wantErr: "The blocks of a Cavena 890 file have 128 bytes each."},
		{name: "incomplete block in lenient mode", file: partial, lenient: true, cues: []string{"One"}, rest: hex.EncodeToString(partial[388+128:])},
		{name: "short end", file: append(header(t, nil), block(16, 0x54, "", "One")[:124]...), cues: []string{"One"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := read(t, tt.file, tt.lenient)
			if tt.wantErr != "" {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Contains(t, perr.Msg, tt.wantErr)
				return
			}
			require.NoError(t, err)
			var got []string
			for _, c := range s.All() {
				got = append(got, c.Text())
			}
			assert.Equal(t, tt.cues, got)
			if tt.rest == "" {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, model.Skipped, warnings[0].Action)
			assert.Equal(t, model.NoLine, warnings[0].Line)
			assert.Equal(t, []string{tt.rest}, warnings[0].Lines)
		})
	}
}

func TestReadStoresTheShortEnd(t *testing.T) {
	t.Parallel()

	file := append(header(t, nil), block(16, 0x54, "", "One")...)
	for _, short := range []bool{true, false} {
		content := file
		if short {
			content = bytes.Clone(file[:len(file)-4])
		}
		s, _, err := read(t, content, false)
		require.NoError(t, err)
		data, _ := s.FormatData("cavena890")
		stored, _ := data.Bool("shortEnd")
		assert.Equal(t, short, stored)
		assert.Len(t, write(t, s, model.CommonWriteOptions{}), len(content))
	}
}
