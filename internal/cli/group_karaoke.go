package cli

import (
	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/karaoke"
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

func buildKaraoke(a *args, add func(edit)) error {
	if err := needs(a, "karaoke", "karaoke-style"); err != nil {
		return err
	}
	if !a.Has("karaoke") {
		return nil
	}
	style, hasStyle := a.Value("karaoke-style")
	if hasStyle && style == "" {
		// The library reads an empty Style as u, but the CLI rejects an empty --karaoke-style.
		return usage("The style  must be one of the core markup tags b, i, u, s or font, for example u or font color=\"#ffff00\".")
	}
	opts := karaoke.Options{Style: style}
	if err := opts.Validate(); err != nil {
		return usage(message(err))
	}
	add(steps{inPlace(func(s *stbox.Subtitle) error {
		_, err := karaoke.Apply(s, opts)
		return err
	})})
	return nil
}
