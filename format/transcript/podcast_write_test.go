package transcript_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func writePodcast(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts transcript.PodcastWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, transcript.WritePodcast(&out, s, common, transcript.WriteOptions{Podcast: opts}))
	return out.String()
}

// dialogue holds the 3 cues of testWritesOneSegmentPerCueAndSpeaker. The words end at the next word start.
func dialogue() *model.Subtitle {
	s := &model.Subtitle{}
	w := func(text string, start, end int) model.Word {
		return model.Word{Text: text, Start: time.Duration(start) * time.Millisecond, End: time.Duration(end) * time.Millisecond}
	}
	s.AddCues(
		model.Cue{
			Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"<v Anna><i>Where</i> are", "you?"},
			Words: []model.Word{w("Where", 1000, 1500), w("are", 1500, 2000), w("you?", 2000, 3000)},
		},
		model.Cue{
			Start: 3 * time.Second, End: 5 * time.Second, Lines: []string{"<v Ben>Home &amp; bed.", "<v Anna>Why?"},
			Words: []model.Word{w("bed.", 4000, 4000)},
		},
		model.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"No &lt;speaker&gt; here"}, Words: []model.Word{w("here", 9000, 9000)}},
	)
	return s
}

func TestWritePodcastSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts transcript.PodcastWriteOptions
		want string
	}{
		{
			"one segment per cue and speaker",
			transcript.PodcastWriteOptions{},
			`{"version":"1.0.0","segments":[{"speaker":"Anna","startTime":1.0,"endTime":3.0,"body":"Where are you?"},` +
				`{"speaker":"Ben","startTime":3.0,"endTime":5.0,"body":"Home & bed."},{"speaker":"Anna","startTime":3.0,"endTime":5.0,"body":"Why?"},` +
				`{"startTime":6.0,"endTime":7.0,"body":"No <speaker> here"}]}`,
		},
		{
			"one segment per word",
			transcript.PodcastWriteOptions{WordSegments: true},
			`{"version":"1.0.0","segments":[{"speaker":"Anna","startTime":1.0,"endTime":1.5,"body":"Where"},` +
				`{"speaker":"Anna","startTime":1.5,"endTime":2.0,"body":"are"},{"speaker":"Anna","startTime":2.0,"endTime":3.0,"body":"you?"},` +
				`{"speaker":"Ben","startTime":3.0,"endTime":4.0,"body":"Home &"},{"speaker":"Ben","startTime":4.0,"endTime":4.0,"body":"bed."},` +
				`{"speaker":"Anna","startTime":4.0,"endTime":5.0,"body":"Why?"},{"startTime":6.0,"endTime":7.0,"body":"No <speaker>"},` +
				`{"startTime":7.0,"endTime":7.0,"body":"here"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, writePodcast(t, dialogue(), model.CommonWriteOptions{}, tt.opts))
		})
	}
}

func TestWritePodcastWordEndsAndOrder(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{
		Start: 0, End: 3 * time.Second, Lines: []string{"a b"},
		Words: []model.Word{{Text: "a", Start: 0, End: time.Second}, {Text: "b", Start: 2 * time.Second, End: 2500 * time.Millisecond}},
	})
	s.AddCue(model.Cue{Start: 4 * time.Second, End: 2 * time.Second, Lines: []string{"back"}})
	got := writePodcast(t, s, model.CommonWriteOptions{}, transcript.PodcastWriteOptions{WordSegments: true})
	assert.JSONEq(t, `{"version":"1.0.0","segments":[{"startTime":0.0,"endTime":1.0,"body":"a"},`+
		`{"startTime":2.0,"endTime":2.5,"body":"b"},{"startTime":4.0,"endTime":4.0,"body":"back"}]}`, got,
		"a word ends at its own end when that is earlier, and a cue that ends before it starts writes a segment that reads back")
}

func TestWritePodcastFileDataAndPretty(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	fileData := model.NewData()
	fileData.Set("version", "1.0.1")
	fileData.Set("language", "en")
	require.NoError(t, s.SetFormatData("podcast-transcript", fileData))
	c := model.Cue{Start: 0, End: 1250 * time.Millisecond, Lines: []string{"Hi"}}
	cueData := model.NewData()
	cueData.Set("confidence", 0.9)
	require.NoError(t, c.SetFormatData("podcast-transcript", cueData))
	s.AddCue(c)

	want := "{\r\n    \"version\": \"1.0.1\",\r\n    \"segments\": [\r\n        {\r\n            \"startTime\": 0.0,\r\n" +
		"            \"endTime\": 1.25,\r\n            \"body\": \"Hi\",\r\n            \"confidence\": 0.9\r\n        }\r\n" +
		"    ],\r\n    \"language\": \"en\"\r\n}\r\n"
	got := writePodcast(t, s, model.CommonWriteOptions{LineEnding: model.CRLF}, transcript.PodcastWriteOptions{PrettyPrint: true})
	assert.Equal(t, want, got)

	words := writePodcast(t, s, model.CommonWriteOptions{}, transcript.PodcastWriteOptions{WordSegments: true})
	assert.NotContains(t, words, "confidence", "word segments carry no cue data")
}

// Ports testWritesWordSegmentsFromWhisperJson. The Whisper words end before the next word starts, so the segments
// keep their ends. PHP ends every word at the next word start.
func TestWritePodcastWordSegmentsOfWhisperJSON(t *testing.T) {
	t.Parallel()

	s := readFile(t, "openai_whisper_word_timestamps.json", withWords)
	out := parseJSON(t, writePodcast(t, s, model.CommonWriteOptions{}, transcript.PodcastWriteOptions{WordSegments: true}))
	assert.JSONEq(t, `{"startTime":0.0,"endTime":0.24,"body":"The"}`, compact(t, out, "segments", 0))
}

// Ports testRealFileRoundTripsThroughTheFormatter and testWordSegmentsRoundTripWithTheirStartTimes, and adds the ends.
func TestWritePodcastRoundTrips(t *testing.T) {
	t.Parallel()

	for _, file := range podcastFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s := readPodcastFile(t, file, transcript.ReadOptions{})
			again := readPodcast(t, writePodcast(t, s, model.CommonWriteOptions{}, transcript.PodcastWriteOptions{PrettyPrint: true}), transcript.ReadOptions{})
			assert.Equal(t, describe(s), describe(again))
			a, _ := s.FormatData("podcast-transcript")
			b, _ := again.FormatData("podcast-transcript")
			assert.Equal(t, jsonOf(a), jsonOf(b))
		})
	}

	t.Run("word segments keep speaker, start, end and body", func(t *testing.T) {
		t.Parallel()

		source := parseJSON(t, string(testfiles.Read(t, "podcast/real/spec_word_segments.json")))
		s := readPodcastFile(t, "spec_word_segments.json", transcript.ReadOptions{WordTimestamps: true})
		out := parseJSON(t, writePodcast(t, s, model.CommonWriteOptions{}, transcript.PodcastWriteOptions{WordSegments: true}))
		want, _ := source.Get("segments")
		got, _ := out.Get("segments")
		require.Len(t, got.Items(), len(want.Items()))
		for i, w := range want.Items() {
			g := got.Items()[i]
			for _, key := range []string{"speaker", "startTime", "endTime"} {
				wv, _ := w.Get(key)
				gv, _ := g.Get(key)
				wf, _ := wv.AsFloat64()
				gf, _ := gv.AsFloat64()
				assert.InDelta(t, wf, gf, 0, "segment %d %s", i, key)
			}
			wb, _ := w.Get("body")
			gb, _ := g.Get("body")
			assert.Equal(t, trimmed(wb), trimmed(gb))
			ws, _ := w.Get("speaker")
			gs, _ := g.Get("speaker")
			assert.Equal(t, orderedjson.Block(ws), orderedjson.Block(gs))
		}
	})
}

func trimmed(v orderedjson.Value) string {
	s, _ := v.AsString()
	return string(bytes.TrimSpace([]byte(s)))
}

func TestWritePodcastUnwritable(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 0, End: time.Second, Lines: []string{"bad \xff byte"}})
	var out bytes.Buffer
	err := transcript.WritePodcast(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{})
	require.ErrorIs(t, err, model.ErrUnwritable)
	assert.ErrorIs(t, err, model.ErrInvalidUTF8)
}
