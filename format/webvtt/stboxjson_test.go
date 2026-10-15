package webvtt_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func readJSON(t *testing.T, content []byte) *model.Subtitle {
	t.Helper()

	s, _, err := stboxjson.Read(bytes.NewReader(content), model.CommonReadOptions{}, stboxjson.ReadOptions{})
	require.NoError(t, err)
	return s
}

// Ports the WebVTT row of testJsonFrom3xKeepsTheStoredPositions of tests/Parsers/JsonRealFilesTest.php of subtitle-toolbox-php.
func TestJSONFrom3xKeepsTheStoredPositions(t *testing.T) {
	t.Parallel()

	fromJSON := readJSON(t, testfiles.Read(t, "json/real/w3c_cue_settings_3x.json"))
	source := readFile(t, "vtt/real/w3c_cue_settings.vtt")
	assert.Equal(t, write(t, source, model.CommonWriteOptions{}), write(t, fromJSON, model.CommonWriteOptions{}))
}

// Ports testJsonFrom3xStillTakesANewAlignment of tests/Parsers/JsonRealFilesTest.php of subtitle-toolbox-php.
func TestJSONFrom3xStillTakesANewAlignment(t *testing.T) {
	t.Parallel()

	s := readJSON(t, testfiles.Read(t, "json/real/w3c_cue_settings_3x.json"))
	c, ok := s.Cue(0)
	require.True(t, ok)
	c.Alignment = model.TopCenter
	require.NoError(t, s.SetCue(0, c))
	assert.Contains(t, write(t, s, model.CommonWriteOptions{}), "00:00:00.000 --> 00:00:04.000 size:35% line:0\n")
}

// Ports testBottomCenterIsWrittenAsNull of tests/Formatters/JsonFormatterTest.php of subtitle-toolbox-php.
func TestBottomCenterIsWrittenAsNull(t *testing.T) {
	t.Parallel()

	s := readFile(t, "vtt/real/w3c_cue_settings.vtt")
	c, _ := s.Cue(2)
	align, _ := cueData(t, c).String("align")
	assert.Equal(t, "center", align)

	var out bytes.Buffer
	require.NoError(t, stboxjson.Write(&out, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{OmitFormatData: true}))
	var doc struct {
		Cues []struct {
			Alignment *int `json:"alignment"`
		} `json:"cues"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	alignments := make([]any, 0, len(doc.Cues))
	for _, c := range doc.Cues {
		if c.Alignment == nil {
			alignments = append(alignments, nil)
			continue
		}
		alignments = append(alignments, *c.Alignment)
	}
	assert.Equal(t, []any{1, 3, nil}, alignments)

	again, ok := readJSON(t, out.Bytes()).Cue(2)
	require.True(t, ok)
	assert.Equal(t, model.BottomCenter, again.Alignment)
}
