package mkv_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestTracks(t *testing.T) {
	t.Parallel()

	want := []model.Track{
		{Container: model.Matroska, Number: 3, CodecID: "S_TEXT/UTF8", Format: model.SubRip, Language: "de", Name: "Deutsch (Forced)", Forced: true},
		{Container: model.Matroska, Number: 4, CodecID: "S_TEXT/ASS", Format: model.ASS, Language: "eng", Name: "English", Default: true},
		{Container: model.Matroska, Number: 5, CodecID: "S_TEXT/WEBVTT", Format: model.WebVTT, Language: "fre", Name: "Français"},
		{Container: model.Matroska, Number: 6, CodecID: "S_TEXT/SSA", Format: model.ASS, Language: "spa"},
		{Container: model.Matroska, Number: 7, CodecID: "S_DVBSUB", Format: model.PGS, Language: "ita"},
		{Container: model.Matroska, Number: 8, CodecID: "S_TEXT/UTF8", Format: model.SubRip, Language: "eng"},
	}
	assert.Equal(t, want, openTestdata(t, "text_tracks.mkv").Tracks())
}

func TestExtractTextTracks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		track  int
		format model.Format
		forced bool
		lang   string
		want   string
	}{
		{
			"SubRip with language and forced flag", 3, model.SubRip, true, "de",
			"1\n00:00:01,000 --> 00:00:03,500\nDer Zug nach Hamburg fährt um acht Uhr ab.\n\n" +
				"2\n00:00:04,000 --> 00:00:06,000\n<i>Gleis 4</i>\nBitte nicht einsteigen.\n\n" +
				"3\n00:00:09,800 --> 00:00:12,000\nDie Bäckerei öffnet um sechs.\n\n" +
				"4\n00:00:15,000 --> 00:00:17,250\nMorgen wird es sonnig\nund warm.\n",
		},
		{
			"ASS in ReadOrder with the header of CodecPrivate", 4, model.ASS, false, "eng",
			"[Script Info]\n; Written for the subtitle-toolbox tests\nTitle: Station\nScriptType: v4.00+\nPlayResX: 1920\nPlayResY: 1080\nWrapStyle: 0\n\n" +
				"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
				"Style: Default,Arial,48,&H00FFFFFF,&H000000FF,&H00000000,&H64000000,0,0,0,0,100,100,0,0,1,2,1,2,60,60,40,1\n" +
				"Style: Sign,Arial,40,&H0000FFFF,&H000000FF,&H00000000,&H64000000,-1,0,0,0,100,100,0,0,1,2,0,8,60,60,40,1\n\n" +
				"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
				"Dialogue: 1,0:00:01.00,0:00:06.00,Sign,,0,0,0,,{\\an8}PLATFORM 4\n" +
				"Dialogue: 0,0:00:01.00,0:00:04.00,Default,Guard,0,0,0,,The train to the coast leaves at eight.\n" +
				"Dialogue: 0,0:00:07.00,0:00:09.00,Default,,0,0,0,,The bakery on the corner\\Nis open {\\i1}every{\\i0} day.\n" +
				"Dialogue: 0,0:00:11.00,0:00:13.50,Default,Guard,0,0,0,,Bring an umbrella, it may rain later.\n",
		},
		{
			"SSA with the Events section added", 6, model.ASS, false, "spa",
			"[Script Info]\nTitle: Estacion\nScriptType: v4.00\n\n[V4 Styles]\n" +
				"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding\n" +
				"Style: Default,Arial,20,16777215,65535,65535,0,0,0,1,2,0,2,30,30,30,0,0\n\n" +
				"[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
				"Dialogue: Marked=0,0:00:03.00,0:00:05.00,Default,,0000,0000,0000,,El tren sale a las ocho.\n" +
				"Dialogue: Marked=0,0:00:13.00,0:00:15.50,Default,,0000,0000,0000,,La panadería abre a las seis.\n",
		},
		{
			"WebVTT with settings, identifiers, comments and relative timestamps", 5, model.WebVTT, false, "fre",
			"WEBVTT - Gare\n\nSTYLE\n::cue {\n  color: yellow;\n}\n\nNOTE Fichier écrit pour les tests\n\n" +
				"annonce-1\n00:00:02.000 --> 00:00:05.000 line:10% align:start\nLe train pour Lyon part à huit heures.\n\n" +
				"NOTE Deuxième annonce\n\n" +
				"2\n00:00:06.000 --> 00:00:08.500\nLa boulangerie ouvre à six heures.\nLe pain est <00:00:07.500>encore chaud.\n\n" +
				"3\n00:00:12.000 --> 00:00:14.000\nDemain, il fera beau.\n",
		},
		{
			"blocks without duration end at the next block, the last after 5 s", 8, model.SubRip, false, "eng",
			"1\n00:00:02,500 --> 00:00:08,000\nNext stop: Central Station.\n\n" +
				"2\n00:00:08,000 --> 00:00:16,000\nDoors open on the left.\n\n" +
				"3\n00:00:16,000 --> 00:00:21,000\nEnd of the line.\n",
		},
	}
	f := openTestdata(t, "text_tracks.mkv")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := f.Extract(tt.track, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, tt.want, write(t, s, tt.format))
			for _, c := range s.Cues() {
				assert.Equal(t, tt.forced, c.Forced)
			}
			lang, ok := s.Metadata(model.MetadataLanguage)
			assert.True(t, ok)
			assert.Equal(t, tt.lang, lang)
		})
	}
}

type cueRow struct {
	start, end time.Duration
	text       string
}

func cueRows(s *model.Subtitle) []cueRow {
	rows := make([]cueRow, 0, s.Len())
	for _, c := range s.Cues() {
		rows = append(rows, cueRow{c.Start, c.End, c.Text()})
	}
	return rows
}

func TestExtractFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		track int
		want  []cueRow
	}{
		{"zlib", "compressed.mkv", 3, []cueRow{
			{1 * time.Second, 3 * time.Second, "The ferry leaves at noon."},
			{4 * time.Second, 6 * time.Second, "Tickets are sold\nat the harbour."},
		}},
		{"zlib with CodecPrivate", "compressed.mkv", 4, []cueRow{{2 * time.Second, 5 * time.Second, "The museum is closed on Mondays."}}},
		{"header stripping", "compressed.mkv", 5, []cueRow{
			{1500 * time.Millisecond, 3500 * time.Millisecond, "Weather: sunny."},
			{5 * time.Second, 7500 * time.Millisecond, "Weather: cloudy, with light rain."},
		}},
		{"zlib, then header stripping", "compressed.mkv", 6, []cueRow{{3 * time.Second, 4500 * time.Millisecond, "Bakery: fresh bread at seven."}}},
		{"unknown sizes and a TimestampScale of 100000", "unknown_sizes.mkv", 2, []cueRow{
			{1235 * time.Millisecond, 3 * time.Second, "Tåget till Malmö är försenat."},
			{4 * time.Second, 5500 * time.Millisecond, "Bageriet stänger klockan fem."},
			{7 * time.Second, 9 * time.Second, "I morgon blir det soligt."},
		}},
		{"Tracks after the clusters through the SeekHead", "seek_head.mkv", 2, []cueRow{
			{1 * time.Second, 3 * time.Second, "The library opens at nine."},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := openTestdata(t, tt.file).Extract(tt.track, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cueRows(s))
		})
	}
}

func TestLastCueDuration(t *testing.T) {
	t.Parallel()

	file := cat(ebmlHeader("matroska"), unknownSizeElement(idSegment, cat(info(1000000, "x"), tracks(subTrack(1, "S_TEXT/UTF8")),
		cluster(0, blockGroup(1, 1000, "Platform 4.", 1000), simpleBlock(1, 3000, "Mind the gap.", 0)))))
	d := 1005 * time.Millisecond
	tests := []struct {
		name string
		opts model.CommonReadOptions
		end  time.Duration
	}{
		{"default of 5 s", model.CommonReadOptions{}, 8 * time.Second},
		{"rounded to milliseconds", model.CommonReadOptions{LastCueDuration: &d}, 4005 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := openFile(t, file).Extract(1, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, []cueRow{{time.Second, 2 * time.Second, "Platform 4."}, {3 * time.Second, tt.end, "Mind the gap."}}, cueRows(s))
		})
	}
}

func TestBuilderRules(t *testing.T) {
	t.Parallel()

	assPrivate := "\xEF\xBB\xBF[Script Info]\r\nTitle: x\r\n\r\n[events]  \r\nFormat: Text, End, Style, Start, Marked\r\n\r\n"
	tests := []struct {
		name  string
		track []byte
		block []byte
		want  cueRow
	}{
		{
			"SubRip text with a blank line inside the cue", subTrack(1, "S_TEXT/UTF8"), simpleBlock(1, 0, "\r\nOne\r\n \t\r\n\r\nTwo\n", 0),
			cueRow{0, 5 * time.Second, "One\nTwo"},
		},
		{
			"ASS fields in the order of the Format line, CRLF and BOM in CodecPrivate",
			trackEntry(trackFields{number: 1, kind: trackSub, codecID: "S_TEXT/ASS", codecPrivate: []byte(assPrivate)}),
			simpleBlock(1, 0, "7,0,Sign,,0,0,0,,line one\nline two", 0),
			cueRow{0, 5 * time.Second, "line one\nline two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := openFile(t, segmentFile("matroska", tracks(tt.track), cluster(0, tt.block)))
			s, warnings, err := f.Extract(1, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			assert.Equal(t, []cueRow{tt.want}, cueRows(s))
		})
	}
}

func TestTimeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		codec   string
		scale   uint64
		cluster []byte
		time    string
	}{
		{"cluster timestamp", "S_TEXT/UTF8", 1000000, cluster(1<<40, simpleBlock(2, 0, "x", 0)), "1099511627776"},
		{"block duration", "S_TEXT/UTF8", 1000000, cluster(0, blockGroup(2, 0, "x", 1<<40)), "0"},
		{"PGS block", "S_HDMV/PGS", 1000000, cluster(1<<40, simpleBlock(2, 0, "x", 0)), "1099511627776"},
		{"start of -2^63 ns", "S_TEXT/UTF8", 1 << 48, cluster(0, simpleBlock(2, -32768, "x", 0)), "-32768"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := openFile(t, segmentFile("matroska", info(tt.scale, "x"), tracks(subTrack(2, tt.codec)), tt.cluster))
			_, _, err := f.Extract(2, model.CommonReadOptions{})
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			assert.Empty(t, pe.Format)
			assert.Regexp(t, fmt.Sprintf(`^The block of track 2 at byte \d+ has the time %s at a TimestampScale of %d ns\. Times must be below 100000 hours\.$`, tt.time, tt.scale), pe.Msg)
		})
	}
}

func hugeCodecPrivate() []byte {
	declared := uint64(1) << 40
	entry := cat(uintElement(0xD7, 2), uintElement(0x83, trackSub), element(0x86, []byte("S_TEXT/UTF8")),
		idBytes(0x63A2), sizeBytes(declared), []byte("abc"))
	entry = cat(idBytes(0xAE), sizeBytes(declared+uint64(len(entry))), entry)
	return cat(idBytes(idTracks), sizeBytes(declared+uint64(len(entry))), entry)
}

func TestBrokenFiles(t *testing.T) {
	t.Parallel()

	// brokenFile builds a file with a Tracks element, or the S_TEXT/UTF8 track 2 with the given fields, followed by clusters.
	brokenFile := func(clusters, trackElement []byte, fields trackFields, cut int) []byte {
		if trackElement == nil {
			fields.number, fields.kind, fields.codecID = 2, trackSub, "S_TEXT/UTF8"
			trackElement = tracks(trackEntry(fields))
		}
		file := segmentFile("matroska", trackElement, clusters)
		return file[:len(file)-cut]
	}
	encrypted := element(0x6240, cat(uintElement(0x5031, 0), uintElement(0x5032, 1), uintElement(0x5033, 1), element(0x5035, nil)))
	tests := []struct {
		name string
		file []byte
		want string
	}{
		{
			"cluster timestamp of more than 8 bytes", testfiles.Read(t, "mkv/huge_timestamp.mkv"),
			"The unsigned integer of the element 0xE7 at byte 1562 is longer than 8 bytes or not below 2^63.",
		},
		{
			"laced block", brokenFile(cluster(0, simpleBlock(2, 0, "\x01\x02ab", lacingXiph)), nil, trackFields{}, 0),
			"A block of track 2 uses lacing, which subtitle tracks do not use.",
		},
		{
			"block header too short", brokenFile(cluster(0, element(0xA3, []byte("\x82\x00"))), nil, trackFields{}, 0),
			"The block header at byte 86 is not valid.",
		},
		{
			"child larger than its parent", brokenFile(nil, tracks(element(0xAE, cat(uintElement(0xD7, 2), []byte("\x86\x85ab")))), trackFields{}, 0),
			"The element at byte 55 does not fit into its parent element.",
		},
		{
			"unknown size outside Segment and Cluster", brokenFile(nil, unknownSizeElement(idTracks, nil), trackFields{}, 0),
			"The element 0x1654AE6B at byte 57 has an unknown size, which only Segment and Cluster may have.",
		},
		{
			"no Tracks element", brokenFile(nil, info(1000000, "x"), trackFields{}, 0),
			"The file has no Tracks element.",
		},
		{
			"zlib data that does not inflate",
			brokenFile(cluster(0, blockGroup(2, 0, "not zlib", 1000)), nil, trackFields{encodings: compression(0)}, 0),
			"The zlib data of track 2 is not valid or inflates to more than 64 MB.",
		},
		{
			"encrypted track", brokenFile(cluster(0, blockGroup(2, 0, "secret", 1000)), nil, trackFields{encodings: encrypted}, 0),
			"Track 2 uses encryption or a compression other than zlib and header stripping.",
		},
		{
			"bzlib compression", brokenFile(cluster(0, blockGroup(2, 0, "x", 1000)), nil, trackFields{encodings: compression(1)}, 0),
			"Track 2 uses encryption or a compression other than zlib and header stripping.",
		},
		{
			"invalid element header in a cluster", brokenFile(cat(cluster(0), []byte{0, 0, 0, 0}), nil, trackFields{}, 0),
			"The EBML element header at byte 84 is not valid.",
		},
		{
			"block data cut by the file end",
			brokenFile(cluster(0, blockGroup(2, 0, "The ferry leaves at noon.", 1000)), nil, trackFields{}, 10),
			"The element data at byte 92 needs 25 bytes, but the file ends before.",
		},
		{
			"CodecPrivate of 2^40 bytes", brokenFile(nil, hugeCodecPrivate(), trackFields{}, 0),
			"The element data at byte 89 needs 1099511627776 bytes, but the file ends before.",
		},
		{
			"zlib block that inflates past 64 MB",
			brokenFile(cluster(0, blockGroup(2, 0, string(zlibFilled(t, 0, 64<<20+1)), 1000)), nil, trackFields{encodings: compression(0)}, 0),
			"The zlib data of track 2 is not valid or inflates to more than 64 MB.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f, err := mkv.Open(bytes.NewReader(tt.file), int64(len(tt.file)))
			if err == nil {
				for _, track := range f.Tracks() {
					if _, _, err = f.Extract(track.Number, model.CommonReadOptions{}); err != nil {
						break
					}
				}
			}
			var pe *model.ParseError
			require.ErrorAs(t, err, &pe)
			assert.Empty(t, pe.Format)
			assert.Equal(t, tt.want, pe.Msg)
		})
	}
}
