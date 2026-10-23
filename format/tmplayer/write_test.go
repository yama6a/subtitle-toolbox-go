package tmplayer_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testWritesStartTimesAndAnEmptyLineAtTheEndOfACueBeforeAGap, testAShortCueEndsOneSecondAfterItsStart and
// testStripsTagsAndSkipsCuesWithoutText of tests/Formatters/TmPlayerFormatterTest.php of subtitle-toolbox-php,
// then adds rows of its own.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cues []model.Cue
		want string
	}{
		{
			"gap entries",
			[]model.Cue{
				{Start: time.Second, End: 4 * time.Second, Lines: []string{"Where are you?", "Home."}},
				{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"Next"}},
				{Start: 3725400 * ms, End: 3727 * time.Second, Lines: []string{"Later"}},
				{Start: 3727600 * ms, End: 3730 * time.Second, Lines: []string{"Last"}},
			},
			"00:00:01:Where are you?|Home.\n00:00:04:Next\n00:00:06:\n01:02:05:Later\n01:02:07:\n01:02:08:Last\n",
		},
		{
			"a short cue ends 1 s after its start",
			[]model.Cue{
				{Start: 1100 * ms, End: 1300 * ms, Lines: []string{"Short"}},
				{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"Next"}},
			},
			"00:00:01:Short\n00:00:02:\n00:00:05:Next\n",
		},
		{
			"tags are stripped and cues without text skipped",
			[]model.Cue{
				{Start: time.Second, End: 2 * time.Second, Lines: []string{"<i>Rain</i> &amp; <b>wind</b>"}},
				{Start: 2 * time.Second, End: 3 * time.Second, Lines: []string{"<b></b>"}},
				{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"Sun"}},
			},
			"00:00:01:Rain & wind\n00:00:02:\n00:00:03:Sun\n",
		},
		{"no gap entry after the last cue", []model.Cue{{Start: time.Second, End: 2 * time.Second, Lines: []string{"Only"}}}, "00:00:01:Only\n"},
		{
			"no gap entry when the next cue starts at the end",
			[]model.Cue{
				{Start: time.Second, End: 3 * time.Second, Lines: []string{"A"}},
				{Start: 3499 * ms, End: 5 * time.Second, Lines: []string{"B"}},
			},
			"00:00:01:A\n00:00:03:B\n",
		},
		{
			"a gap entry when the next cue starts after the end",
			[]model.Cue{
				{Start: time.Second, End: 3 * time.Second, Lines: []string{"A"}},
				{Start: 3500 * ms, End: 5 * time.Second, Lines: []string{"B"}},
			},
			"00:00:01:A\n00:00:03:\n00:00:04:B\n",
		},
		{
			"a start below 0 is 0",
			[]model.Cue{
				{Start: -3 * time.Second, End: -time.Second, Lines: []string{"A"}},
				{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"B"}},
			},
			"00:00:00:A\n00:00:01:\n00:00:05:B\n",
		},
		{
			"half seconds round up",
			[]model.Cue{{Start: 1500 * ms, Lines: []string{"A"}}, {Start: 2499 * ms, Lines: []string{"B"}}},
			"00:00:02:A\n00:00:02:B\n",
		},
		{
			"hours above 99",
			[]model.Cue{{Start: 100*time.Hour + time.Second, Lines: []string{"A"}}},
			"100:00:01:A\n",
		},
		{"a ruby annotation is text", []model.Cue{{Lines: []string{"<ruby>Tokyo<rt>to</rt></ruby>"}}}, "00:00:00:Tokyo (to)\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(tt.cues...), model.CommonWriteOptions{}))
		})
	}
}

func TestWriteEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Empty(t, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
}

// Ports testAppliesTheOutputOptions of tests/Formatters/TmPlayerFormatterTest.php of subtitle-toolbox-php.
func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{End: time.Second, Lines: []string{"Hello"}})
	assert.Equal(t, "\xEF\xBB\xBF00:00:00:Hello\r\n", write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}))
	assert.Equal(t, "00:00:00:Hello\n", write(t, s, model.CommonWriteOptions{}))
	assert.Equal(t, "00:00:00:Hello\n", write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := tmplayer.Write(failingWriter{}, subtitle(model.Cue{Lines: []string{"x"}}), model.CommonWriteOptions{}, tmplayer.WriteOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}
