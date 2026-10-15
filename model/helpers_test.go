package model_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func sec(f float64) time.Duration {
	return time.Duration(math.Round(f * float64(time.Second)))
}

// fromTimes builds a subtitle with one cue per start and end pair. Cue N gets the text "textN".
func fromTimes(times ...[2]float64) *model.Subtitle {
	cues := make([]model.Cue, len(times))
	for i, tm := range times {
		cues[i] = model.Cue{Start: sec(tm[0]), End: sec(tm[1]), Lines: []string{fmt.Sprintf("text%d", i)}}
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func texts(s *model.Subtitle) []string {
	out := []string{}
	for _, c := range s.All() {
		out = append(out, c.Text())
	}
	return out
}

func textsAt(t *testing.T, s *model.Subtitle, indexes []int) []string {
	t.Helper()

	out := make([]string, 0, len(indexes))
	for _, i := range indexes {
		c, ok := s.Cue(i)
		require.True(t, ok, i)
		out = append(out, c.Text())
	}
	return out
}

func starts(s *model.Subtitle) []time.Duration {
	var out []time.Duration
	for _, c := range s.All() {
		out = append(out, c.Start)
	}
	return out
}

// harbourTour builds the cues and comments of tests/files/editing/harbour_tour.vtt of subtitle-toolbox-php.
func harbourTour(tb testing.TB) *model.Subtitle {
	tb.Helper()

	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: sec(1), End: sec(4), Lines: []string{"Welcome to the harbour tour."}},
		model.Cue{Start: sec(4), End: sec(6), Lines: []string{"Welcome to the harbour tour."}},
		model.Cue{Start: sec(6.5), End: sec(12.5), Lines: []string{"The old crane on the left is from 1912.", "It still works on Sundays."}},
		model.Cue{Start: sec(13), End: sec(15), Lines: []string{"<v Guide>Please keep your hands"}},
		model.Cue{Start: sec(15), End: sec(17), Lines: []string{"<v Guide>inside the boat."}},
		model.Cue{Start: sec(18), End: sec(20), Lines: []string{"Next stop, the fish market."}},
	)
	require.NoError(tb, s.AddComment("Recorded on the north pier", 0))
	require.NoError(tb, s.AddComment("Segment two starts here", 1))
	require.NoError(tb, s.AddComment("The guide speaks from here", 3))
	require.NoError(tb, s.AddComment("The boat turns left here", 4))
	require.NoError(tb, s.AddComment("End of recording", 6))
	return s
}

// weatherSigns builds the cue times of tests/files/ass/real/own_signs_crlf.ass of subtitle-toolbox-php, in file order.
func weatherSigns() *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: sec(5), End: sec(9), Lines: []string{"SUNNY", "25 C"}},
		model.Cue{Start: sec(1), End: sec(4), Lines: []string{"Good evening. Here is the weather."}},
		model.Cue{Start: sec(4.5), End: sec(7.5), Lines: []string{"Clouds move in from the west", "during the night."}},
		model.Cue{Start: sec(5), End: sec(9), Lines: []string{"m 0 0 l 200 0 200 60 0 60"}},
		model.Cue{Start: sec(8), End: sec(11), Lines: []string{"Expect light rain, mostly after 10 pm."}},
		model.Cue{Start: sec(11.2), End: sec(14), Lines: []string{"Tomorrow: dry and windy"}},
		model.Cue{Start: sec(14.1), End: sec(17.3), Lines: []string{"That's all for tonight from us."}},
	)
	return s
}
