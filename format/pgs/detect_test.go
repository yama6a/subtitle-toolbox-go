package pgs_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	header := func(typ byte) []byte {
		return []byte{'P', 'G', 0x0D, 0x0A, 0, 0, 0, 0, 0, 0, typ}
	}
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{name: "palette segment", content: header(0x14), want: true},
		{name: "object segment", content: header(0x15), want: true},
		{name: "composition segment, PTS with CR LF", content: header(0x16), want: true},
		{name: "PTS 0x000D0A00 that PHP does not detect", content: []byte("PG\x00\x0D\x0A\x00\x00\x00\x00\x00\x16"), want: true},
		{name: "window segment", content: header(0x17), want: true},
		{name: "end segment", content: header(0x80), want: true},
		{name: "unknown segment type", content: header(0x13)},
		{name: "10 bytes", content: header(0x16)[:10]},
		{name: "other magic", content: append([]byte("PX"), header(0x16)[2:]...)},
		{name: "BOM before the header", content: append([]byte("\xEF\xBB\xBF"), header(0x16)...)},
		{name: "empty"},
		{name: "SubRip", content: []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n")},
		{name: "WebVTT", content: []byte("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, pgs.Detect(tt.content))
		})
	}
}

func TestDetectFindsEveryFixture(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "pgs/*.sup"))
	require.NoError(t, err)
	require.Len(t, files, 6)
	for _, f := range files {
		assert.True(t, pgs.Detect(testfiles.Read(t, "pgs/"+filepath.Base(f))), f)
	}
}
