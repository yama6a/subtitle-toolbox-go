package webvtt_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func realFiles(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(testfiles.Path(t, filepath.Join(realDir, "*.vtt")))
	require.NoError(t, err)
	require.Len(t, paths, 30)
	rels := make([]string, len(paths))
	for i, p := range paths {
		rels[i] = filepath.Join(realDir, filepath.Base(p))
	}
	return rels
}

func read(t *testing.T, content []byte, lenient bool) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := webvtt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, webvtt.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, _ := read(t, testfiles.Read(t, rel), false)
	return s
}
