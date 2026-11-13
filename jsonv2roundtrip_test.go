package stbox_test

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// TestLibraryJSONVersion2KeepsTheNativeBytes sends each fixture with words or a speaker through library JSON version 2
// and back to its own format. The bytes equal the direct write of the fixture.
func TestLibraryJSONVersion2KeepsTheNativeBytes(t *testing.T) {
	t.Parallel()

	root := testfiles.Path(t, ".")
	var paths []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir() && d.Name() == "expected":
			return filepath.SkipDir
		case d.Type().IsRegular() && !strings.Contains(d.Name(), ".out."):
			paths = append(paths, path)
		}
		return nil
	}))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		require.NoError(t, err)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			t.Parallel()

			s, info, err := parseBytes(testfiles.Read(t, filepath.ToSlash(rel)), stbox.ReadOptions{})
			if err != nil || info.Format == stbox.JSON || !hasWordsOrSpeaker(s) {
				t.Skip("no words or speaker")
			}
			var direct bytes.Buffer
			if _, err := stbox.Write(&direct, s, info.Format, stbox.WriteOptions{}); err != nil {
				t.Skip("the format does not write the file")
			}
			var library bytes.Buffer
			_, err = stbox.Write(&library, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: 2}})
			require.NoError(t, err)
			back, _, err := parseBytes(library.Bytes(), stbox.ReadOptions{})
			require.NoError(t, err)
			var viaJSON bytes.Buffer
			_, err = stbox.Write(&viaJSON, back, info.Format, stbox.WriteOptions{})
			require.NoError(t, err)
			assert.Equal(t, direct.String(), viaJSON.String())
		})
	}
}

func hasWordsOrSpeaker(s *stbox.Subtitle) bool {
	return slices.ContainsFunc(s.Cues(), func(c stbox.Cue) bool { return len(c.Words) > 0 || c.Speaker != "" })
}
