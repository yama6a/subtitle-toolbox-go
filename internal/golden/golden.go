// Package golden compares test output with golden files under testdata.
package golden

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

const utf8BOM = "\ufeff"

// Equal compares got with the golden file at path. With update set, it writes got to the file first.
func Equal(tb testing.TB, path string, got []byte, update bool) {
	tb.Helper()

	if update {
		write(tb, path, got)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read golden file: %v", err)
		return
	}
	compare(tb, path, want, got)
}

// RoundTrip compares got byte for byte with the golden file at golden when it exists. Otherwise it compares
// the normalised got with the normalised input file. With update set, it writes or deletes the golden file first.
func RoundTrip(tb testing.TB, input, golden string, got []byte, update bool) {
	tb.Helper()

	in, err := os.ReadFile(input)
	if err != nil {
		tb.Fatalf("read input file: %v", err)
		return
	}
	in = Normalize(in)
	if update {
		if bytes.Equal(in, Normalize(got)) {
			if err := os.Remove(golden); err != nil && !errors.Is(err, fs.ErrNotExist) {
				tb.Fatalf("delete golden file: %v", err)
			}
		} else {
			write(tb, golden, got)
		}
	}
	want, err := os.ReadFile(golden)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		compare(tb, input, in, Normalize(got))
	case err != nil:
		tb.Fatalf("read golden file: %v", err)
	default:
		compare(tb, golden, want, got)
	}
}

// Normalize returns b without a leading UTF-8 BOM and with LF line ends.
func Normalize(b []byte) []byte {
	s := strings.TrimPrefix(string(b), utf8BOM)
	return []byte(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n"))
}

func write(tb testing.TB, path string, got []byte) {
	tb.Helper()

	if err := os.WriteFile(path, got, 0o600); err != nil {
		tb.Fatalf("write golden file: %v", err)
	}
}

func compare(tb testing.TB, path string, want, got []byte) {
	tb.Helper()

	if bytes.Equal(want, got) {
		return
	}
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	i := 0
	for i < len(wantLines) && i < len(gotLines) && wantLines[i] == gotLines[i] {
		i++
	}
	tb.Errorf("output differs from %s at line %d:\nwant: %q\ngot:  %q", path, i+1, line(wantLines, i), line(gotLines, i))
}

func line(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(end of file)"
}
