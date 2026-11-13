package transcript_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func readPodcast(t *testing.T, content string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _, err := transcript.ReadPodcast(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s
}

func readPodcastFile(t *testing.T, name string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	return readPodcast(t, string(testfiles.Read(t, "podcast/real/"+name)), opts)
}

const dialogueWords = `{"version": "1.0.0", "segments": [
	{"speaker": "Anna", "startTime": 0, "endTime": 0.4, "body": "Hello"}, {"speaker": "Anna", "startTime": 0.5, "endTime": 0.9, "body": "there."},
	{"speaker": "Anna", "startTime": 1, "endTime": 1.4, "body": "Come"}, {"speaker": "Anna", "startTime": 1.5, "endTime": 1.9, "body": "in!"},
	{"speaker": "Ben", "startTime": 2, "endTime": 2.4, "body": "Thanks"}, {"speaker": "Anna", "startTime": 2.5, "endTime": 2.9, "body": "Sit"},
	{"speaker": "Anna", "startTime": 3, "endTime": 3.4, "body": "\"down.\""}, {"speaker": "Anna", "startTime": 3.5, "endTime": 3.9, "body": "Tea"}]}`

// Ports the cue tests of tests/Parsers/PodcastTranscriptParserTest.php of subtitle-toolbox-php.
func TestReadPodcastCues(t *testing.T) {
	t.Parallel()

	keep := transcript.ReadOptions{KeepSegments: true}
	tests := []struct {
		name    string
		content string
		opts    transcript.ReadOptions
		want    []string
	}{
		{
			"words join by speaker and sentence end", dialogueWords,
			transcript.ReadOptions{},
			[]string{"0-0.9 <v Anna>Hello there.", "1-1.9 <v Anna>Come in!", "2-2.4 <v Ben>Thanks", "2.5-3.4 <v Anna>Sit \"down.\"", "3.5-3.9 <v Anna>Tea"},
		},
		{
			"a pause of 1 s ends a group",
			`{"segments":[{"startTime":1,"endTime":1.5,"body":"Hello"},{"startTime":60,"endTime":60.5,"body":"again"},` +
				`{"startTime":61.4,"endTime":61.8,"body":"and"},{"startTime":62.8,"endTime":63,"body":"bye"}]}`,
			transcript.ReadOptions{},
			[]string{"1-1.5 Hello", "60-61.8 again and", "62.8-63 bye"},
		},
		{
			"keep segments", `{"segments": [{"speaker": "Anna", "startTime": 0, "endTime": 0.4, "body": "Hello"}, {"speaker": "Anna", "startTime": 0.5, "endTime": 0.9, "body": "there."}]}`,
			keep,
			[]string{"0-0.4 <v Anna>Hello", "0.5-0.9 <v Anna>there."},
		},
		{
			"word timestamps of a joined group", `{"segments": [{"speaker": "Anna", "startTime": 0, "endTime": 0.4, "body": "Hello"}, ` +
				`{"speaker": "Anna", "startTime": 0.5, "endTime": 0.9, "body": "there."}, {"speaker": "Anna", "startTime": 1, "endTime": 2, "body": "Bye."}]}`,
			transcript.ReadOptions{WordTimestamps: true, SpeakerVoices: true},
			[]string{"0-0.9 <v Anna><00:00:00.000>Hello <00:00:00.500>there.", "1-2 <v Anna>Bye."},
		},
		{
			"a segment of several words never joins",
			`{"segments": [{"startTime": 0, "endTime": 2, "body": "Where are"}, {"startTime": 2, "endTime": 3, "body": "you"}, {"startTime": 3, "endTime": 4, "body": "going?"}]}`,
			transcript.ReadOptions{},
			[]string{"0-2 Where are", "2-4 you going?"},
		},
		{
			"an end from the first later start",
			`{"segments": [{"startTime": 1, "body": "One two"}, {"startTime": 1, "body": "Three four"}, {"startTime": 5, "body": "Five six"}]}`,
			transcript.ReadOptions{},
			[]string{"1-5 One two", "1-5 Three four", "5-10 Five six"},
		},
		{
			"an end from a later start that skips lower starts",
			`{"segments": [{"startTime": 4, "body": "a b"}, {"startTime": 2, "body": "c d"}, {"startTime": 3, "body": "e f"}, {"startTime": 9, "body": "g h"}]}`,
			transcript.ReadOptions{},
			[]string{"2-3 c d", "3-9 e f", "4-9 a b", "9-14 g h"},
		},
		{
			"escaped text and a trimmed speaker", `{"segments": [{"speaker": " Dr. O'Neil ", "startTime": 0, "endTime": 1, "body": " Fish & <chips>\n "}]}`,
			transcript.ReadOptions{SpeakerVoices: true},
			[]string{"0-1 <v Dr. O'Neil>Fish &amp; &lt;chips&gt;"},
		},
		{
			"segments without a body give no cue, and still end the segment before them",
			`{"segments": [{"startTime": 0, "body": "a b"}, {"speaker": "Anna", "startTime": 3}, {"startTime": 1, "endTime": 2, "body": " "}, {"startTime": 2, "endTime": 3, "body": "Hi"}]}`,
			transcript.ReadOptions{},
			[]string{"0-3 a b", "2-3 Hi"},
		},
		{
			"null fields", `{"segments": [{"speaker": null, "startTime": 0, "endTime": null, "body": null}, {"startTime": 1, "endTime": 2, "body": "x"}]}`,
			transcript.ReadOptions{},
			[]string{"1-2 x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, describe(readPodcast(t, tt.content, tt.opts)))
		})
	}
}

func TestReadPodcastWarnsWhenAGroupEndsBeforeItStarts(t *testing.T) {
	t.Parallel()

	content := `{"segments": [{"startTime": 5, "endTime": 6, "body": "a"}, {"startTime": 1, "endTime": 2, "body": "b"}]}`
	s, warnings, err := transcript.ReadPodcast(strings.NewReader(content), model.CommonReadOptions{}, transcript.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"5-5 a b"}, describe(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, "The cue ends at 2 s, before it starts at 5 s. The parser set the end to the start.", warnings[0].Message)
	assert.Equal(t, model.Repaired, warnings[0].Action)
	assert.Equal(t, 0, warnings[0].Block)
	assert.Equal(t, []string{`{"startTime":5,"endTime":6,"body":"a"}`, `{"startTime":1,"endTime":2,"body":"b"}`}, warnings[0].Lines)
}

func TestReadPodcastData(t *testing.T) {
	t.Parallel()

	content := "\xEF\xBB\xBF" + `{"version": "1.0.0", "segments": [{"startTime": 0, "endTime": 1, "body": "Hi", "confidence": 0.9}], "language": "en"}`
	s := readPodcast(t, content, transcript.ReadOptions{})
	file, ok := s.FormatData("podcast-transcript")
	require.True(t, ok)
	assert.JSONEq(t, `{"version":"1.0.0","language":"en"}`, jsonOf(file))
	c, _ := s.Cue(0)
	cueData, ok := c.FormatData("podcast-transcript")
	require.True(t, ok)
	assert.Equal(t, `{"confidence":0.9}`, jsonOf(cueData))

	joined := readPodcast(t, `{"segments": [{"startTime": 0, "endTime": 1, "body": "a", "x": 1}, {"startTime": 1, "endTime": 2, "body": "b", "x": 2}]}`, transcript.ReadOptions{})
	c, _ = joined.Cue(0)
	_, ok = c.FormatData("podcast-transcript")
	assert.False(t, ok, "a joined group has no cue data")
}

func TestReadPodcastErrors(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, content, want string }{
		{"no JSON", "{", "The content is not valid JSON: Syntax error."},
		{"root list", "[1]", "The JSON root must be an object."},
		{"no segments", `{"version": "1.0.0"}`, `The JSON has no "segments" list.`},
		{"segments as an object", `{"segments": {"0": {"startTime": 0, "body": "a"}}}`, `The JSON has no "segments" list.`},
		{"segment no object", `{"segments": ["x"]}`, "The field segments[0] must be an object."},
		{"segment an empty list", `{"segments": [[]]}`, "The field segments[0] must be an object."},
		{"start a string", `{"segments": [{"startTime": 0, "body": "Hi"}, {"startTime": "0:01", "body": "Bye"}]}`, "The field segments[1].startTime must be a number."},
		{"no start", `{"segments": [{"body": "Hi"}]}`, "The field segments[0].startTime must be a number."},
		{"negative start", `{"segments": [{"startTime": -1, "body": "Hi"}]}`, "The field segments[0].startTime must be a number."},
		{"end a string", `{"segments": [{"startTime": 0, "endTime": "1", "body": "Hi"}]}`, "The field segments[0].endTime must be a number."},
		{"speaker a number", `{"segments": [{"startTime": 0, "speaker": 7, "body": "Hi"}]}`, "The field segments[0].speaker must be a string."},
		{"body a number", `{"segments": [{"startTime": 0, "body": 7}]}`, "The field segments[0].body must be a string."},
		{"type errors come before limit errors", `{"segments": [{"startTime": 1e12, "body": 7}]}`, "The field segments[0].body must be a string."},
		{"start of 100000 hours", `{"segments": [{"startTime": 360000000, "body": "Hi"}]}`, "The field segments[0].startTime is not below 100000 hours."},
		{"end of 100000 hours", `{"segments": [{"startTime": 0, "endTime": 359999999.9996, "body": "Hi"}]}`, "The field segments[0].endTime is not below 100000 hours."},
		{"end before start", `{"segments": [{"startTime": 2, "endTime": 1.5, "body": "Hi"}]}`, "The cue ends at 1.5 s, before it starts at 2 s."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := transcript.ReadPodcast(strings.NewReader(tt.content), model.CommonReadOptions{}, transcript.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.want, perr.Msg)
			assert.Equal(t, model.PodcastTranscript, perr.Format)
		})
	}
}

// Ports testSkipsABadSegmentInLenientMode and adds the swap of a segment that ends before it starts.
func TestReadPodcastLenient(t *testing.T) {
	t.Parallel()

	content := `{"segments": [{"startTime": 0, "endTime": 1, "body": "Hi"}, {"startTime": 1, "body": 7}, "x", {"startTime": 5, "endTime": 4, "body": "Swap"}]}`
	s, warnings, err := transcript.ReadPodcast(strings.NewReader(content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"0-1 Hi", "4-5 Swap"}, describe(s))
	type row struct {
		msg    string
		block  int
		line   string
		action model.WarningAction
	}
	got := make([]row, 0, len(warnings))
	for _, w := range warnings {
		got = append(got, row{w.Message, w.Block, w.Lines[0], w.Action})
	}
	assert.Equal(t, []row{
		{"The field segments[1].body must be a string.", 1, `{"startTime":1,"body":7}`, model.Skipped},
		{"The field segments[2] must be an object.", 2, `"x"`, model.Skipped},
		{"The cue ends at 4 s, before it starts at 5 s. The parser swapped the times.", 3, `{"startTime":5,"endTime":4,"body":"Swap"}`, model.Repaired},
	}, got)
	var perr *model.ParseError
	_, _, err = transcript.ReadPodcast(strings.NewReader(`{"segments": 5}`), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
	assert.ErrorAs(t, err, &perr, "a missing list fails in lenient mode too")
}

var podcastFiles = []string{
	"spec_word_segments.json", "podcast_transcript_convert_from_srt.json", "podcast_transcript_convert_from_html.json",
}

// Ports the cue rows of tests/Parsers/PodcastTranscriptRealFilesTest.php.
func TestReadPodcastRealFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file        string
		count       int
		first, last string
	}{
		{"spec_word_segments.json", 4, "0.5-3 <v Marta>The bakery opens at seven.", "8.25-11.5 <v Marta>The rest of the year it closes."},
		{
			"podcast_transcript_convert_from_srt.json", 6, "0-2.76 <v Marta>This week we look at how to keep",
			"25.8-28.125 <v Marta>No. The fridge makes it stale faster " + string(rune(0x2013)) + " use a bread box.",
		},
		{
			"podcast_transcript_convert_from_html.json", 4, "0-7 <v Speaker 1>The library opens a new reading room on Friday.",
			"19-24 <v Speaker 2>That sounds good. Thank you.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			got := describe(readPodcastFile(t, tt.file, transcript.ReadOptions{SpeakerVoices: true}))
			require.Len(t, got, tt.count)
			assert.Equal(t, tt.first, got[0])
			assert.Equal(t, tt.last, got[len(got)-1])
		})
	}
}
