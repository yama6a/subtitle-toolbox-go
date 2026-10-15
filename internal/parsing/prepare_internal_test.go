package parsing

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func controlWarning(line int) model.ParseWarning {
	return model.ParseWarning{
		Message: "The file has control characters, the first on line " + strconv.Itoa(line) + ". The parser removed them.",
		Line:    line,
		Block:   model.NoBlock,
		Action:  model.Repaired,
	}
}

// Ports the cases of tests/Parsers/ControlCharactersTest.php, tests/Parsers/RepeatedBomTest.php
// and tests/Parsers/NoCuesTest.php of subtitle-toolbox-php that the pre-pass decides.
func TestPrepare(t *testing.T) {
	t.Parallel()

	const cue = "1\n00:00:01,000 --> 00:00:02,000\n"
	tests := []struct {
		name       string
		content    string
		lenient    bool
		replaceNUL bool
		want       string
		blank      bool
		warnings   []model.ParseWarning
	}{
		{name: "plain text stays", content: cue + "Hello\n", want: cue + "Hello\n"},
		{name: "one BOM", content: "\ufeff" + cue + "Hello\n", want: cue + "Hello\n"},
		{name: "two BOMs", content: "\ufeff\ufeff" + cue + "Hello\n", want: cue + "Hello\n"},
		{name: "two BOMs lenient give no warning", content: "\ufeff\ufeff" + cue, lenient: true, want: cue},
		{name: "BOM after the start stays", content: cue + "\ufeffHello\n", want: cue + "\ufeffHello\n"},
		{name: "Ctrl-Z at the end", content: cue + "Hello\n\x1a", want: cue + "Hello\n"},
		{name: "Ctrl-Z run then CR LF", content: cue + "Hello\n\x1a\x1a\r\n", want: cue + "Hello\n\r\n"},
		{name: "Ctrl-Z after CR LF", content: cue + "Hello\r\n\x1a", want: cue + "Hello\r\n"},
		{name: "Ctrl-Z lenient gives no warning", content: cue + "Hello\n\x1a", lenient: true, want: cue + "Hello\n"},
		{
			name:     "only the last Ctrl-Z run goes",
			content:  "\x1a" + " " + "\x1a\n",
			lenient:  true,
			blank:    true,
			warnings: []model.ParseWarning{controlWarning(1)},
		},
		{
			name:     "Ctrl-Z before text is a control character",
			content:  cue + "Hel\x1alo\n",
			lenient:  true,
			want:     cue + "Hello\n",
			warnings: []model.ParseWarning{controlWarning(3)},
		},
		{name: "form feed strict", content: cue + "The fer\fry leaves.\n", want: cue + "The ferry leaves.\n"},
		{name: "vertical tab strict", content: cue + "The fer\vry leaves.\n", want: cue + "The ferry leaves.\n"},
		{name: "escape strict", content: cue + "The fer\x1bry leaves.\n", want: cue + "The ferry leaves.\n"},
		{name: "DEL strict", content: cue + "The fer\x7fry leaves.\n", want: cue + "The ferry leaves.\n"},
		{name: "every control byte", content: "a\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7fb", want: "ab"},
		{
			name:     "lenient warns once with the first line",
			content:  cue + "The fer\fry \fleaves.\n\x7f\n",
			lenient:  true,
			want:     cue + "The ferry leaves.\n\n",
			warnings: []model.ParseWarning{controlWarning(3)},
		},
		{
			name:     "lenient removes a NUL",
			content:  cue + "The fer\x00ry \x00leaves.\n",
			lenient:  true,
			want:     cue + "The ferry leaves.\n",
			warnings: []model.ParseWarning{controlWarning(3)},
		},
		{
			name:     "2 CRs and LF end one line",
			content:  "a\r\r\nb\x01",
			lenient:  true,
			want:     "a\r\r\nb",
			warnings: []model.ParseWarning{controlWarning(2)},
		},
		{
			name:     "2 CRs end 2 lines",
			content:  "a\r\rb\x01",
			lenient:  true,
			want:     "a\r\rb",
			warnings: []model.ParseWarning{controlWarning(3)},
		},
		{
			name:     "line count with CR LF and lone CR",
			content:  "a\r\nb\rc\nd\x01",
			lenient:  true,
			want:     "a\r\nb\rc\nd",
			warnings: []model.ParseWarning{controlWarning(4)},
		},
		{name: "WebVTT NUL strict", content: "H\x00i\n", replaceNUL: true, want: "H\ufffdi\n"},
		{name: "WebVTT NUL lenient", content: "H\x00i\n", lenient: true, replaceNUL: true, want: "H\ufffdi\n"},
		{name: "format characters and tabs stay", content: "\u200eThe\u200dferry\u200f\tleaves.\r\n", want: "\u200eThe\u200dferry\u200f\tleaves.\r\n"},
		{name: "empty", content: "", blank: true},
		{name: "BOM and white space", content: "\ufeff \r\n\t\n", blank: true},
		{name: "BOM and white space lenient", content: "\ufeff \r\n\t\n", lenient: true, blank: true},
		{
			name:     "blank after control characters keeps the warning",
			content:  "\n\x01\n",
			lenient:  true,
			blank:    true,
			warnings: []model.ParseWarning{controlWarning(2)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewState(model.SubRip, model.CommonReadOptions{Lenient: tt.lenient})
			s.ReplaceNUL = tt.replaceNUL
			input := []byte(tt.content)
			got, ok, err := s.Prepare(input, false)
			require.NoError(t, err)
			assert.Equal(t, tt.content, string(input), "Prepare changed its input")
			assert.Equal(t, !tt.blank, ok)
			if !tt.blank {
				assert.Equal(t, tt.want, string(got))
			}
			assert.Equal(t, tt.warnings, s.warnings)
			assert.Len(t, s.Warnings(), len(tt.warnings))
		})
	}
}

// Ports testStrictModeRemovesControlCharactersWithoutWarningAndThrowsForNul
// of tests/Parsers/ControlCharactersTest.php of subtitle-toolbox-php.
func TestPrepareRejectsNULInStrictMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
	}{
		{"line 4", "\n1\n00:00:01,000 --> 00:00:02,000\nThe fer\x00ry leaves.\n", 4},
		{"first byte", "\x00", 1},
		{"after a lone CR", "a\r\x00", 2},
		{"after CR LF", "a\r\nb\r\n\x00", 3},
		{"before a control character", "a\n\x01\n\x00", 3},
		{"after 2 CRs and LF", "a\r\r\nb\x00", 2},
		{"after 3 CRs and LF, then a CR", "a\r\r\r\n\r\x00", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewState(model.SubRip, model.CommonReadOptions{})
			got, ok, err := s.Prepare([]byte(tt.content), false)
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{
				Format: model.SubRip,
				Line:   tt.line,
				Msg:    "Line " + strconv.Itoa(tt.line) + " has a NUL character. Check the encoding of the file.",
			}, *perr)
			assert.Nil(t, got)
			assert.False(t, ok)
			assert.Empty(t, s.warnings)
		})
	}
}

func TestNoCues(t *testing.T) {
	t.Parallel()

	const msg = "The file has text but no cues."
	found := func() (int, bool) { return 2, true }
	tests := []struct {
		name     string
		lenient  bool
		cues     int
		skipped  bool
		textLine func() (int, bool)
		wantErr  bool
		warnings []model.ParseWarning
	}{
		{name: "strict text without cues", textLine: found, wantErr: true},
		{name: "lenient text without cues", lenient: true, textLine: found, warnings: []model.ParseWarning{{Message: msg, Line: 2, Block: model.NoBlock, Action: model.Skipped}}},
		{name: "no text line", textLine: func() (int, bool) { return 0, false }},
		{name: "cues", cues: 1, textLine: func() (int, bool) { panic("the hook runs only without cues") }},
		{
			name:     "a skipped block",
			lenient:  true,
			skipped:  true,
			textLine: found,
			warnings: []model.ParseWarning{{Message: "Block #0 is broken.", Line: 1, Block: 0, Action: model.Skipped}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewState(model.SubRip, model.CommonReadOptions{Lenient: tt.lenient})
			if tt.skipped {
				s.Warn("Block #0 is broken.", 1, 0, nil, model.Skipped)
			}
			err := s.NoCues(tt.cues, tt.textLine)
			if tt.wantErr {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, model.ParseError{Format: model.SubRip, Line: 2, Msg: msg}, *perr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.warnings, s.warnings)
		})
	}
}

// Ports the invalid UTF-8 cases of tests/EncodingTest.php of subtitle-toolbox-php that the pre-pass decides.
func TestPrepareInvalidUTF8(t *testing.T) {
	t.Parallel()

	const (
		content = "\ufeffCaf\u00e9 noir, caf\xE9 noir\x01\n"
		msg     = "The content is not valid UTF-8. The first bad byte is at offset 18. Set the source encoding of the file."
	)
	tests := []struct {
		name     string
		lenient  bool
		replaces bool
		want     string
		warnings []model.ParseWarning
		wantErr  bool
	}{
		{name: "strict keeps the bytes", want: "Caf\u00e9 noir, caf\xE9 noir\n"},
		{
			name:     "lenient keeps the bytes and warns first",
			lenient:  true,
			want:     "Caf\u00e9 noir, caf\xE9 noir\n",
			warnings: []model.ParseWarning{{Message: msg, Line: model.NoLine, Block: model.NoBlock, Action: model.Repaired}, controlWarning(1)},
		},
		{name: "strict fails for a format that replaces", replaces: true, wantErr: true},
		{
			name:     "lenient replaces each bad byte",
			lenient:  true,
			replaces: true,
			want:     "Caf\u00e9 noir, caf\ufffd noir\n",
			warnings: []model.ParseWarning{{Message: msg + " The parser read the bad bytes as U+FFFD.", Line: model.NoLine, Block: model.NoBlock, Action: model.Repaired}, controlWarning(1)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := NewState(model.SAMI, model.CommonReadOptions{Lenient: tt.lenient})
			got, ok, err := s.Prepare([]byte(content), tt.replaces)
			if tt.wantErr {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, model.ParseError{Format: model.SAMI, Msg: msg}, *perr)
				assert.Empty(t, s.warnings)
				return
			}
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.want, string(got))
			assert.Equal(t, tt.warnings, s.warnings)
		})
	}
}

func TestPrepareValidUTF8GivesNoWarning(t *testing.T) {
	t.Parallel()

	for _, replaces := range []bool{false, true} {
		s := NewState(model.SAMI, model.CommonReadOptions{Lenient: true})
		got, ok, err := s.Prepare([]byte("Caf\u00e9 \ufffd\n"), replaces)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Caf\u00e9 \ufffd\n", string(got))
		assert.Empty(t, s.warnings)
	}
}
