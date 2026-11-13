package subviewer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func readFile(t *testing.T, rel string) *model.Subtitle {
	t.Helper()

	s, warnings, err := subviewer.Read(bytes.NewReader(testfiles.Read(t, rel)), model.CommonReadOptions{}, subviewer.ReadOptions{})
	require.NoError(t, err)
	require.Empty(t, warnings)
	return s
}

type realFile struct {
	name         string
	writerShaped bool
}

// realFiles holds the table of tests/Parsers/SubViewerRealFilesTest.php of subtitle-toolbox-php. A file that is not
// in the shape of the writer gets a header block, or an end line for each cue. The read goldens check what each file
// reads as.
func realFiles() []realFile {
	return []realFile{
		{"subviewer2_crlf.sub", true},
		{"mantas_headers_shape.sub", true},
		{"mantas_plain_shape.sub", false},
		{"subtitle_edit_v1_shape.sub", true},
		{"subviewer1_delay.sub", false},
	}
}

// optionsFor returns the write options that keep the version, the line ending and the BOM of a file.
func optionsFor(content string) (model.CommonWriteOptions, subviewer.WriteOptions) {
	common := model.CommonWriteOptions{BOM: model.BOMRemove}
	if strings.HasPrefix(content, "\xEF\xBB\xBF") {
		common.BOM = model.BOMAdd
	}
	if strings.Contains(content, "\r\n") {
		common.LineEnding = model.CRLF
	}
	version := subviewer.V2
	if strings.Contains(content, startScriptLine) {
		version = subviewer.V1
	}
	return common, subviewer.WriteOptions{Version: version}
}

const startScriptLine = "******** START SCRIPT ********"

// Ports testRealFileSurvivesARoundTrip of tests/Parsers/SubViewerRealFilesTest.php of subtitle-toolbox-php.
func TestRealFilesSurviveARoundTrip(t *testing.T) {
	t.Parallel()

	for _, f := range realFiles() {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()

			content := string(testfiles.Read(t, realDir+"/"+f.name))
			s := readFile(t, realDir+"/"+f.name)
			common, opts := optionsFor(content)
			var out bytes.Buffer
			_, err := subviewer.Write(&out, s, common, opts)
			require.NoError(t, err)
			back, warnings, err := subviewer.Read(bytes.NewReader(out.Bytes()), model.CommonReadOptions{}, subviewer.ReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, cueRows(s), cueRows(back))
			assert.Equal(t, s.AllMetadata(), back.AllMetadata())
			assert.Equal(t, fileData(s), fileData(back))
		})
	}
}

// Ports testRealFileFormatsToItsOwnBytes of tests/Parsers/SubViewerRealFilesTest.php of subtitle-toolbox-php.
func TestRealFilesWriteTheirOwnBytes(t *testing.T) {
	t.Parallel()

	for _, f := range realFiles() {
		if !f.writerShaped {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()

			content := string(testfiles.Read(t, realDir+"/"+f.name))
			common, opts := optionsFor(content)
			var out bytes.Buffer
			_, err := subviewer.Write(&out, readFile(t, realDir+"/"+f.name), common, opts)
			require.NoError(t, err)
			ending := "\n"
			if common.LineEnding == model.CRLF {
				ending = "\r\n"
			}
			assert.Equal(t, strings.TrimRight(content, "\r\n")+ending, out.String())
		})
	}
}

func TestMaxDelayFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_max_delay.sub")
	out := write(t, s, subviewer.WriteOptions{})
	assert.Contains(t, out, "[99999:59:59]\n\n[end]")
	back, _ := mustRead(t, out, false)
	assert.Equal(t, cueRows(s), cueRows(back))
}
