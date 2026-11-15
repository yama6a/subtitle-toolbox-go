package cli

import (
	"errors"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/ass"
)

//nolint:gochecknoglobals // the option names of the ass group, in the order of the --to ass message
var assOptions = []string{"ass-karaoke-tag", "ass-style"}

func assGroup() group {
	return group{
		name:    "ass",
		summary: "Set how ASS output writes word timestamps and the Default style.",
		options: []option{
			valueOption("ass-karaoke-tag", "TAG", "ASS karaoke tag for word timestamps: k, kf or ko. Default: k."),
			valueOption("ass-style", "STYLE", "Change the Default style, for example 'Fontname=Roboto,Fontsize=48,Outline=2'."),
		},
	}
}

func assWriteOptions(a *args) (ass.WriteOptions, bool, error) {
	tag, hasTag, err := a.Choice("ass-karaoke-tag", string(ass.KaraokeInstant), string(ass.KaraokeFill), string(ass.KaraokeOutline))
	if err != nil {
		return ass.WriteOptions{}, false, err
	}
	if hasTag && a.Has("karaoke") {
		return ass.WriteOptions{}, false, usage("Pass only one of --karaoke and --ass-karaoke-tag.")
	}
	style, hasStyle := a.Value("ass-style")
	if hasStyle && style == "" {
		// The library reads an empty Style as no change, but the CLI rejects an empty --ass-style.
		return ass.WriteOptions{}, false, usage(`The option --ass-style is not valid. The ASS style "" must hold Field=Value pairs such as Fontsize=48, got "".`)
	}
	opts := ass.WriteOptions{KaraokeTag: ass.KaraokeTag(tag), Style: style}
	if err := opts.Validate(); err != nil {
		var aerr *stbox.InvalidArgumentError
		if errors.As(err, &aerr) {
			return ass.WriteOptions{}, false, usage("The option --ass-style is not valid. " + aerr.Msg)
		}
		return ass.WriteOptions{}, false, err
	}
	return opts, hasTag || hasStyle, nil
}

// needsASSOutput rejects the options of the ass group without --to ass. It reads --to only, so an ASS input
// without --to fails too.
func needsASSOutput(a *args, to stbox.Format) error {
	if to == stbox.ASS {
		return nil
	}
	var passed []string
	for _, name := range assOptions {
		if a.Has(name) {
			passed = append(passed, "--"+name)
		}
	}
	if len(passed) == 0 {
		return nil
	}
	return usage("Pass --to ass with " + strings.Join(passed, " and ") + ".")
}
