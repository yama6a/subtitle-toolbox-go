package cli

import (
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

func timingGroup() group {
	return group{
		name:    "timing",
		summary: "Fix overlaps, short cues and long cues, and add lead-in and lead-out.",
		options: []option{
			flagOption("timing-fix-overlaps", "End each cue at least --timing-min-gap seconds before the next cue starts."),
			valueOption("timing-min-duration", "SECONDS", "Show each cue for at least this time where the next cue allows it."),
			valueOption("timing-lead-in", "SECONDS", "Start each cue this time earlier where the previous cue allows it."),
			valueOption("timing-lead-out", "SECONDS",
				"End each cue this time later where the next cue allows it. The lead-out comes before the lead-in."),
			valueOption("timing-max-duration", "SECONDS",
				"End each cue at most this time after its start. Runs after the other timing fixes."),
			valueOption("timing-min-gap", "SECONDS", "Gap between cues for --timing-fix-overlaps, "+
				"--timing-min-duration and the lead options. Default: "+timecode.Seconds(defaultMinGap)+"."),
		},
		build: buildTiming,
	}
}

const defaultMinGap time.Duration = 0

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildTiming(a *args) (edit, error) {
	gapUsers := []string{"timing-fix-overlaps", "timing-min-duration", "timing-lead-in", "timing-lead-out"}
	if err := needsOneOf(a, gapUsers, "timing-min-gap"); err != nil {
		return nil, err
	}
	var minDuration, leadIn, leadOut, maxDuration time.Duration
	minGap := defaultMinGap
	for _, v := range []struct {
		get  func(name string) (time.Duration, bool, error)
		name string
		to   *time.Duration
	}{
		{a.PositiveTime, "timing-min-duration", &minDuration},
		{a.NonNegativeTime, "timing-min-gap", &minGap},
		{a.NonNegativeTime, "timing-lead-in", &leadIn},
		{a.NonNegativeTime, "timing-lead-out", &leadOut},
		{a.PositiveTime, "timing-max-duration", &maxDuration},
	} {
		d, ok, err := v.get(v.name)
		if err != nil {
			return nil, err
		}
		if ok {
			*v.to = d
		}
	}
	st := flagSteps(a, []flagStep{
		{"timing-fix-overlaps", func(s *stbox.Subtitle) error { return s.FixOverlaps(minGap) }},
		{"timing-min-duration", func(s *stbox.Subtitle) error { return s.ExtendShortCues(minDuration, minGap) }},
	})
	if a.Has("timing-lead-in") || a.Has("timing-lead-out") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.AddLeadInOut(leadIn, leadOut, minGap) }))
	}
	if a.Has("timing-max-duration") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.LimitLongCues(maxDuration) }))
	}
	return stepsEdit(st), nil
}
