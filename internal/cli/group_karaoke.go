package cli

import (
	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/karaoke"
)

func karaokeGroup() group {
	return group{
		name:    "karaoke",
		summary: "Write one cue per word timestamp.",
		options: []option{
			flagOption("karaoke", "Write one cue per word timestamp, with the active word styled, for players without karaoke."),
			valueOption("karaoke-style", "TAG", "Style of the active word for --karaoke: b, i, u, s or 'font color=\"#ffff00\"'. Default: u."),
		},
		build: buildKaraoke,
	}
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildKaraoke(a *args) (edit, error) {
	if err := needs(a, "karaoke", "karaoke-style"); err != nil {
		return nil, err
	}
	if !a.Has("karaoke") {
		return nil, nil //nolint:nilnil // a nil edit means the group is off
	}
	style, hasStyle := a.Value("karaoke-style")
	if hasStyle && style == "" {
		return nil, usage("The option --karaoke-style needs a tag, for example u or font color=\"#ffff00\".")
	}
	opts := karaoke.Options{Style: style}
	if err := opts.Validate(); err != nil {
		return nil, usage(message(err))
	}
	return steps{inPlace(func(s *stbox.Subtitle) error {
		_, err := karaoke.Apply(s, opts)
		return err
	})}, nil
}
