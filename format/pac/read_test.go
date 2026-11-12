package pac_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestReadCodePage(t *testing.T) {
	t.Parallel()

	story := "STORY:News\xFE\x02\x03LANG:RUS"
	tests := []struct {
		name     string
		content  []byte
		opts     pac.ReadOptions
		lenient  bool
		codePage pac.CodePage
		text     string
		warning  string
	}{
		{name: "language of the story record", content: file(story, "Lf&"), codePage: pac.Cyrillic, text: "\u0414\u0430."},
		{name: "language in lower case", content: file("story:News\xFE\x02\x03lang:heb", "Lf"), codePage: pac.Hebrew, text: "Lf"},
		{name: "option wins over the story record", content: file(story, "Hello"), opts: pac.ReadOptions{CodePage: pac.Latin}, codePage: pac.Latin, text: "Hello"},
		{name: "option against the bytes", content: file("Lf"), opts: pac.ReadOptions{CodePage: pac.Greek}, codePage: pac.Greek, text: "\u039c\u03b6"},
		{name: "unknown language is detected", content: file("STORY:News\xFE\x02\x03LANG:XYZ", "Hello"), codePage: pac.Latin, text: "Hello"},
		{name: "detected", content: file("Hello"), codePage: pac.Latin, text: "Hello"},
		{
			name: "no code page fits, lenient", content: file("\xC3\xC4"), lenient: true, codePage: pac.Latin, text: "\u00c3\u00c4",
			warning: "The code page of the PAC file is not clear. No code page gives text in its script. " +
				"Set the code page of the file, for example ISO-8859-7 for Greek. The parser read it as latin.",
		},
		{
			name: "several code pages fit, lenient", content: file("Lf"), lenient: true, codePage: pac.Greek, text: "\u039c\u03b6",
			warning: "The code page of the PAC file is not clear. These code pages fit: greek, cyrillic. " +
				"Set the code page of the file, for example ISO-8859-7 for Greek. The parser read it as greek.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := read(t, tt.content, tt.lenient, tt.opts)
			data, _ := s.FormatData("pac")
			stored, _ := data.String("codePage")
			assert.Equal(t, string(tt.codePage), stored)
			require.Equal(t, 1+len(s.Cues())-1, s.Len())
			assert.Equal(t, tt.text, s.Cues()[s.Len()-1].Text())
			if tt.warning == "" {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, tt.warning, warnings[0].Message)
			assert.Equal(t, model.Repaired, warnings[0].Action)
		})
	}
}

func TestReadStrictCodePageError(t *testing.T) {
	t.Parallel()

	_, _, err := pac.Read(bytes.NewReader(file("\xC3\xC4")), model.CommonReadOptions{}, pac.ReadOptions{})
	var perr *model.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "The code page of the PAC file is not clear. No code page gives text in its script. "+
		"Set the code page of the file, for example ISO-8859-7 for Greek.", perr.Msg)
}

func TestReadInvalidFiles(t *testing.T) {
	t.Parallel()

	valid := file("Hello")
	with := func(at int, b ...byte) []byte {
		out := bytes.Clone(valid)
		copy(out[at:], b)
		return out
	}
	number := func(n uint16) []byte { return binary.LittleEndian.AppendUint16(nil, n) }
	tests := []struct {
		name       string
		content    []byte
		strict     string
		warning    string
		cues       int
		block      int
		blockStart string
	}{
		{name: "short header", content: []byte{1, 0, 0}, strict: "A PAC file starts with the byte 01h and 19 zero bytes."},
		{name: "length runs past the file", content: with(32, 0xFF, 0), strict: "The PAC record at byte 20 is not complete.", warning: "The PAC record at byte 20 is not complete."},
		{name: "length below 1", content: with(32, 0, 0), strict: "The PAC record at byte 20 is not complete.", warning: "The PAC record at byte 20 is not complete."},
		{name: "mark below 60h", content: with(23, 0x50), strict: "The PAC record at byte 20 is not complete.", warning: "The PAC record at byte 20 is not complete."},
		{name: "mark above 67h", content: with(23, 0x68), strict: "The PAC record at byte 20 is not complete.", warning: "The PAC record at byte 20 is not complete."},
		{
			name: "end block", content: with(len(valid)-len(footer), 7), strict: "The byte at 43 is neither a PAC record nor the end block.",
			warning: "The byte at 43 is neither a PAC record nor the end block.", cues: 1, block: 1,
		},
		{name: "minute 60", content: with(24, number(60)...), strict: "Subtitle number 0 has a time code that is not valid.", warning: "Subtitle number 0 has a time code that is not valid.", block: 0},
		{name: "second 60", content: with(26, number(6000)...), strict: "Subtitle number 0 has a time code that is not valid.", warning: "Subtitle number 0 has a time code that is not valid.", block: 0},
		{name: "frame 25 at 25 fps", content: with(28, number(0)...), strict: "", cues: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := pac.Read(bytes.NewReader(tt.content), model.CommonReadOptions{}, pac.ReadOptions{})
			if tt.strict == "" {
				require.NoError(t, err)
				return
			}
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.strict, perr.Msg)
			if tt.warning == "" {
				return
			}
			s, warnings := read(t, tt.content, true, pac.ReadOptions{})
			assert.Equal(t, tt.cues, s.Len())
			require.Len(t, warnings, 1)
			assert.Equal(t, tt.warning, warnings[0].Message)
			assert.Equal(t, model.Skipped, warnings[0].Action)
			assert.Equal(t, model.NoLine, warnings[0].Line)
			assert.Equal(t, tt.block, warnings[0].Block)
			assert.NotEmpty(t, warnings[0].Lines)
		})
	}
}

func TestReadText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		opts pac.ReadOptions
		want []string
	}{
		{name: "colors, codes and FFh", text: "\x03Red\xFFtext\x1FC1 end\x1FCcomment", want: []string{"Red text end"}},
		{name: "italics take the place of a space", text: "Go<now>, he said<quietly", want: []string{"Go <i>now</i>, he said <i>quietly</i>"}},
		{name: "2-byte code", text: "Caf\xE2\x65", want: []string{"Caf\u00e9"}},
		{name: "L before an accented letter", text: "L\xE2\x65", want: []string{"L\u00e9"}},
		{name: "moved ASCII bytes", text: "\x23\x5C\x7C", want: []string{"\u00a3\u00c6\u00e6"}},
		{name: "second line", text: "One\xFE\x02\x03Two", want: []string{"One", "Two"}},
		{name: "Unicode run", text: "\x1F\xEF\xBB\xBFCaf\xC3\xA9\xFF.Done", want: []string{"Caf\u00e9.Done"}},
		{name: "invalid UTF-8 in a Unicode run", text: "\x1F\xEF\xBB\xBFa\xE4\xBD\xFFb.", want: []string{"a?.b"}},
		{name: "Hebrew reverses Latin letters", text: "\xA0\xA1(ab12)", opts: pac.ReadOptions{CodePage: pac.Hebrew}, want: []string{"\u05d0\u05d1)21ba("}},
		{name: "Arabic mark before its letter", text: "\xE5\x81", opts: pac.ReadOptions{CodePage: pac.Arabic}, want: []string{"\u0627\u064e"}},
		{name: "trail byte FEh stays in its pair", text: "\x1FW16.\xA5\xFE\xA5\x5D", opts: pac.ReadOptions{CodePage: pac.ChineseTraditional}, want: []string{"\u5168\u5305"}},
		{name: "double-byte run with ASCII", text: "\x1FW16.\x00A\x41B\xB0\xA1", opts: pac.ReadOptions{CodePage: pac.Korean}, want: []string{"AAB\uac00"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := tt.opts
			if opts.CodePage == pac.CodePageAuto {
				opts.CodePage = pac.Latin
			}
			s, _ := read(t, file(tt.text), false, opts)
			require.Equal(t, 1, s.Len())
			assert.Equal(t, tt.want, s.Cues()[0].Lines)
		})
	}
}

func TestReadPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		vp, align byte
		want      model.Position
	}{
		{name: "top left", vp: 0, align: 1, want: model.Position{Alignment: model.TopLeft, Row: 1, Grid: model.Grid{Rows: 12}}},
		{name: "middle right", vp: 6, align: 0, want: model.Position{Alignment: model.MiddleRight, Row: 7, Grid: model.Grid{Rows: 12}}},
		{name: "bottom center", vp: 10, align: 2, want: model.Position{Alignment: model.BottomCenter, Row: 11, Grid: model.Grid{Rows: 12}}},
		{name: "bottom center for 3", vp: 11, align: 3, want: model.Position{Alignment: model.BottomCenter, Row: 12, Grid: model.Grid{Rows: 12}}},
		{name: "vertical position above 11 has no row", vp: 12, align: 1, want: model.Position{Alignment: model.BottomLeft}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := file("Hello")
			content[34], content[36] = tt.vp, tt.align
			s, _ := read(t, content, false, pac.ReadOptions{CodePage: pac.Latin})
			require.Equal(t, 1, s.Len())
			assert.Equal(t, tt.want, s.Cues()[0].Position)
		})
	}
}

func TestReadTimes(t *testing.T) {
	t.Parallel()

	at30 := file("Hello")
	binary.LittleEndian.PutUint16(at30[30:], 229) // 2 s and frame 29
	s, _ := read(t, at30, false, pac.ReadOptions{CodePage: pac.Latin})
	data, _ := s.FormatData("pac")
	rate, _ := data.Int("frameRate")
	assert.EqualValues(t, 30, rate)
	assert.Equal(t, int64(1000), s.Cues()[0].Start.Milliseconds())
	assert.Equal(t, int64(2967), s.Cues()[0].End.Milliseconds())
}

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{name: "file", content: file("Hello"), want: true},
		{name: "mark 67h", content: append(append([]byte{1}, make([]byte, 22)...), 0x67), want: true},
		{name: "mark 68h", content: append(append([]byte{1}, make([]byte, 22)...), 0x68)},
		{name: "header and text", content: append(append([]byte{1}, make([]byte, 19)...), 1, 2, 3)},
		{name: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, pac.Detect(tt.content))
		})
	}
}

func TestCodePageFromEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want pac.CodePage
		ok   bool
	}{
		{name: "ISO-8859-1", want: pac.Latin, ok: true},
		{name: "Windows-1252", want: pac.Latin, ok: true},
		{name: "ISO-8859-7", want: pac.Greek, ok: true},
		{name: "Windows-874", want: pac.Thai, ok: true},
		{name: "BIG5", want: pac.ChineseTraditional, ok: true},
		{name: "gb18030", want: pac.ChineseSimplified, ok: true},
		{name: "EUC-KR", want: pac.Korean, ok: true},
		{name: "sjis", want: pac.Japanese, ok: true},
		{name: "UTF8", want: pac.Unicode, ok: true},
		{name: "KOI8-R"},
		{name: "TIS-620"},
		{name: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := pac.CodePageFromEncoding(tt.name)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadOptionsValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, pac.ReadOptions{CodePage: pac.Thai}.Validate())
	_, _, err := pac.Read(bytes.NewReader(file("Hello")), model.CommonReadOptions{}, pac.ReadOptions{CodePage: "klingon"})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	var ierr *model.InvalidArgumentError
	require.ErrorAs(t, err, &ierr)
	assert.Equal(t, "ReadOptions.PAC.CodePage", ierr.Option)
}
