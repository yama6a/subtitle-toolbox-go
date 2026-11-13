package stbox_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// A PGS header holds a 32-bit PTS and DTS, so byte 10 moves when Detect trims white space or changes CR LF to LF.
func TestDetectReadsPGSFromTheRawBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content []byte
		want    stbox.Format
	}{
		{name: "fixture", content: testfiles.Read(t, "pgs/shapes_576p.sup"), want: stbox.PGS},
		{name: "CR LF in the PTS", content: []byte("PG\r\n\r\n\x00\x00\x00\x00\x16\x00\x00"), want: stbox.PGS},
		{name: "BOM before the header", content: append([]byte("\xEF\xBB\xBF"), testfiles.Read(t, "pgs/shapes_576p.sup")...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d, ok := stbox.Detect(tt.content)
			assert.Equal(t, tt.want != "", ok)
			assert.Equal(t, tt.want, d.Format)
		})
	}
}

func TestLoadReadsPGSByContentAndExtension(t *testing.T) {
	t.Parallel()

	path := testfiles.Path(t, "pgs/shapes_576p.sup")
	s, info, err := stbox.Load(path, stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.PGS, info.Format)
	assert.Empty(t, info.SourceEncoding)
	assert.Equal(t, 2, s.Len())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	s, info, err = parseBytes(content, stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.PGS, info.Format)
	assert.Equal(t, 2, s.Len())
}
