package transcript

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestPodcastSegmentsHaveNoWordSegments(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{
		Start: 0, End: 2 * time.Second, Lines: []string{"<v Ann>Hi &amp; <v Bo>bye"}, Words: []model.Word{{Text: "bye", Start: time.Second, End: 2 * time.Second}},
	})
	assert.Equal(t, []podcastSegment{
		{speaker: "Ann", start: 0, end: 2 * time.Second, body: "Hi &", word: -1},
		{speaker: "Bo", start: 0, end: 2 * time.Second, body: "bye", word: -1},
	}, podcastSegments(s))
}
