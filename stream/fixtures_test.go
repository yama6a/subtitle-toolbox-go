package stream_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

// fixtureFiles returns the paths, relative to testdata, of every file with the extension ext.
func fixtureFiles(tb testing.TB, ext string) []string {
	tb.Helper()

	root := testfiles.Root(tb)
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == ext {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(tb, err)
	require.NotEmpty(tb, files)
	return files
}

func TestSubRipReaderMatchesTheFullReaderOnEveryFixture(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".srt") {
		content := testfiles.Read(t, rel)
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(rel+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareSubRip(t, content, w.wrap, lenient)
				})
			}
		}
	}
}

func TestWebVTTReaderMatchesTheFullReaderOnEveryFixture(t *testing.T) {
	t.Parallel()

	for _, rel := range fixtureFiles(t, ".vtt") {
		content := testfiles.Read(t, rel)
		for _, w := range wraps {
			for _, lenient := range []bool{false, true} {
				t.Run(rel+"/"+w.name+"/"+modeName(lenient), func(t *testing.T) {
					t.Parallel()

					compareWebVTT(t, content, w.wrap, lenient)
				})
			}
		}
	}
}

func modeName(lenient bool) string {
	if lenient {
		return "lenient"
	}
	return "strict"
}
