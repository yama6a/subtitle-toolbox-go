package mpegts_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// subtitling returns a subtitling descriptor with one entry for each service: language, type, composition and ancillary page.
func subtitling(services ...[4]any) []byte {
	var entries []byte
	for _, s := range services {
		entries = append(entries, s[0].(string)...)
		entries = append(entries, byte(s[1].(int)), byte(s[2].(int)>>8), byte(s[2].(int)), byte(s[3].(int)>>8), byte(s[3].(int)))
	}
	return append([]byte{0x59, byte(len(entries))}, entries...)
}

func dvbTrack(number int, language, name string) model.Track {
	return model.Track{Container: model.MPEGTS, Number: number, CodecID: "dvb_subtitle", Format: model.PGS, Language: language, Name: name}
}

func TestDVBTracks(t *testing.T) {
	t.Parallel()

	t.Run("dvb_subtitles.ts", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []model.Track{dvbTrack(257, "deu", ""), dvbTrack(258, "eng", "hearing impaired")}, open(t, fixture(t, "dvb_subtitles.ts")).Tracks())
	})

	t.Run("numbers, languages and names", func(t *testing.T) {
		t.Parallel()

		w := newWriter()
		w.tables(1, pmtPID, videoPID,
			pmtStream{pid: 300, streamType: 0x06, descriptors: append(
				subtitling([4]any{"GER", 0x10, 1, 1}, [4]any{"g1r", 0x20, 2, 1}),
				subtitling([4]any{"fra", 0x25, 3, 1}, [4]any{"spa", 0x26, 4, 1})...)},
			pmtStream{pid: 888, streamType: 0x06, descriptors: subtitling([4]any{"eng", 0x10, 1, 1}, [4]any{"eng", 0x10, 2, 1})},
			teletextStream(600, eng888()),
		)
		w.packets(videoPID, make([]byte, 400))
		assert.Equal(t, []model.Track{
			{Container: model.MPEGTS, Number: 888, CodecID: "dvb_teletext", Format: model.SubRip, Language: "eng"},
			dvbTrack(300, "ger", ""), dvbTrack(300001, "und", "hearing impaired"), dvbTrack(300002, "fra", "hearing impaired"), dvbTrack(300003, "spa", ""),
			dvbTrack(888000, "eng", ""), dvbTrack(888001, "eng", ""),
		}, open(t, w.bytes()).Tracks())
	})
}
