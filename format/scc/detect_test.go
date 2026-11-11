package scc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
)

// Rows of the SCC signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"header and code line", "Scenarist_SCC V1.0\n\n00:00:01:00\t942c 942c\n", true},
		{"header only", "Scenarist_SCC V1.0", true},
		{"spaces and tabs after the header", "Scenarist_SCC V1.0 \t\n", true},
		{"other version", "Scenarist_SCC V2.0\n", false},
		{"lower case", "scenarist_scc v1.0\n", false},
		{"text after the header", "Scenarist_SCC V1.0 x\n", false},
		{"header on the second line", "\nScenarist_SCC V1.0\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, scc.Detect([]byte(tt.content)))
		})
	}
}

func TestDetectRealFiles(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(testfiles.Path(t, "scc/real/*.scc"))
	require.NoError(t, err)
	require.Len(t, paths, 9)
	for _, p := range paths {
		content, err := os.ReadFile(p)
		require.NoError(t, err)
		assert.True(t, scc.Detect([]byte(strings.ReplaceAll(string(content), "\r\n", "\n"))), filepath.Base(p))
	}
}
