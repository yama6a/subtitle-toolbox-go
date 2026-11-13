package mp4_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestReadGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		file   string
		track  int
		golden string
		write  func(*bytes.Buffer, *model.Subtitle) error
	}{
		{"tx3g as SubRip", "ffmpeg_mov_text.mp4", 2, "ffmpeg_mov_text.mp4.2.srt", func(b *bytes.Buffer, s *model.Subtitle) error {
			_, err := srt.Write(b, s, model.CommonWriteOptions{BOM: model.BOMRemove}, srt.WriteOptions{})
			return err
		}},
		{"c608 as SCC", "ffmpeg_c608.mov", 1, "ffmpeg_c608.mov.1.scc", func(b *bytes.Buffer, s *model.Subtitle) error {
			_, err := scc.Write(b, s, model.CommonWriteOptions{}, scc.WriteOptions{})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := extract(t, openFixture(t, tt.file), tt.track, mp4.ExtractOptions{})
			var got bytes.Buffer
			require.NoError(t, tt.write(&got, s))
			assert.Equal(t, string(testfiles.Read(t, "mp4/expected/"+tt.golden)), got.String())
		})
	}
}

func TestTracks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file string
		want []model.Track
	}{
		{"text_tracks.mp4", []model.Track{
			{Container: model.MP4, Number: 2, CodecID: "tx3g", Format: model.SubRip, Language: "eng", Name: "English", Default: true},
			{Container: model.MP4, Number: 3, CodecID: "tx3g", Format: model.SubRip, Language: "fr-CA", Forced: true},
			{Container: model.MP4, Number: 4, CodecID: "c608", Format: model.SCC, Language: "eng"},
			{Container: model.MP4, Number: 5, CodecID: "enct", Language: "deu"},
		}},
		{"one_track.mp4", []model.Track{
			{Container: model.MP4, Number: 2, CodecID: "tx3g", Format: model.SubRip, Language: "eng", Default: true},
		}},
		{"c708_track.mov", []model.Track{
			{Container: model.MP4, Number: 1, CodecID: "c708", Format: model.MCC, Language: "fra", Default: true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, openFixture(t, tt.file).Tracks())
		})
	}
}

func TestExtractFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		file     string
		track    int
		language string
		want     []cueRow
	}{
		{"tx3g with gaps, a style box, UTF-16 and CR LF", "text_tracks.mp4", 2, "eng", []cueRow{
			{ms(1000), ms(3500), []string{"The tram to the old town leaves from stop 3."}, false},
			{ms(4000), ms(7000), []string{"Tickets are sold", "at the machine."}, false},
			{ms(7000), ms(9000), []string{"<b>Bold</b> words here."}, false},
			{ms(10000), ms(12250), []string{"Café at the corner, 2 € a cup."}, false},
			{ms(12250), ms(14000), []string{"Line one", "Line two"}, false},
		}},
		{"tx3g with version 1 boxes, co64, stz2 and the forced flag", "text_tracks.mp4", 3, "fr-CA", []cueRow{
			{ms(1000), ms(3000), []string{"Le tram part à huit heures."}, true},
			{ms(3000), ms(5500), []string{"La boulangerie ouvre à six heures."}, true},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := extract(t, openFixture(t, tt.file), tt.track, mp4.ExtractOptions{})
			assert.Equal(t, tt.want, rows(s))
			lang, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, tt.language, lang)
		})
	}
}

func TestExtractEqualsTheSourceFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mov     string
		source  string
		scc     scc.ReadOptions
		mcc     mcc.ReadOptions
		readMCC bool
		minCues int
	}{
		{name: "pop-on c608", mov: "ffmpeg_c608.mov", source: "scc/real/captionmaker_popon_df.scc", minCues: 1},
		{name: "roll-up c608 by screen", mov: "ffmpeg_c608_rollup.mov", source: "scc/real/rollup_news_ndf.scc", scc: scc.ReadOptions{RollUp: scc.RollUpScreen}, minCues: 1},
		{name: "roll-up c608 by line", mov: "ffmpeg_c608_rollup.mov", source: "scc/real/rollup_news_ndf.scc", scc: scc.ReadOptions{RollUp: scc.RollUpLines}, minCues: 1},
		{name: "c708 service 1", mov: "c708_track.mov", source: "mcc/real/captionmaker_608_708.mcc", readMCC: true, minCues: 1},
		{name: "c708 service 2", mov: "c708_track.mov", source: "mcc/real/captionmaker_608_708.mcc", mcc: mcc.ReadOptions{Service: 2}, readMCC: true, minCues: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := bytes.NewReader(testfiles.Read(t, tt.source))
			var want *model.Subtitle
			var err error
			if tt.readMCC {
				want, _, err = mcc.Read(content, model.CommonReadOptions{}, tt.mcc)
			} else {
				want, _, err = scc.Read(content, model.CommonReadOptions{}, tt.scc)
			}
			require.NoError(t, err)

			got := extract(t, openFixture(t, tt.mov), 1, mp4.ExtractOptions{SCC: tt.scc, MCC: tt.mcc})
			require.GreaterOrEqual(t, len(got.Cues()), tt.minCues)
			assert.Equal(t, want.Cues(), got.Cues())
		})
	}
}

func TestC608AfterItsEditList(t *testing.T) {
	t.Parallel()

	s := extract(t, openFixture(t, "text_tracks.mp4"), 4, mp4.ExtractOptions{})
	require.Len(t, s.Cues(), 1)
	cue := s.Cues()[0]
	assert.Equal(t, cueRow{ms(734), ms(2002), []string{"BUS 7"}, false}, rows(s)[0])
	data, ok := cue.FormatData("scc")
	require.True(t, ok)
	mode, _ := data.Get("mode")
	assert.Equal(t, "pop-on", mode)
}

func TestTrackNamesAndLanguages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     trackSpec
		wantName string
		wantLang string
	}{
		{"a plain name", trackSpec{name: "English"}, "English", "und"},
		{"a name box with version and flags", trackSpec{name: "\x00\x00\x00\x00English"}, "English", "und"},
		{"a QuickTime text list", trackSpec{name: string(cat(u16(7), u16(0x15C7), []byte("English")))}, "English", "und"},
		{"a name that stops at a NUL byte", trackSpec{name: "Eng\x00lish"}, "Eng", "und"},
		{"a name that is not UTF-8", trackSpec{name: "Caf\xE9"}, "", "und"},
		{"the language of mdhd", trackSpec{language: "fra"}, "", "fra"},
		{"the language of elng", trackSpec{language: "fra", elng: "fr-CA"}, "", "fr-CA"},
		{"an empty elng box", trackSpec{language: "deu", elng: ""}, "", "deu"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tracks := openBytes(t, single(tt.spec, textSample("Hi"))).Tracks()
			require.Len(t, tracks, 1)
			assert.Equal(t, tt.wantName, tracks[0].Name)
			assert.Equal(t, tt.wantLang, tracks[0].Language)
		})
	}
}
