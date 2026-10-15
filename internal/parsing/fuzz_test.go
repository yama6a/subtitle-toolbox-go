package parsing_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzPrepare(f *testing.F) {
	f.Add([]byte("\ufeff\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n\x1a"), false, false)
	f.Add([]byte("a\rb\x00c\n"), true, false)
	f.Add([]byte("WEBVTT\n\nH\x00i\x7f\n"), false, true)
	f.Add([]byte("a\x1a b\x1a\n"), true, false)

	f.Fuzz(func(t *testing.T, content []byte, lenient, replaceNUL bool) {
		s := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: lenient})
		s.ReplaceNUL = replaceNUL
		got, ok, err := s.Prepare(bytes.Clone(content), false)
		if err != nil {
			if lenient || replaceNUL || !bytes.ContainsRune(content, 0) || !errors.Is(err, model.ErrParse) {
				t.Fatalf("Prepare(%q) returned %v", content, err)
			}
			return
		}
		if !ok {
			return
		}
		for _, b := range got {
			if b < 0x20 && b != '\t' && b != '\n' && b != '\r' || b == 0x7f {
				t.Fatalf("Prepare(%q) kept the control byte %#x", content, b)
			}
		}
		if len(bytes.Trim(got, " \t\r\n")) == 0 {
			t.Fatalf("Prepare(%q) returned blank content as text", content)
		}
	})
}
