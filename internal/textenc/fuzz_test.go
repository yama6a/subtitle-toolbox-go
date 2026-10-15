package textenc_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

func FuzzDecode(f *testing.F) {
	f.Add([]byte("\xFF\xFEH\x00i\x00"), uint8(0))
	f.Add([]byte("\x00H\x00i"), uint8(9))
	f.Add([]byte("Le caf\xE9 est ferm\xE9."), uint8(255))
	f.Add([]byte("\x84\x31\xA4\x37\x80"), uint8(37))
	f.Add([]byte("\x00\x00\xFE\xFF\x00\x11\x00\x00"), uint8(4))

	names := append(textenc.Names(), "")
	f.Fuzz(func(t *testing.T, data []byte, pick uint8) {
		name := names[int(pick)%len(names)]
		got, err := textenc.Decode(data, name)
		if err != nil {
			return
		}
		if !strings.EqualFold(got.Encoding, "UTF-8") && !utf8.Valid(got.Text) {
			t.Fatalf("Decode(%q, %q) read %s and returned invalid UTF-8 %q", data, name, got.Encoding, got.Text)
		}
	})
}
