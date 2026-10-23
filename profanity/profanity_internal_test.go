package profanity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestApplyTimeoutChangesNothing(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"hell", "x"}})
	s.AddCue(model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"Go to hell."}})

	// A negative timeout puts the regexp2 deadline in the past, so the first match attempt fails on any CPU.
	_, err := apply(&s, Options{Words: []string{"hell"}}, -time.Hour)
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	var aerr *model.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "The word match took longer than 1 s on cue 1.", aerr.Msg)
	assert.Equal(t, [][]string{{"hell", "x"}, {"Go to hell."}}, [][]string{s.Cues()[0].Lines, s.Cues()[1].Lines})
}
