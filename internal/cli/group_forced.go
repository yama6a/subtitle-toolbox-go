package cli

import (
	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

func forcedGroup() group {
	return group{
		name:    "forced",
		summary: "Keep only the forced cues.",
		options: []option{flagOption("forced-only", "Keep only the forced cues, for example the translations of signs.")},
		build: func(a *args, add func(edit)) error {
			if a.Has("forced-only") {
				add(steps{func(s *stbox.Subtitle) (*stbox.Subtitle, error) { return s.ForcedCuesOnly(), nil }})
			}
			return nil
		},
	}
}
