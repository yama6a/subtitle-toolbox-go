package stbox_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// Ports testAegisubFileKeepsMetadataCommentsAndFormatData of tests/Parsers/JsonRealFilesTest.php of
// subtitle-toolbox-php. Both reads detect the format.
func TestAegisubJSONEqualsTheASSFile(t *testing.T) {
	t.Parallel()

	fromJSON, info, err := parseBytes(testfiles.Read(t, "json/real/own_aegisub.json"), stbox.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, stbox.JSON, info.Format)
	original, info, err := parseBytes(testfiles.Read(t, "ass/real/own_aegisub.ass"), stbox.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, stbox.ASS, info.Format)

	library := func(s *stbox.Subtitle) string {
		var out bytes.Buffer
		_, err := stbox.Write(&out, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: 1}})
		require.NoError(t, err)
		return out.String()
	}
	assert.Equal(t, library(original), library(fromJSON))
	title, _ := fromJSON.Metadata(stbox.MetadataTitle)
	assert.Equal(t, "Morning train to the coast", title)
	assert.Len(t, fromJSON.Comments(), 3)
	d, _ := fromJSON.FormatData("ass")
	section, _ := d.String("stylesSection")
	assert.Equal(t, "V4+ Styles", section)
}
