package parsing_test

import (
	"bytes"
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzPrepare(f *testing.F) {
	f.Add([]byte("\ufeff\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nHi\r\n\x1a"), false, false, false)
	f.Add([]byte("a\rb\x00c\n"), true, false, false)
	f.Add([]byte("WEBVTT\n\nH\x00i\x7f\n"), false, true, false)
	f.Add([]byte("a\x1a b\x1a\n"), true, false, false)
	f.Add([]byte("caf\xe9\n"), false, false, true)
	f.Add([]byte("caf\xe9\xe9\n"), true, false, true)

	f.Fuzz(func(t *testing.T, content []byte, lenient, replaceNUL, replaces bool) {
		s := parsing.NewState(model.SubRip, model.CommonReadOptions{Lenient: lenient})
		s.ReplaceNUL = replaceNUL
		got, ok, err := s.Prepare(bytes.Clone(content), replaces)
		if err != nil {
			nul := !replaceNUL && bytes.ContainsRune(content, 0)
			invalid := replaces && !utf8.Valid(content)
			if lenient || !nul && !invalid || !errors.Is(err, model.ErrParse) {
				t.Fatalf("Prepare(%q) returned %v", content, err)
			}
			return
		}
		for _, w := range s.Warnings() {
			if w.Action != model.Skipped && w.Action != model.Repaired {
				t.Fatalf("Prepare(%q) gave the warning %+v without an action", content, w)
			}
		}
		if !ok {
			return
		}
		for _, b := range got {
			if b < 0x20 && b != '\t' && b != '\n' && b != '\r' || b == 0x7f {
				t.Fatalf("Prepare(%q) kept the control byte %#x", content, b)
			}
		}
		if replaces && !utf8.Valid(got) {
			t.Fatalf("Prepare(%q) kept invalid UTF-8 for a format that replaces it", content)
		}
		if len(bytes.Trim(got, " \t\r\n")) == 0 {
			t.Fatalf("Prepare(%q) returned blank content as text", content)
		}
	})
}
