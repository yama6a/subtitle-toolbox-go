package codepage

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
)

var testTables = map[string]*Table{ //nolint:gochecknoglobals // test lookup
	"iso8859_5":      ISO8859_5,
	"iso8859_6":      ISO8859_6,
	"iso8859_7_1987": ISO8859_7_1987,
	"iso8859_8_1988": ISO8859_8_1988,
	"cp437":          CP437,
	"cp850":          CP850,
	"cp860":          CP860,
	"cp863":          CP863,
	"cp865":          CP865,
}

func TestPairs(t *testing.T) {
	t.Parallel()

	var pairs []struct {
		Table, Bytes, Text, Mode string
	}
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "codepage/pairs.json"), &pairs))

	for _, p := range pairs {
		t.Run(p.Table+"/"+p.Bytes, func(t *testing.T) {
			t.Parallel()

			b, err := hex.DecodeString(p.Bytes)
			require.NoError(t, err)
			decode, encode := DecodeISO6937, EncodeISO6937
			if p.Table != "iso6937" {
				decode, encode = testTables[p.Table].Decode, testTables[p.Table].Encode
			}
			if p.Mode != "encode" {
				assert.Equal(t, p.Text, decode(b))
			}
			if p.Mode != "decode" {
				assert.Equal(t, b, encode(p.Text))
			}
		})
	}
}

func TestTablesRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		defined int
	}{
		{"iso8859_5", 96},
		{"iso8859_6", 51},
		{"iso8859_7_1987", 90},
		{"iso8859_8_1988", 58},
		{"cp437", 128},
		{"cp850", 128},
		{"cp860", 128},
		{"cp863", 128},
		{"cp865", 128},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			table := testTables[tt.name]
			defined := 0
			for b := 0x80; b <= 0xFF; b++ {
				text := table.Decode([]byte{byte(b)})
				if text == "" {
					continue
				}
				defined++
				assert.Equal(t, []byte{byte(b)}, table.Encode(text), "byte %02X", b)
			}
			assert.Equal(t, tt.defined, defined)
		})
	}
}

func TestISO6937Table(t *testing.T) {
	t.Parallel()

	defined := 0
	for b := 0x20; b <= 0xFF; b++ {
		if isDiacritic(byte(b)) || iso6937Character(byte(b)) == 0 {
			continue
		}
		if b == iso6937Currency || b >= 0x80 {
			defined++
		}
		text := DecodeISO6937([]byte{byte(b)})
		assert.Equal(t, []byte{byte(b)}, EncodeISO6937(text), "byte %02X", b)
	}
	assert.Equal(t, 74, defined)
	assert.Len(t, iso6937Composed, 251)
}

func TestISO6937Composed(t *testing.T) {
	t.Parallel()

	for _, p := range iso6937Composed {
		want := []byte{p.diacritic, p.base}
		assert.Equal(t, want, EncodeISO6937(string(p.letter)), "%U", p.letter)
		assert.Equal(t, string(p.letter), DecodeISO6937(want), "%U", p.letter)
	}
}

func TestEncodableISO6937(t *testing.T) {
	t.Parallel()

	for r := rune(0); r < 0x3000; r++ {
		want := r == Replacement || !strings.ContainsRune(string(EncodeISO6937(string(r))), Replacement)
		assert.Equal(t, want, EncodableISO6937(r), "%U", r)
	}
}

func FuzzISO6937(f *testing.F) {
	f.Add([]byte("Caf\xC2e"), "Café")
	f.Add([]byte("\xC2\xC8\xA6"), "B́́")
	f.Fuzz(func(t *testing.T, b []byte, s string) {
		DecodeISO6937(b)
		out := EncodeISO6937(s)
		if utf8.ValidString(s) {
			assert.LessOrEqual(t, len(out), 2*utf8.RuneCountInString(s))
		}
	})
}

func FuzzTable(f *testing.F) {
	f.Add([]byte("\x80\xA4\xFF"), "a€b")
	f.Fuzz(func(t *testing.T, b []byte, s string) {
		for _, table := range testTables {
			table.Decode(b)
			out := table.Encode(s)
			if utf8.ValidString(s) {
				assert.Len(t, out, utf8.RuneCountInString(s))
			}
		}
	})
}
