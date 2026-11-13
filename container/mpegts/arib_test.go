package mpegts_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestARIBReadGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file, golden string
		write        func(*bytes.Buffer, *model.Subtitle) error
	}{
		{"arib_japanese.ts", "arib_japanese.ts.304.to.vtt", func(b *bytes.Buffer, s *model.Subtitle) error {
			_, err := webvtt.Write(b, s, model.CommonWriteOptions{}, webvtt.WriteOptions{})
			return err
		}},
		{"arib_brazilian.m2ts", "arib_brazilian.m2ts.304.to.srt", func(b *bytes.Buffer, s *model.Subtitle) error {
			_, err := srt.Write(b, s, model.CommonWriteOptions{}, srt.WriteOptions{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			t.Parallel()

			s, _, err := open(t, fixture(t, tt.file)).Extract(304, model.CommonReadOptions{})
			require.NoError(t, err)
			var got bytes.Buffer
			require.NoError(t, tt.write(&got, s))
			assert.Equal(t, string(testfiles.Read(t, "mpegts/expected/"+tt.golden)), got.String())
		})
	}
}
