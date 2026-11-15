package cli

import (
	"context"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/profanity"
)

func maskingGroup() group {
	return group{
		name:    "masking",
		summary: "Mask words and write the mute times for players.",
		options: []option{
			valueOption("mask-words", "FILE", "Mask the words of this file, one per line. A * at the end matches any ending."),
			valueOption("mask", "STYLE", "How --mask-words masks a word: stars, first-letter, remove, or none to keep the text. "+
				"Default: "+string(profanity.MaskStars)+"."),
			valueOption("mute-edl", "FILE", "Write the times of the --mask-words matches to this EDL (edit decision list) file. "+
				"Kodi and MPlayer read it to mute the audio."),
			valueOption("mute-filter", "FILE", "Write an FFmpeg volume filter that mutes the --mask-words matches to this file."),
			valueOption("mute-padding", "SECONDS", "Widen each mute range by this time on both sides. Default: 0."),
		},
		build: buildMasking,
	}
}

func buildMasking(a *args, add func(edit)) error {
	if err := needs(a, "mask-words", "mask", "mute-edl", "mute-filter", "mute-padding"); err != nil {
		return err
	}
	words, ok := a.Value("mask-words")
	if !ok {
		return nil
	}
	mask, _, err := a.Choice("mask", string(profanity.MaskStars), string(profanity.MaskFirstLetter),
		string(profanity.MaskRemove), string(profanity.MaskNone))
	if err != nil {
		return err
	}
	m := &maskingEdit{wordsPath: words, opts: profanity.Options{Mask: profanity.Mask(mask)}}
	for _, o := range []muteFile{
		{sideOutput{option: "mute-edl"}, profanity.EDL},
		{sideOutput{option: "mute-filter"}, profanity.FFmpegVolumeFilter},
	} {
		path, ok := a.Value(o.option)
		if !ok {
			continue
		}
		if path == stdinArg {
			return usage("The option --" + o.option + " needs a file path.")
		}
		o.path = path
		m.muteFiles = append(m.muteFiles, o)
	}
	if m.opts.Padding, _, err = a.NonNegativeTime("mute-padding"); err != nil {
		return err
	}
	add(m)
	return nil
}

// maskingEdit masks the words of the --mask-words file and keeps the mute ranges of the last input for the mute
// files.
type maskingEdit struct {
	wordsPath string
	opts      profanity.Options
	muteFiles []muteFile
	ranges    []profanity.MuteRange
}

// muteFile is the file of --mute-edl or --mute-filter and the function that writes its content.
type muteFile struct {
	sideOutput
	render func([]profanity.MuteRange) string
}

func (m *maskingEdit) apply(_ context.Context, s *stbox.Subtitle, _ *console, _ string) (*stbox.Subtitle, error) {
	report, err := profanity.Apply(s, m.opts)
	m.ranges = report.MuteRanges
	return s, err
}

// loadWords reads one word per line. A UTF-8 BOM, the line endings CR LF, LF and CR, the spaces around a word and
// empty lines do not count. A word list that fails Validate is a usage error.
func (m *maskingEdit) loadWords(c *console) error {
	content, err := readSideText(c, m.wordsPath)
	if err != nil {
		return err
	}
	text := string(content)
	for strings.HasPrefix(text, "\ufeff") {
		text = strings.TrimPrefix(text, "\ufeff")
	}
	for _, line := range parsing.Lines(text) {
		if word := strings.Trim(line, markupcore.TrimSet); word != "" {
			m.opts.Words = append(m.opts.Words, word)
		}
	}
	if err := m.opts.Validate(); err != nil {
		return usage(message(err))
	}
	return nil
}

// writeMuteFiles creates the mute files with the ranges of the last input. A file that is not empty ends with LF.
func (m *maskingEdit) writeMuteFiles(r *runner, label string) error {
	for _, f := range m.muteFiles {
		content := f.render(m.ranges)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if err := r.files.create(f.path, []byte(content)); err != nil {
			return err
		}
		r.report(label + " -> " + f.path + "\n")
	}
	return nil
}
