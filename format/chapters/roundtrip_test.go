package chapters_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestRealFilesRoundTrip(t *testing.T) {
	t.Parallel()

	lf := model.CommonWriteOptions{}
	crlfBOM := model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}
	tests := []struct {
		file string
		rd   reader
		wr   writer
		// sameBytes holds the write options under which the written file equals the source, nil when it does not.
		sameBytes *model.CommonWriteOptions
	}{
		{"podcast/real/spec_basic_example.json", chapters.ReadPodcast, chapters.WritePodcast, nil},
		{"podcast/real/spec_complex_example.json", chapters.ReadPodcast, chapters.WritePodcast, nil},
		{"podcast/real/hosting_export.json", chapters.ReadPodcast, chapters.WritePodcast, nil},
		{"ffmetadata/real/ffmpeg_mp4_export.ffmeta", chapters.ReadFFMetadata, chapters.WriteFFMetadata, &lf},
		{"ffmetadata/real/m4b_audiobook.ffmeta", chapters.ReadFFMetadata, chapters.WriteFFMetadata, &lf},
		{"ffmetadata/real/hand_written_crlf.ffmeta", chapters.ReadFFMetadata, chapters.WriteFFMetadata, nil},
		{"ogm/real/mkvextract_simple.txt", chapters.ReadOGM, chapters.WriteOGM, &lf},
		{"ogm/real/windows_tool_crlf_bom.txt", chapters.ReadOGM, chapters.WriteOGM, &crlfBOM},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			source := testfiles.Read(t, "chapters/"+tt.file)
			first, _, err := tt.rd(bytes.NewReader(source), model.CommonReadOptions{}, chapters.ReadOptions{})
			require.NoError(t, err)
			written := writeString(t, tt.wr, first, lf)
			second := readSubtitle(t, tt.rd, written, chapters.ReadOptions{})

			assert.Equal(t, libraryJSON(t, first), libraryJSON(t, second))
			if tt.sameBytes != nil {
				assert.Equal(t, string(source), writeString(t, tt.wr, first, *tt.sameBytes))
			}
		})
	}
}

// libraryJSON returns the library JSON of s, which holds the cues, the metadata and the format data.
func libraryJSON(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	var buf bytes.Buffer
	_, err := stboxjson.Write(&buf, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
	require.NoError(t, err)
	return buf.String()
}
