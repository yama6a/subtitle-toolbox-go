package textenc_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

const fixtures = "../../testdata/encoding"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(fixtures, filepath.FromSlash(name)))
	require.NoError(t, err)
	return data
}

type texts struct {
	KoreanCues  []string `json:"koreanCues"`
	KoreanLine  string   `json:"koreanLine"`
	ChineseLine string   `json:"chineseLine"`
	RussianLine string   `json:"russianLine"`
}

func readTexts(t *testing.T) texts {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "texts.json"))
	require.NoError(t, err)
	var tx texts
	require.NoError(t, json.Unmarshal(data, &tx))
	return tx
}

// Ports the cases of tests/EncodingTest.php of subtitle-toolbox-php that the decode steps decide.
func TestDecode(t *testing.T) {
	t.Parallel()

	const (
		noBOM    = "The content is UTF-16BE without a BOM."
		noBOMLE  = "The content is UTF-16LE without a BOM."
		detected = "The content is not UTF-8. Detection picked Windows-1252. Set the source encoding if that is wrong."
	)
	tests := []struct {
		name     string
		data     string
		source   string
		text     string
		encoding string
		warning  string
	}{
		{name: "1 UTF-8 BOM stays", data: "\xEF\xBB\xBFcaf\xE9", source: "Windows-1252", text: "\xEF\xBB\xBFcaf\xE9", encoding: "UTF-8"},
		{name: "2 UTF-32BE BOM", data: "\x00\x00\xFE\xFF\x00\x00\x00A", text: "A", encoding: "UTF-32BE"},
		{name: "2 UTF-32LE BOM before the UTF-16LE BOM", data: "\xFF\xFE\x00\x00A\x00\x00\x00", text: "A", encoding: "UTF-32LE"},
		{name: "2 UTF-16BE BOM", data: "\xFE\xFF\x00H\x00i", text: "Hi", encoding: "UTF-16BE"},
		{name: "2 UTF-16LE BOM wins over the source encoding", data: "\xFF\xFEH\x00\xE9\x00", source: "Windows-1252", text: "H\u00e9", encoding: "UTF-16LE"},
		{name: "2 UTF-16 surrogate pair", data: "\xFF\xFE\x3D\xD8\x00\xDE", text: "\U0001F600", encoding: "UTF-16LE"},
		{name: "3 UTF-16BE without BOM", data: "\x00H\x00i", text: "Hi", encoding: "UTF-16BE", warning: noBOM},
		{name: "3 UTF-16BE without BOM before the source encoding", data: "\x00H\x00i", source: "Windows-1252", text: "Hi", encoding: "UTF-16BE", warning: noBOM},
		{name: "3 UTF-16LE without BOM", data: "H\x00i\x00", source: "utf-8", text: "Hi", encoding: "UTF-16LE", warning: noBOMLE},
		{name: "3 a wide source encoding stops the sniff", data: "\x00H\x00i", source: "UTF-16LE", text: "\u4800\u6900", encoding: "UTF-16LE"},
		{name: "8 UTF-16 reads little endian", data: "\x00H\x00i", source: "utf-16", text: "\u4800\u6900", encoding: "utf-16"},
		{name: "8 UTF-32 reads little endian", data: "A\x00\x00\x00", source: "UTF-32", text: "A", encoding: "UTF-32"},
		{name: "3 odd length is not UTF-16", data: "\x00H\x00", text: "\x00H\x00", encoding: "UTF-8"},
		{name: "4 valid UTF-8", data: "caf\u00e9", text: "caf\u00e9", encoding: "UTF-8"},
		{name: "4 a NUL byte keeps the bytes", data: "caf\xE9\x00", text: "caf\xE9\x00", encoding: "UTF-8"},
		{name: "4 mostly UTF-8 keeps the bytes", data: "Caf\u00e9 cr\u00e8me, caf\xE9 noir", text: "Caf\u00e9 cr\u00e8me, caf\xE9 noir", encoding: "UTF-8"},
		{name: "4 empty", data: "", text: "", encoding: "UTF-8"},
		{name: "5 detection", data: "Le caf\xE9 est ferm\xE9.", text: "Le caf\u00e9 est ferm\u00e9.", encoding: "Windows-1252", warning: detected},
		{name: "6 UTF8 keeps bad bytes", data: "caf\xE9", source: "utf8", text: "caf\xE9", encoding: "UTF-8"},
		{name: "7 ASCII read with a code page is UTF-8", data: "Hello", source: "Windows-1252", text: "Hello", encoding: "UTF-8"},
		{name: "7 UTF-8 read with a code page is UTF-8", data: "caf\u00e9", source: "Windows-1256", text: "caf\u00e9", encoding: "UTF-8"},
		{name: "8 the chosen encoding, named as given", data: "caf\xE9", source: "windows-1252", text: "caf\u00e9", encoding: "windows-1252"},
		{name: "8 Windows-1250 wins over detection", data: "O\xF9 \xEAtes", source: "Windows-1250", text: "O\u016f \u0119tes", encoding: "Windows-1250"},
		{name: "8 NUL bytes go to the chosen encoding", data: "A\x00\x00\x00", source: "UTF-32LE", text: "A", encoding: "UTF-32LE"},
		{name: "8 GB18030 holds a real U+FFFD", data: "\x84\x31\xA4\x37", source: "GB18030", text: "\ufffd", encoding: "GB18030"},
		{name: "8 ISO-8859-1 C1 controls are legal", data: "\x81\xE9", source: "ISO-8859-1", text: "\u0081\u00e9", encoding: "ISO-8859-1"},
		{name: "8 ISO-8859-11", data: "\xA1\x80", source: "ISO-8859-11", text: "\u0e01\u0080", encoding: "ISO-8859-11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := textenc.Decode([]byte(tt.data), tt.source)
			require.NoError(t, err)
			assert.Equal(t, tt.text, string(got.Text))
			assert.Equal(t, tt.encoding, got.Encoding)
			assert.Equal(t, tt.warning, got.Warning)
		})
	}
}

func TestDecodeFails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     string
		source   string
		encoding string
	}{
		{name: "Windows-1252 byte 0x81", data: "caf\xE9\x81", source: "Windows-1252", encoding: "Windows-1252"},
		{name: "Windows-874 byte 0x81", data: "\x81", source: "cp874", encoding: "cp874"},
		{name: "odd UTF-16 length", data: "\xFF\xFEA", encoding: "UTF-16LE"},
		{name: "unpaired high surrogate", data: "\xFF\xFE\x00\xD8A\x00", encoding: "UTF-16LE"},
		{name: "unpaired low surrogate", data: "\xFE\xFF\xDC\x00", encoding: "UTF-16BE"},
		{name: "high surrogate at the end", data: "\xFE\xFF\x00A\xD8\x00", encoding: "UTF-16BE"},
		{name: "UTF-32 length", data: "\x00\x00\xFE\xFF\x00\x00\x00", encoding: "UTF-32BE"},
		{name: "UTF-32 above U+10FFFF", data: "\x00\x00\xFE\xFF\x00\x11\x00\x00", encoding: "UTF-32BE"},
		{name: "UTF-32 surrogate", data: "\xFF\xFE\x00\x00\x00\xD8\x00\x00", encoding: "UTF-32LE"},
		{name: "UTF-16 sniff with an unpaired surrogate", data: strings.Repeat("\x00a", 20) + "\xD8\x00\x00!", encoding: "UTF-16BE"},
		{name: "GB18030 undefined byte", data: "ab\xFF", source: "GB18030", encoding: "GB18030"},
		{name: "ISO-8859-11 undefined byte", data: "ab\xDB", source: "ISO-8859-11", encoding: "ISO-8859-11"},
		{name: "ISO-8859-8 undefined byte", data: "ab\xA1", source: "ISO-8859-8", encoding: "ISO-8859-8"},
		{name: "Shift_JIS cut character", data: "ab\x82", source: "Shift_JIS", encoding: "Shift_JIS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := textenc.Decode([]byte(tt.data), tt.source)
			var cerr *textenc.ConvertError
			require.ErrorAs(t, err, &cerr)
			assert.Equal(t, tt.encoding, cerr.Encoding)
			assert.Equal(t, "Cannot convert the content from "+tt.encoding+" to UTF-8.", err.Error())
		})
	}
}

func TestDecodeUnknownName(t *testing.T) {
	t.Parallel()

	_, err := textenc.Decode([]byte("Hello"), "CP1125")
	require.ErrorIs(t, err, textenc.ErrUnknownEncoding)
}

// Ports testShiftJisKeepsTheBackslashesOfAss and testCp932KeepsTheBackslashesOfAss of tests/EncodingTest.php
// of subtitle-toolbox-php.
func TestShiftJISKeepsBackslashes(t *testing.T) {
	t.Parallel()

	want := readDetected(t)
	for _, file := range []string{"japanese-cp932.ass", "japanese-shift_jis.srt"} {
		for _, source := range []string{"", "Shift_JIS", "SJIS", "shift_jis", "CP932"} {
			t.Run(file+", "+source, func(t *testing.T) {
				t.Parallel()

				got, err := textenc.Decode(readFixture(t, file), source)
				require.NoError(t, err)
				text := strings.ReplaceAll(string(got.Text), "\r\n", "\n")
				assert.Contains(t, text, want[file].Text)
				if source != "" {
					assert.Equal(t, source, got.Encoding)
				}
				if strings.HasSuffix(file, ".ass") {
					assert.Contains(t, text, `\N`)
					assert.Contains(t, text, `{\i1}`)
				}
			})
		}
	}
}

// Ports testUtf16WithoutBomKeepsAnExplicitUtf16SourceEncoding of tests/EncodingTest.php of subtitle-toolbox-php.
// glibc iconv, and so PHP, reads UTF-16 without a BOM as little endian on x86-64 and arm64.
func TestDecodeNamedUTF16WithoutBOM(t *testing.T) {
	t.Parallel()

	for _, source := range []string{"UTF-16", "UTF-16LE"} {
		got, err := textenc.Decode(readFixture(t, "utf-16-no-bom/utf-16le-no-bom.srt"), source)
		require.NoError(t, err)
		assert.Equal(t, source, got.Encoding)
		assert.Empty(t, got.Warning)
		assert.Contains(t, string(got.Text), "The caf\u00e9 opens at nine.")
	}
}

// Ports testTextInAnotherEncodingFamilyKeepsItsBytes of tests/EncodingTest.php of subtitle-toolbox-php.
func TestUndetectedTextKeepsItsBytes(t *testing.T) {
	t.Parallel()

	line, err := charmap.CodePage866.NewEncoder().String(readTexts(t).RussianLine)
	require.NoError(t, err)
	data := []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\n" + line + "\r\n")
	got, err := textenc.Decode(data, "")
	require.NoError(t, err)
	assert.Equal(t, data, got.Text)
	assert.Equal(t, "UTF-8", got.Encoding)
	assert.Empty(t, got.Warning)
}

// Ports testUtf16FileWithoutBomParses and testUtf16LeFileWithBomParsesWithoutSourceEncoding of
// tests/EncodingTest.php of subtitle-toolbox-php for the decode step.
func TestDecodeUTF16Files(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(filepath.Join(fixtures, "utf-16-no-bom"))
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() == "SOURCES.md" {
			continue
		}
		name := "utf-16-no-bom/" + entry.Name()
		for _, source := range []string{"", "Windows-1252"} {
			t.Run(name+", "+source, func(t *testing.T) {
				t.Parallel()

				encoding := strings.ToUpper(strings.Split(entry.Name(), "-no-bom")[0])
				got, err := textenc.Decode(readFixture(t, name), source)
				require.NoError(t, err)
				assert.Equal(t, encoding, got.Encoding)
				assert.Equal(t, "The content is "+encoding+" without a BOM.", got.Warning)
				assert.Contains(t, string(got.Text), "The caf\u00e9 opens at nine.")
				assert.Contains(t, string(got.Text), "Gr\u00fc\u00dfe from the harbor.")
			})
		}
	}
	for _, source := range []string{"", "Windows-1252"} {
		got, err := textenc.Decode(readFixture(t, "notepad-utf-16le.vtt"), source)
		require.NoError(t, err)
		assert.Equal(t, "UTF-16LE", got.Encoding)
		assert.Empty(t, got.Warning)
		assert.Contains(t, string(got.Text), "Gr\u00fc\u00dfe aus K\u00f6ln!")
	}
}

// glibc iconv reads the pair C8 A5 of this file as private use, and the x/text Big5 table has no entry for it.
// docs/encodings.md lists the difference.
func TestGBKFileReadAsBig5Fails(t *testing.T) {
	t.Parallel()

	_, err := textenc.Decode(readFixture(t, "chinese-gbk.srt"), "Big5")
	require.EqualError(t, err, "Cannot convert the content from Big5 to UTF-8.")
}
