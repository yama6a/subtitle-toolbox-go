package subviewer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
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
	cues         int
	first, last  cueRow
	writerShaped bool
}

// realFiles holds the table of tests/Parsers/SubViewerRealFilesTest.php of subtitle-toolbox-php. A file that is not
// in the shape of the writer gets a header block, or an end line for each cue.
func realFiles() []realFile {
	return []realFile{
		{
			"subviewer2_crlf.sub", 6,
			cueRow{1500 * ms, 4 * time.Second, []string{"The bakery opens at six."}},
			cueRow{3600 * time.Second, 3603500 * ms, []string{"See you tomorrow morning."}},
			true,
		},
		{
			"mantas_headers_shape.sub", 2,
			cueRow{137440 * ms, 140380 * ms, []string{"Rain moves in from the west", "in the late afternoon."}},
			cueRow{3740480 * ms, 3742500 * ms, []string{"Tomorrow stays dry."}},
			true,
		},
		{
			"mantas_plain_shape.sub", 2,
			cueRow{137400 * ms, 140400 * ms, []string{"The train to the harbour", "leaves from platform 2."}},
			cueRow{3740500 * ms, 3742500 * ms, []string{"Mind the gap."}},
			false,
		},
		{
			"subtitle_edit_v1_shape.sub", 4,
			cueRow{2 * time.Second, 5 * time.Second, []string{"The ferry leaves at nine."}},
			cueRow{90 * time.Second, 94 * time.Second, []string{"The last boat returns at six."}},
			true,
		},
		{
			"subviewer1_delay.sub", 4,
			cueRow{3 * time.Second, 6 * time.Second, []string{"Sunny in the north", "and cloudy in the south."}},
			cueRow{17 * time.Second, 22 * time.Second, []string{"Back at eight with the news."}},
			false,
		},
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

// Ports testRealFileParses of tests/Parsers/SubViewerRealFilesTest.php of subtitle-toolbox-php.
func TestRealFilesRead(t *testing.T) {
	t.Parallel()

	for _, f := range realFiles() {
		t.Run(f.name, func(t *testing.T) {
			t.Parallel()

			rows := cueRows(readFile(t, realDir+"/"+f.name))
			require.Len(t, rows, f.cues)
			assert.Equal(t, f.first, rows[0])
			assert.Equal(t, f.last, rows[len(rows)-1])
		})
	}
}

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
			require.NoError(t, subviewer.Write(&out, s, common, opts))
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
			require.NoError(t, subviewer.Write(&out, readFile(t, realDir+"/"+f.name), common, opts))
			ending := "\n"
			if common.LineEnding == model.CRLF {
				ending = "\r\n"
			}
			assert.Equal(t, strings.TrimRight(content, "\r\n")+ending, out.String())
		})
	}
}

// Ports testSubViewer2HeaderGoesToMetadataAndFormatData of tests/Parsers/SubViewerRealFilesTest.php.
func TestVersion2HeaderOfARealFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/subviewer2_crlf.sub")
	assert.Equal(t, map[string]string{"title": "Morning at the bakery", "author": "Jane Doe"}, s.AllMetadata())
	assert.Equal(t, map[string]any{
		"version": int64(2),
		"header": map[string]any{
			"SOURCE": "", "PRG": "", "FILEPATH": "", "DELAY": "0", "CD TRACK": "0", "COMMENT": "Times in centiseconds",
		},
		"style": "[COLF]&HFFFFFF,[STYLE]bd,[SIZE]18,[FONT]Arial",
	}, fileData(s))
	assert.Equal(t, []string{"Flour &amp; water &lt;and&gt; a pinch of salt."}, s.Cues()[2].Lines)
}

// Ports testSubViewer1DelayIsAddedToEveryTime of tests/Parsers/SubViewerRealFilesTest.php.
func TestVersion1DelayOfARealFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/subviewer1_delay.sub")
	assert.Equal(t, []cueRow{
		{3 * time.Second, 6 * time.Second, []string{"Sunny in the north", "and cloudy in the south."}},
		{8 * time.Second, 11 * time.Second, []string{"Light wind from the east."}},
		{11 * time.Second, 14 * time.Second, []string{"Snow on the hills tonight."}},
		{17 * time.Second, 22 * time.Second, []string{"Back at eight with the news."}},
	}, cueRows(s))
	assert.Equal(t, map[string]any{"version": int64(1), "header": map[string]any{"DELAY": "0"}}, fileData(s))
	assert.Equal(t, map[string]string{"title": "Weather report", "author": "Tom Berg"}, s.AllMetadata())
}

func TestMaxDelayFile(t *testing.T) {
	t.Parallel()

	s := readFile(t, realDir+"/own_max_delay.sub")
	last := s.Cues()[s.Len()-1]
	assert.Equal(t, 100000*time.Hour-time.Second, last.End)
	assert.Equal(t, 100000*time.Hour-time.Second, last.Start+3*time.Second)
	out := write(t, s, subviewer.WriteOptions{})
	assert.Contains(t, out, "[99999:59:59]\n\n[end]")
	back, _ := mustRead(t, out, false)
	assert.Equal(t, cueRows(s), cueRows(back))
}
