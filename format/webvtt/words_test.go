package webvtt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestReadWordsAndVoices(t *testing.T) {
	t.Parallel()

	word := func(text string, start, end time.Duration) model.Word {
		return model.Word{Text: text, Start: start, End: end}
	}
	tests := []struct {
		name      string
		settings  string
		text      string
		wantLines []string
		speaker   string
		words     []model.Word
		class     string
		end       bool
		wantData  bool
	}{
		{
			name: "short timestamp", text: "Hi <00:01.500>there",
			wantLines: []string{"Hi there"}, words: []model.Word{word("there", 1500*ms, 4*sec)},
		},
		{
			name: "short and long timestamps", text: "<00:01.000>one <00:00:02.000>two <00:03.000>three",
			wantLines: []string{"one two three"},
			words:     []model.Word{word("one", sec, 2*sec), word("two", 2*sec, 3*sec), word("three", 3*sec, 4*sec)},
		},
		{
			name: "voice class without settings", text: "<v.loud Anna>Hi</v>",
			wantLines: []string{"Hi"}, speaker: "Anna", class: ".loud", end: true, wantData: true,
		},
		{name: "voice end without settings", text: "<v Anna>Hi</v>", wantLines: []string{"Hi"}, speaker: "Anna", end: true, wantData: true},
		{name: "plain voice without settings", text: "<v Anna>Hi", wantLines: []string{"Hi"}, speaker: "Anna"},
		{
			name: "voice class with settings", settings: " align:start", text: "<v.loud Anna>Hi</v>",
			wantLines: []string{"Hi"}, speaker: "Anna", class: ".loud", end: true, wantData: true,
		},
		{name: "no voice and no settings", text: "Hi", wantLines: []string{"Hi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := parse(t, "WEBVTT\n\n00:00.000 --> 00:04.000"+tt.settings+"\n"+tt.text+"\n", false)
			c := firstCue(t, s)
			assert.Equal(t, tt.wantLines, c.Lines)
			assert.Equal(t, tt.speaker, c.Speaker)
			assert.Equal(t, tt.words, c.Words)
			d, ok := c.FormatData("vtt")
			assert.Equal(t, tt.wantData || tt.settings != "", ok)
			class, _ := d.String("voiceClass")
			end, _ := d.Bool("voiceEnd")
			assert.Equal(t, tt.class, class)
			assert.Equal(t, tt.end, end)
			if !tt.wantData && tt.settings == "" {
				assert.Zero(t, d.Len())
			}
		})
	}
}

func TestShortTimestampsWriteWithHoursAndKeepTheVoiceForm(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, in, want string }{
		{"short timestamp", "Hi <00:01.500>there", "Hi <00:00:01.500>there"},
		{"voice class", "<v.loud Anna>Hi</v>", "<v.loud Anna>Hi</v>"},
		{"voice class and timestamp", "<v.loud Anna>Hi <00:01.500>there</v>", "<v.loud Anna>Hi <00:00:01.500>there</v>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := parse(t, "WEBVTT\n\n00:00.000 --> 00:04.000\n"+tt.in+"\n", false)
			assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:00.000 --> 00:00:04.000\n"+tt.want+"\n", write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestEditedVoiceWritesFromItsFormatData(t *testing.T) {
	t.Parallel()

	s, _ := parse(t, "WEBVTT\n\n00:00.000 --> 00:04.000\n<v.loud Anna>Hi</v>\n", false)
	c := firstCue(t, s)
	c.DeleteFormatData("vtt")
	c.Lines = []string{"Hello"}
	require.NoError(t, s.SetCue(0, c))
	assert.Equal(t, bom+"WEBVTT\n\n1\n00:00:00.000 --> 00:00:04.000\n<v Anna>Hello\n", write(t, s, model.CommonWriteOptions{}))
}
