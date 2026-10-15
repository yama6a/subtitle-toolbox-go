package testfiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
)

func TestRootFromAPackageFolder(t *testing.T) {
	t.Parallel()

	root := testfiles.Root(t)
	assert.True(t, filepath.IsAbs(root))
	assert.Equal(t, "testdata", filepath.Base(root))
	_, err := os.Stat(filepath.Join(filepath.Dir(root), "go.mod"))
	require.NoError(t, err)
}

func TestPathAndRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rel  string
		want string
	}{
		{"json/SOURCES.md", "# Sources\n"},
		{"cli/SOURCES.md", "# Sources\n"},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, filepath.Join(testfiles.Root(t), filepath.FromSlash(tt.rel)), testfiles.Path(t, tt.rel))
			assert.Equal(t, tt.want, string(testfiles.Read(t, tt.rel)[:len(tt.want)]))
		})
	}
}
