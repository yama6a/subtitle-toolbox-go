package textenc_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

func TestLookup(t *testing.T) {
	t.Parallel()

	aliases := map[string][]string{
		"UTF-8":        {"UTF8", "utf-8"},
		"Windows-1250": {"CP1250", "windows-1250"},
		"Windows-1258": {"CP1258"},
		"Windows-874":  {"CP874"},
		"ISO-8859-1":   {"ISO8859-1", "LATIN1", "latin1"},
		"ISO-8859-11":  {"ISO8859-11"},
		"ISO-8859-16":  {"ISO8859-16"},
		"Shift_JIS":    {"SHIFT-JIS", "SJIS", "MS_KANJI", "CSSHIFTJIS", "CP932", "Windows-31J", "shift_jis"},
		"GBK":          {"CP936"},
		"Big5":         {"CP950"},
		"CP949":        {"UHC"},
		"CP437":        {"IBM437"},
		"CP850":        {"IBM850"},
		"CP866":        {"IBM866"},
	}
	names := textenc.Names()
	require.Len(t, names, 44)
	assert.Equal(t, []string{"UTF-8", "UTF-16LE", "UTF-16BE", "UTF-16", "UTF-32LE", "UTF-32BE", "UTF-32"}, names[:7])
	for _, name := range names {
		for _, n := range append([]string{name, strings.ToLower(name), strings.ToUpper(name)}, aliases[name]...) {
			e, err := textenc.Lookup(n)
			require.NoError(t, err, n)
			assert.Equal(t, name, e.Name, n)
		}
	}
	for _, name := range []string{"CP1125", "UTF-7", "ISO-8859-12", "UCS-2", ""} {
		_, err := textenc.Lookup(name)
		require.ErrorIs(t, err, textenc.ErrUnknownEncoding, name)
	}
}

// TestDecoderEdges pins the x/text results where glibc iconv, and so PHP, differs. docs/encodings.md lists them.
func TestDecoderEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		encoding string
		in       string
		want     string
		ok       bool
	}{
		{name: "EUC-KR decodes a CP949 syllable", encoding: "EUC-KR", in: "\x8C\x63", want: "\ub620", ok: true},
		{name: "Big5 decodes an HKSCS pair", encoding: "Big5", in: "\x87\x40", want: "\u43f0", ok: true},
		{name: "GBK byte 0x80 is the euro sign", encoding: "GBK", in: "\x80", want: "\u20ac", ok: true},
		{name: "Windows-1255 byte 0xCA", encoding: "Windows-1255", in: "\xCA", want: "\u05ba", ok: true},
		{name: "Windows-874 byte 0x80", encoding: "Windows-874", in: "\x80", want: "\u20ac", ok: true},
		{name: "Windows-874 byte 0x91", encoding: "Windows-874", in: "\x91", want: "\u2018", ok: true},
		{name: "Windows-874 byte 0x81", encoding: "Windows-874", in: "\x81"},
		{name: "Windows-874 byte 0x98", encoding: "Windows-874", in: "\x98"},
		{name: "Windows-1252 byte 0x8D", encoding: "Windows-1252", in: "\x8D"},
		{name: "ISO-8859-1 byte 0x8D", encoding: "ISO-8859-1", in: "\x8D", want: "\u008d", ok: true},
		{name: "Shift_JIS 0x5C stays a backslash", encoding: "Shift_JIS", in: "\\~\x83\x5C", want: "\\~\u30bd", ok: true},
		{name: "Big5 byte 0x80", encoding: "Big5", in: "\x80"},
		{name: "Big5 pair C6 A1", encoding: "CP950", in: "\xC6\xA1", want: "\u2460", ok: true},
		{name: "EUC-JP byte 0x80", encoding: "EUC-JP", in: "\x80"},
		{name: "EUC-JP NEC row 13", encoding: "EUC-JP", in: "\xAD\xA1", want: "\u2460", ok: true},
		{name: "EUC-JP wave dash", encoding: "EUC-JP", in: "\xA1\xC1", want: "\uff5e", ok: true},
		{name: "GBK pair A8 BF", encoding: "GBK", in: "\xA8\xBF", want: "\u01f9", ok: true},
		{name: "GB18030 byte 0x80 is the euro sign", encoding: "GB18030", in: "\x80", want: "\u20ac", ok: true},
		{name: "GB18030 user-defined pair A1 40", encoding: "GB18030", in: "\xA1\x40"},
		{name: "GB18030 pair A3 A0", encoding: "GB18030", in: "\xA3\xA0", want: "\u3000", ok: true},
		{name: "Shift_JIS user-defined pair F0 40", encoding: "Shift_JIS", in: "\xF0\x40"},
		{name: "Shift_JIS byte 0x80", encoding: "CP932", in: "\x80", want: "\u0080", ok: true},
		{name: "KOI8-U byte 0xAE", encoding: "KOI8-U", in: "\xAE", want: "\u045e", ok: true},
		{name: "ISO-8859-2 C1 controls are legal", encoding: "ISO-8859-2", in: "\x80\x9F", want: "\u0080\u009f", ok: true},
		{name: "ISO-8859-3 undefined byte 0xA5", encoding: "ISO-8859-3", in: "\xA5"},
		{name: "ISO-8859-11 byte 0xDB", encoding: "ISO-8859-11", in: "\xDB"},
		{name: "ISO-8859-11 byte 0xFF", encoding: "ISO-8859-11", in: "\xFF"},
		{name: "ISO-8859-11 byte 0xA0", encoding: "ISO-8859-11", in: "\xA0", want: "\u00a0", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, err := textenc.Lookup(tt.encoding)
			require.NoError(t, err)
			got, ok := e.Decode([]byte(tt.in))
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
