package chapters_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestReadPodcast(t *testing.T) {
	t.Parallel()

	unsorted := `{"version": "1.2.0", "chapters": [{"startTime": 260, "title": "Progress <report>"}, ` +
		`{"startTime": 0, "title": "Intro", "endTime": 150.5}, {"startTime": 168}]}`
	tests := []struct {
		name    string
		content string
		media   time.Duration
		want    []span
	}{
		{
			"sorted, with the end of the file and the next start", unsorted, 0,
			[]span{{0, sec(150.5), "Intro"}, {sec(168), sec(260), ""}, {sec(260), sec(260), "Progress &lt;report&gt;"}},
		},
		{
			"media duration ends the last chapter", unsorted, 4980 * time.Second,
			[]span{{0, sec(150.5), "Intro"}, {sec(168), sec(260), ""}, {sec(260), sec(4980), "Progress &lt;report&gt;"}},
		},
		{
			"null end, rounded times, a title that is no string",
			`{"chapters": [{"startTime": 0.0004, "endTime": null, "title": 5}, {"startTime": 1.2345}]}`, 0,
			[]span{{0, sec(1.235), ""}, {sec(1.235), sec(1.235), ""}},
		},
		{"end before start stays", `{"chapters": [{"startTime": 10, "endTime": 5}]}`, 0, []span{{sec(10), sec(5), ""}}},
		{"empty chapters list", `{"chapters": []}`, 0, nil},
		{"blank content", " \n", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readSubtitle(t, chapters.ReadPodcast, tt.content, chapters.ReadOptions{MediaDuration: tt.media})
			assert.Equal(t, tt.want, spansOf(s))
		})
	}
}

func TestReadPodcastErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"not JSON", "{", "The content is not valid JSON: Syntax error."},
		{"root is a list", `[1]`, "The JSON root must be an object."},
		{"root is a string", `"x"`, "The JSON root must be an object."},
		{"empty list root", `[]`, `The JSON has no "chapters" list.`},
		{"no chapters", `{"version": "1.2.0"}`, `The JSON has no "chapters" list.`},
		{"chapters object", `{"chapters": {"a": 1}}`, `The JSON has no "chapters" list.`},
		{"start as a string", `{"chapters": [{"startTime": 0}, {"startTime": "1:00"}]}`, "The field chapters[1].startTime must be a number."},
		{"negative start", `{"chapters": [{"startTime": 0, "title": "A"}, {"startTime": -5, "title": "B"}]}`, "The field chapters[1].startTime must be a number."},
		{"chapter is no object", `{"chapters": [5]}`, "The field chapters[0].startTime must be a number."},
		{"infinite start", `{"chapters": [{"startTime": 1e400}]}`, "The field chapters[0].startTime must be a number."},
		{"infinite end", `{"chapters": [{"startTime": 0, "endTime": 1e400}]}`, "The field chapters[0].endTime must be a number."},
		{"end as a string", `{"chapters": [{"startTime": 0, "endTime": "x"}]}`, "The field chapters[0].endTime must be a number."},
		{"100000 hours", `{"chapters": [{"startTime": 0}, {"startTime": 360000000}]}`, "The field chapters[1].startTime is not below 100000 hours."},
		{"end at 1e20", `{"chapters": [{"startTime": 0, "endTime": 1e20}]}`, "The field chapters[0].endTime is not below 100000 hours."},
		{"checks come before the limit", `{"chapters": [{"startTime": 360000000, "endTime": "x"}]}`, "The field chapters[0].endTime must be a number."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, chapters.ReadPodcast, tt.content)
			assert.Equal(t, tt.want, perr.Msg)
			assert.Equal(t, model.PodcastChapters, perr.Format)
		})
	}
}

func TestReadPodcastKeepsTheOtherMembers(t *testing.T) {
	t.Parallel()

	content := `{"version": "1.2.0", "title": "Episode 7", "author": 5, "podcastName": "Harbour",
		"chapters": [{"startTime": 0, "toc": false, "location": {"name": "Paris", "lat": 52.0}}]}`
	s := readSubtitle(t, chapters.ReadPodcast, content, chapters.ReadOptions{})

	title, _ := s.Metadata(model.MetadataTitle)
	assert.Equal(t, "Episode 7", title)
	_, hasAuthor := s.Metadata(model.MetadataAuthor)
	assert.False(t, hasAuthor)
	file, _ := s.FormatData("podcast-chapters")
	assert.Equal(t, []string{"version", "podcastName"}, file.Keys())
	c, _ := s.Cue(0)
	cue, _ := c.FormatData("podcast-chapters")
	assert.Equal(t, []string{"toc", "location"}, cue.Keys())
	location, _ := cue.Object("location")
	lat, _ := location.Get("lat")
	assert.InDelta(t, 52.0, lat, 0)
}

func TestWritePodcast(t *testing.T) {
	t.Parallel()

	withData := func(c model.Cue, data *model.Data) model.Cue {
		require.NoError(t, c.SetFormatData("podcast-chapters", data))
		return c
	}
	tests := []struct {
		name string
		make func(t *testing.T) *model.Subtitle
		want string
	}{
		{"cues from another format", func(*testing.T) *model.Subtitle {
			s := chaptersOf(
				model.Cue{Start: 0, End: sec(168), Lines: []string{"Intro"}},
				model.Cue{Start: sec(168), End: sec(250), Lines: []string{"<i>Hearing</i> aids", "&amp; more"}},
				model.Cue{Start: sec(260.25), End: sec(300)},
			)
			s.SetMetadata(model.MetadataTitle, "Episode 7")
			s.SetMetadata(model.MetadataAuthor, "Jane Doe")
			return s
		}, `{
    "version": "1.2.0",
    "author": "Jane Doe",
    "title": "Episode 7",
    "chapters": [
        {
            "startTime": 0,
            "title": "Intro"
        },
        {
            "startTime": 168,
            "endTime": 250,
            "title": "Hearing aids & more"
        },
        {
            "startTime": 260.25,
            "endTime": 300
        }
    ]
}
`},
		{"stored members, numbers and a negative time", func(t *testing.T) *model.Subtitle {
			t.Helper()

			s := chaptersOf(
				withData(model.Cue{Start: -time.Second, End: sec(90), Lines: []string{"A"}},
					model.NewData(model.Field{Key: "location", Value: model.NewData(model.Field{Key: "lat", Value: 52.0})})),
				model.Cue{Start: sec(95.5), End: sec(95.5), Lines: []string{"B"}},
			)
			require.NoError(t, s.SetFormatData("podcast-chapters", model.NewData(
				model.Field{Key: "version", Value: nil}, model.Field{Key: "podcastName", Value: "Harbour"})))
			return s
		}, `{
    "version": "1.2.0",
    "podcastName": "Harbour",
    "chapters": [
        {
            "startTime": 0,
            "endTime": 90,
            "title": "A",
            "location": {
                "lat": 52.0
            }
        },
        {
            "startTime": 95.5,
            "title": "B"
        }
    ]
}
`},
		{"no cues", func(*testing.T) *model.Subtitle { return &model.Subtitle{} }, `{
    "version": "1.2.0",
    "chapters": []
}
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, writeString(t, chapters.WritePodcast, tt.make(t), model.CommonWriteOptions{}))
		})
	}
}

func TestWritePodcastRejectsBadText(t *testing.T) {
	t.Parallel()

	s := chaptersOf(model.Cue{Lines: []string{"bad \xff byte"}})
	err := chapters.WritePodcast(&strings.Builder{}, s, model.CommonWriteOptions{}, chapters.WriteOptions{})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	assert.ErrorIs(t, err, model.ErrInvalidUTF8)
}

func BenchmarkReadPodcast(b *testing.B) {
	chapter := `{"startTime": 1.5, "title": "Chapter", "img": "https://example.com/a.png"},`
	members := make([]string, 1<<15)
	for i := range members {
		members[i] = `"k` + strconv.Itoa(i) + `": 1`
	}
	for _, tc := range []struct{ name, content string }{
		{"chapters=1MB", `{"chapters": [` + strings.Repeat(chapter, (1<<20)/len(chapter)) + `{"startTime": 0}]}`},
		{"chapter members=1MB", `{"chapters": [{"startTime": 0, ` + strings.Join(members, ", ") + `}]}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := chapters.ReadPodcast(strings.NewReader(tc.content), model.CommonReadOptions{}, chapters.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
