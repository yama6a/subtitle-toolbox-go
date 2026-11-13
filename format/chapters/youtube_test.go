package chapters_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

type chapter struct {
	start time.Duration
	title string
}

func read(tb testing.TB, content string, opts chapters.ReadOptions) []chapter {
	tb.Helper()

	s, warnings, err := chapters.ReadYouTube(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(tb, err)
	assert.Empty(tb, warnings)
	var out []chapter
	for _, c := range s.All() {
		out = append(out, chapter{c.Start, c.Text()})
	}
	return out
}

func TestReadYouTubeLines(t *testing.T) {
	t.Parallel()

	const emDash = "—"
	tests := []struct {
		name string
		line string
		want []chapter
	}{
		{"no time", "Recorded live.", nil},
		{"leading", "0:00 Intro", []chapter{{0, "Intro"}}},
		{"parentheses", "(2:48) Hearing aids", []chapter{{168 * time.Second, "Hearing aids"}}},
		{"brackets and bar", "[4:20] | Progress report", []chapter{{260 * time.Second, "Progress report"}}},
		{"em dash before the time", "Namespace " + emDash + " 6:50", []chapter{{410 * time.Second, "Namespace"}}},
		{"em dash glued", "Namespace" + emDash + "6:50", []chapter{{410 * time.Second, "Namespace"}}},
		{"trailing with colon", "The big players: 1:31:50", []chapter{{5510 * time.Second, "The big players"}}},
		{"letter after time", "12:30pm meeting", nil},
		{"seconds above 59", "0:75 Bad seconds", nil},
		{"minutes above 59 with hours", "1:60:00 Bad minutes", nil},
		{"minutes above 59 without hours", "75:00 Long video", []chapter{{4500 * time.Second, "Long video"}}},
		{"short minutes", "1:2:03 Short minutes", []chapter{{3723 * time.Second, "Short minutes"}}},
		{"time only", "2:00:00", []chapter{{7200 * time.Second, ""}}},
		{"hours overflow int64", "99999999999999999999:00:00 Too late", nil},
		{"100000 hours", "6000000:00 Too late", nil},
		{"url", "See https://example.com/a:b", nil},
		{"backtrack to minutes and seconds", "12:34:5x", []chapter{{754 * time.Second, "5x"}}},
		{"closing bracket glued to text", "(2:48)x", nil},
		{"trailing glued to word", "Chapter:2:48", []chapter{{168 * time.Second, "Chapter"}}},
		{"trailing with brackets", "Intro (1:02:48)", []chapter{{3768 * time.Second, "Intro"}}},
		{"trailing after digits", "Part 2 1:30", []chapter{{90 * time.Second, "Part 2"}}},
		{"time inside a word", "abc1:30", nil},
		{"title is escaped", "0:10 Tom & <b>Jerry</b>", []chapter{{10 * time.Second, "Tom &amp; &lt;b&gt;Jerry&lt;/b&gt;"}}},
		{"three digit seconds", "1:234 x", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, read(t, tt.line, chapters.ReadOptions{}))
		})
	}
}

func TestReadYouTubeEnds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		media time.Duration
		want  []time.Duration
	}{
		{"unknown media", 0, []time.Duration{168 * time.Second, 7200 * time.Second, 7200 * time.Second}},
		{"media after the last start", 8000 * time.Second, []time.Duration{168 * time.Second, 7200 * time.Second, 8000 * time.Second}},
		{"media before the last start", 100 * time.Second, []time.Duration{168 * time.Second, 7200 * time.Second, 7200 * time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := chapters.ReadYouTube(strings.NewReader("2:00:00\n0:00 Intro\n2:48 Next"), model.CommonReadOptions{},
				chapters.ReadOptions{MediaDuration: tt.media})
			require.NoError(t, err)
			var ends []time.Duration
			for _, c := range s.All() {
				ends = append(ends, c.End)
			}
			assert.Equal(t, tt.want, ends)
		})
	}
}

func TestReadYouTubeLongLine(t *testing.T) {
	t.Parallel()

	assert.Empty(t, read(t, strings.Repeat("1:2:", 5000)+"x", chapters.ReadOptions{}))
	assert.Len(t, read(t, strings.Repeat("1:2:", 5000)+"1:30", chapters.ReadOptions{}), 1)
}

func TestReadYouTubeOptions(t *testing.T) {
	t.Parallel()

	require.NoError(t, chapters.ReadOptions{}.Validate())
	err := chapters.ReadOptions{MediaDuration: -time.Second}.Validate()
	var ia *model.InvalidArgumentError
	require.ErrorAs(t, err, &ia)
	assert.Equal(t, "ReadOptions.Chapters.MediaDuration", ia.Option)
	assert.Equal(t, "The media duration must be 0 or more seconds, got -1.", ia.Msg)
}

func TestWriteYouTube(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: 900 * time.Millisecond, Lines: []string{"Intro"}},
		model.Cue{Start: 3599990 * time.Millisecond, Lines: []string{"<i>Last</i> minute"}},
		model.Cue{Start: time.Hour},
		model.Cue{Start: 10 * time.Hour, Lines: []string{"Ten", "&amp; hours"}},
	)
	var buf bytes.Buffer
	require.NoError(t, chapters.WriteYouTube(&buf, s, model.CommonWriteOptions{}, chapters.WriteOptions{}))
	assert.Equal(t, "0:00 Intro\n59:59 Last minute\n1:00:00\n10:00:00 Ten & hours\n", buf.String())
}

func TestYouTubeRealFiles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"video_description.txt", "long_stream_chapters.txt"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			first, _, err := chapters.ReadYouTube(bytes.NewReader(testfiles.Read(t, "chapters/youtube/real/"+name)), model.CommonReadOptions{}, chapters.ReadOptions{})
			require.NoError(t, err)
			var buf bytes.Buffer
			require.NoError(t, chapters.WriteYouTube(&buf, first, model.CommonWriteOptions{}, chapters.WriteOptions{}))
			second, _, err := chapters.ReadYouTube(&buf, model.CommonReadOptions{}, chapters.ReadOptions{})
			require.NoError(t, err)
			require.Equal(t, first.Len(), second.Len())
			for i, c := range first.All() {
				d, _ := second.Cue(i)
				assert.Equal(t, c.Start.Truncate(time.Second), d.Start)
				assert.Equal(t, c.Lines, d.Lines)
			}
		})
	}
}

func BenchmarkReadYouTube(b *testing.B) {
	for _, tc := range []struct{ name, line string }{
		{"bytes=1MB", strings.Repeat("1:2:", 1<<18) + "x"},
		{"lines=1MB", strings.Repeat("0:01 Chapter\n", (1<<20)/13)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := chapters.ReadYouTube(strings.NewReader(tc.line), model.CommonReadOptions{}, chapters.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
