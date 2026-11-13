package validation

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The limits of the YouTube chapter rules, from https://support.google.com/youtube/answer/9884579.
const (
	YouTubeMinChapters = 3
	YouTubeMinDuration = 10 * time.Second
)

// CheckYouTubeChapters lists the YouTube chapter rules that s breaks, one violation per rule and chapter. The
// order is RuleFirstChapterAtZero, RuleMinChapters, then RuleMinDuration by chapter. A last chapter whose length is
// 0 or less is skipped, because its length is unknown. RuleMinChapters has the cue index NoCue.
func CheckYouTubeChapters(s *model.Subtitle) []Violation {
	cues := s.Cues()
	var out []Violation
	if len(cues) > 0 && cues[0].Start.Truncate(time.Second) > 0 {
		out = append(out, Violation{Cue: 0, Rule: RuleFirstChapterAtZero, Value: timecode.FloatSeconds(cues[0].Start), Limit: limitOf(0)})
	}
	if len(cues) < YouTubeMinChapters {
		out = append(out, Violation{Cue: NoCue, Rule: RuleMinChapters, Value: float64(len(cues)), Limit: limitOf(YouTubeMinChapters)})
	}
	for i, c := range cues {
		d := c.End - c.Start
		if i == len(cues)-1 && d <= 0 {
			continue
		}
		if d < YouTubeMinDuration {
			out = append(out, Violation{Cue: i, Rule: RuleMinDuration, Value: timecode.FloatSeconds(d), Limit: limitOf(YouTubeMinDuration.Seconds())})
		}
	}
	return out
}
