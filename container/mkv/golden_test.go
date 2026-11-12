package mkv_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// write writes s in the format f without a BOM, as the PHP goldens are.
func write(tb testing.TB, s *model.Subtitle, f model.Format) string {
	tb.Helper()
	var buf bytes.Buffer
	opts := model.CommonWriteOptions{BOM: model.BOMRemove}
	var err error
	switch f { //nolint:exhaustive // the tests write 3 formats
	case model.SubRip:
		_, err = srt.Write(&buf, s, opts, srt.WriteOptions{})
	case model.ASS:
		_, err = ass.Write(&buf, s, opts, ass.WriteOptions{})
	default:
		_, err = webvtt.Write(&buf, s, opts, webvtt.WriteOptions{})
	}
	require.NoError(tb, err)
	return buf.String()
}

func TestReadGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file   string
		track  int
		format model.Format
		golden string
	}{
		{"mkvmerge.mkv", 2, model.SubRip, "mkvmerge.mkv.2.srt"},
		{"mkvmerge.mkv", 3, model.ASS, "mkvmerge.mkv.3.ass"},
		{"ffmpeg_srt.mkv", 2, model.SubRip, "ffmpeg_srt.mkv.2.srt"},
		{"ffmpeg_webvtt.webm", 2, model.WebVTT, "ffmpeg_webvtt.webm.2.vtt"},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := openTestdata(t, tt.file).Extract(tt.track, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, string(testfiles.Read(t, "mkv/expected/"+tt.golden)), write(t, s, tt.format))
		})
	}
}
