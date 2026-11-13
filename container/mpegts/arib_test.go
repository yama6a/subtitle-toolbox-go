package mpegts_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
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

// aribStatement returns the PES data of a caption statement with one text unit.
func aribStatement(codes string) []byte {
	unit := "\x1F\x20" + string([]byte{byte(len(codes) >> 16), byte(len(codes) >> 8), byte(len(codes))}) + codes
	data := "\x3F" + string([]byte{byte(len(unit) >> 16), byte(len(unit) >> 8), byte(len(unit))}) + unit
	group := "\x04\x00\x00" + string([]byte{byte(len(data) >> 8), byte(len(data))}) + data
	return []byte("\x80\xFF\xF0" + group + "\x00\x00")
}

// aribText clears the screen and writes the full width letter A.
const aribText = "\x0C\x9B7 \x53\x1C\x46\x43\x0EA"

func TestARIBTracks(t *testing.T) {
	t.Parallel()

	caption := func(pid int, descriptors string) pmtStream {
		return pmtStream{pid: pid, streamType: 0x06, descriptors: []byte(descriptors)}
	}
	clash := newWriter()
	clash.tables(videoPID,
		teletextStream(teletextPID, ttPage{"jpn", subtitlePageType, 304}),
		caption(304, "\x52\x01\x30\xFD\x03\x00\x08\x3D"))
	clash.packets(304, pes(0xBD, 90000, aribStatement(aribText), 5))

	tests := []struct {
		name string
		file []byte
		want []model.Track
	}{
		{"Japanese management data", fixture(t, "arib_japanese.ts"), []model.Track{
			{Container: model.MPEGTS, Number: 304, CodecID: "arib_caption", Format: model.SubRip, Language: "jpn"},
		}},
		{"Portuguese management data in an M2TS file", fixture(t, "arib_brazilian.m2ts"), []model.Track{
			{Container: model.MPEGTS, Number: 304, CodecID: "arib_caption", Format: model.SubRip, Language: "por"},
		}},
		{"a teletext page with the PID of the stream", clash.bytes(), []model.Track{
			{Container: model.MPEGTS, Number: 304, CodecID: "dvb_teletext", Format: model.SubRip, Language: "jpn"},
			{Container: model.MPEGTS, Number: 304000, CodecID: "arib_caption", Format: model.SubRip, Language: "und"},
		}},
		{"only profile A", func() []byte {
			w := newWriter()
			w.tables(videoPID,
				caption(0x131, "\x52\x01\x38\xFD\x03\x00\x08\x3D"),
				caption(0x132, "\x52\x01\x30\xFD\x03\x00\x12\x3D"),
				caption(0x133, "\x52\x01\x30"))
			w.packets(videoPID, pes(0xE0, 90000, []byte{0}, 5))
			return w.bytes()
		}(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, open(t, tt.file).Tracks())
		})
	}

	s, _, err := open(t, clash.bytes()).Extract(304000, model.CommonReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []cueRow{cue(0, 5, "\uFF21")}, cuesOf(s))
}

func TestARIBWarningsAndJunk(t *testing.T) {
	t.Parallel()

	_, warnings, err := open(t, fixture(t, "arib_japanese.ts")).Extract(304, model.CommonReadOptions{Lenient: true})
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.True(t, strings.HasPrefix(warnings[0].Message, "At 4.500 s: The glyph of the character 0x24 of the set DRCS-1 (16x18 dots, MD5 "))

	clean := fixture(t, "arib_brazilian.m2ts")
	junk := append(append(bytes.Clone(clean[:3*192]), 0, 1, 2), clean[3*192:]...)
	assert.Equal(t, extract(t, clean, 304, model.CommonReadOptions{}), extract(t, junk, 304, model.CommonReadOptions{}))
}

func TestARIBScreenLimit(t *testing.T) {
	t.Parallel()

	codes := "\x0C\x9B1;1 \x57\x9B0 \x58\x9B0 \x59\x0E" + strings.Repeat("\x98\x40A", 600) + strings.Repeat("\x98\x40B", 200)
	w := newWriter()
	w.tables(videoPID, pmtStream{pid: 304, streamType: 0x06, descriptors: []byte("\x52\x01\x30\xFD\x03\x00\x08\x3D")})
	w.packets(304, pes(0xBD, 90000, aribStatement(codes), 5))
	f := open(t, w.bytes())

	_, _, err := f.Extract(304, model.CommonReadOptions{})
	var pe *model.ParseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, "At 0.000 s: The caption screen holds more than 4096 characters. The reader dropped the characters past that.", pe.Msg)

	s, warnings, err := f.Extract(304, model.CommonReadOptions{Lenient: true})
	require.NoError(t, err)
	assert.Len(t, s.Cues(), 1)
	require.Len(t, warnings, 1)
	assert.Equal(t, model.Skipped, warnings[0].Action)
}
