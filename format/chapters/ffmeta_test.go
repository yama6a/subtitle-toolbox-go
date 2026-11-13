package chapters_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestReadFFMetadata(t *testing.T) {
	t.Parallel()

	inOrder := ";FFMETADATA1\ntitle=Meetup\n\n[CHAPTER]\nSTART=0\nEND=60000000000\ntitle=Doors open\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\n;a comment\nEND=90000\ntitle=No start\n" +
		"[CHAPTER]\nTIMEBASE=1/10\nSTART=1200\nno tag on this line\ntitle=No end\n" +
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=130000\nEND=125000\ntitle=Ends early\n"
	tests := []struct {
		name    string
		content string
		media   time.Duration
		want    []span
	}{
		{
			"times in the order of FFmpeg", inOrder, 200 * time.Second,
			[]span{{0, sec(60), "Doors open"}, {sec(60), sec(90), "No start"}, {sec(120), sec(130), "No end"}, {sec(130), sec(125), "Ends early"}},
		},
		{
			"media duration ends a last chapter without END", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1\nSTART=5\ntitle=A\n", 9 * time.Second,
			[]span{{5 * time.Second, 9 * time.Second, "A"}},
		},
		{"START after END is a tag", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1\nEND=7\nSTART=5\ntitle=A\n", 0, []span{{0, 7 * time.Second, "A"}}},
		{"time base after START is a tag", ";FFMETADATA1\n[CHAPTER]\nSTART=5\nTIMEBASE=1/1\ntitle=A\n", 0, []span{{0, 0, "A"}}},
		{"prefix match and rounding", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/3x\nSTART=1ab\nEND=2\ntitle=A\n", 0, []span{{sec(0.333), sec(0.667), "A"}}},
		{"negative start", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=-5\nEND=3\n", 0, []span{{-5 * time.Millisecond, 3 * time.Millisecond, ""}}},
		{"leading zeros of the time base", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=01/0002\nSTART=4\n", 0, []span{{2 * time.Second, 2 * time.Second, ""}}},
		{"title with a continued line", ";FFMETADATA1\n[CHAPTER]\ntitle=a\\\nb\\\\\n", 0, []span{{0, 0, "a\nb\\"}}},
		{"blank content", "\n \n", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readSubtitle(t, chapters.ReadFFMetadata, tt.content, chapters.ReadOptions{MediaDuration: tt.media})
			assert.Equal(t, tt.want, spansOf(s))
		})
	}
}

func TestReadFFMetadataKeepsTheTags(t *testing.T) {
	t.Parallel()

	content := ";FFMETADATA1\r\ntitle=Meetup\r\nartist=Jane\r\ngenre=Talk\\\r\nshow\r\ntitle=Spring\r\n# comment\r\nw\\=x=y\\;z\r\nlone\r\n" +
		"[STREAM]\r\nhandler=Sound\r\n[STREAM]\r\n[CHAPTER]\r\nTIMEBASE=1/1000\r\nSTART=0\r\nEND=5\r\nlang\\=x=en\r\ntitle=A\r\n"
	s := readSubtitle(t, chapters.ReadFFMetadata, content, chapters.ReadOptions{})

	title, _ := s.Metadata(model.MetadataTitle)
	assert.Equal(t, "Spring", title)
	artist, _ := s.Metadata(model.MetadataArtist)
	assert.Equal(t, "Jane", artist)
	file, _ := s.FormatData("ffmeta-chapters")
	tags, _ := file.Object("tags")
	assert.Equal(t, []string{"title", "artist", "genre", "w=x"}, tags.Keys())
	genre, _ := tags.String("genre")
	assert.Equal(t, "Talk\nshow", genre)
	value, _ := tags.String("w=x")
	assert.Equal(t, "y;z", value)
	streams, _ := file.List("streams")
	require.Len(t, streams, 2)
	assert.Equal(t, []any{}, streams[1])
	c, _ := s.Cue(0)
	cue, _ := c.FormatData("ffmeta-chapters")
	cueTags, _ := cue.Object("tags")
	assert.Equal(t, []string{"lang=x"}, cueTags.Keys())
	timeBase, _ := cue.String("timeBase")
	assert.Equal(t, "1/1000", timeBase)
}

func TestReadFFMetadataErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantMsg  string
		wantLine int
	}{
		{"no header", "[CHAPTER]\nSTART=0\n", "The content does not start with the ;FFMETADATA header.", 1},
		{"zero time base", ";FFMETADATA1\n\n[CHAPTER]\nTIMEBASE=1/0\nSTART=0\n", "The chapter time base 1/0 is not valid.", 4},
		{"time base beyond int64", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=99999999999999999999/1000\n", "The chapter time base 99999999999999999999/1000 is not valid.", 3},
		{"100000 hours", ";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=360000000000\n", `The time "START=360000000000" is not below 100000 hours.`, 4},
		{"end of 1e20 ns", ";FFMETADATA1\n[CHAPTER]\nSTART=0\nEND=100000000000000000000\n", `The time "END=100000000000000000000" is not below 100000 hours.`, 4},
		{"line after a continued line", ";FFMETADATA1\ntitle=a\\\nb\n[CHAPTER]\nTIMEBASE=0/1\n", "The chapter time base 0/1 is not valid.", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, chapters.ReadFFMetadata, tt.content)
			assert.Equal(t, tt.wantMsg, perr.Msg)
			assert.Equal(t, tt.wantLine, perr.Line)
		})
	}
}

func TestWriteFFMetadata(t *testing.T) {
	t.Parallel()

	t.Run("escapes, a stored time base and stored tags", func(t *testing.T) {
		t.Parallel()

		first := model.Cue{Start: 0, End: sec(61.5), Lines: []string{"a=b; c#d \\ e"}}
		require.NoError(t, first.SetFormatData("ffmeta-chapters", model.NewData(
			model.Field{Key: "timeBase", Value: "1/90000"},
			model.Field{Key: "tags", Value: model.NewData(model.Field{Key: "lang=x", Value: "en"}, model.Field{Key: "title", Value: "stale"})}),
		))
		s := chaptersOf(first, model.Cue{Start: sec(61.5), End: sec(70), Lines: []string{"first line", "second line"}})

		want := ";FFMETADATA1\n" +
			"[CHAPTER]\nTIMEBASE=1/90000\nSTART=0\nEND=5535000\ntitle=a\\=b\\; c\\#d \\\\ e\nlang\\=x=en\n" +
			"[CHAPTER]\nTIMEBASE=1/1000\nSTART=61500\nEND=70000\ntitle=first line\\\nsecond line\n"
		assert.Equal(t, want, writeString(t, chapters.WriteFFMetadata, s, model.CommonWriteOptions{}))
	})

	t.Run("the current metadata", func(t *testing.T) {
		t.Parallel()

		s := readSubtitle(t, chapters.ReadFFMetadata, ";FFMETADATA1\nmajor_brand=isom\ntitle=Old\nartist=Jane Doe\n", chapters.ReadOptions{})
		s.SetMetadata(model.MetadataTitle, "New")
		s.DeleteMetadata(model.MetadataArtist)
		s.SetMetadata(model.MetadataAlbum, "Notes")

		assert.Equal(t, ";FFMETADATA1\nmajor_brand=isom\ntitle=New\nalbum=Notes\n", writeString(t, chapters.WriteFFMetadata, s, model.CommonWriteOptions{}))
	})

	t.Run("streams and stored tags that are no strings", func(t *testing.T) {
		t.Parallel()

		s := chaptersOf(model.Cue{Start: time.Second, End: 2 * time.Second})
		require.NoError(t, s.SetFormatData("ffmeta-chapters", model.NewData(
			model.Field{Key: "tags", Value: []any{nil, "x"}},
			model.Field{Key: "streams", Value: []any{model.NewData(model.Field{Key: "a", Value: "b"})}},
		)))

		assert.Equal(t, ";FFMETADATA1\n[STREAM]\na=b\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=1000\nEND=2000\n",
			writeString(t, chapters.WriteFFMetadata, s, model.CommonWriteOptions{}))
	})

	t.Run("a stored time base beyond int64 fails", func(t *testing.T) {
		t.Parallel()

		c := model.Cue{Start: time.Second, End: 2 * time.Second}
		require.NoError(t, c.SetFormatData("ffmeta-chapters", model.NewData(model.Field{Key: "timeBase", Value: "1/99999999999999999999"})))
		err := chapters.WriteFFMetadata(&strings.Builder{}, chaptersOf(c), model.CommonWriteOptions{}, chapters.WriteOptions{})

		var uerr *model.UnwritableError
		require.ErrorAs(t, err, &uerr)
		assert.Equal(t, 0, uerr.CueIndex)
		assert.Equal(t, "The stored time base 1/99999999999999999999 has a number beyond 9223372036854775807.", uerr.Reason)
	})
}

func TestReadFFMetadataLargeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, s *model.Subtitle)
	}{
		{
			"100000 lines with the same key", ";FFMETADATA1\n" + strings.Repeat("k=v\n", 100_000) + strings.Repeat("a\\\n", 100_000),
			func(t *testing.T, s *model.Subtitle) {
				t.Helper()

				file, _ := s.FormatData("ffmeta-chapters")
				tags, _ := file.Object("tags")
				assert.Equal(t, 1, tags.Len())
			},
		},
		{"100000 different keys", ";FFMETADATA1\n" + func() string {
			var b strings.Builder
			for i := range 100_000 {
				b.WriteString("k" + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676%26)) + string(rune('a'+i/17576%26)) + "=v\n")
			}
			return b.String()
		}(), func(t *testing.T, s *model.Subtitle) {
			t.Helper()

			file, _ := s.FormatData("ffmeta-chapters")
			tags, _ := file.Object("tags")
			assert.Equal(t, 100_000, tags.Len())
		}},
		{"20000 chapters", ";FFMETADATA1\n" + strings.Repeat("[CHAPTER]\nSTART=1\n", 20_000), func(t *testing.T, s *model.Subtitle) {
			t.Helper()

			assert.Equal(t, 20_000, s.Len())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.check(t, readSubtitle(t, chapters.ReadFFMetadata, tt.content, chapters.ReadOptions{}))
		})
	}
}

func BenchmarkReadFFMetadata(b *testing.B) {
	chapter := "[CHAPTER]\nTIMEBASE=1/1000\nSTART=1500\nEND=2500\ntitle=Chapter\nlang=en\n"
	for _, tc := range []struct{ name, content string }{
		{"chapters=1MB", ";FFMETADATA1\n" + strings.Repeat(chapter, (1<<20)/len(chapter))},
		{"continued lines=1MB", ";FFMETADATA1\n" + strings.Repeat("a\\\n", 1<<19)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := chapters.ReadFFMetadata(strings.NewReader(tc.content), model.CommonReadOptions{}, chapters.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
