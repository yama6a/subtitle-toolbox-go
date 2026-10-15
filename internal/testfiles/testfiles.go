// Package testfiles finds the testdata folder at the module root for tests in any package.
package testfiles

import (
	"os"
	"path/filepath"
	"testing"
)

// Root returns the absolute path of the testdata folder at the module root.
func Root(tb testing.TB) string {
	tb.Helper()

	dir, err := os.Getwd()
	if err != nil {
		tb.Fatalf("find the testdata folder: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatalf("find the testdata folder: no go.mod above the working directory")
			return ""
		}
		dir = parent
	}
}

// Path returns the absolute path of a file under testdata, for example Path(t, "srt/real/own_styled.srt").
func Path(tb testing.TB, rel string) string {
	tb.Helper()

	return filepath.Join(Root(tb), filepath.FromSlash(rel))
}

// Read returns the content of a file under testdata.
func Read(tb testing.TB, rel string) []byte {
	tb.Helper()

	content, err := os.ReadFile(Path(tb, rel))
	if err != nil {
		tb.Fatalf("read test file: %v", err)
	}
	return content
}
