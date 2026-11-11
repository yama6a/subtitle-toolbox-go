package cli

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v2"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
	"github.com/yama6a/subtitle-toolbox-go/v2/validation"
)

// validateCommand checks the cues of each input against the rules that its options turn on.
type validateCommand struct {
	report reportOutput
	rules  validation.Rules
}

var _ fileCommand = &validateCommand{}

func (*validateCommand) name() string { return "validate" }

func (*validateCommand) summary() string {
	return "Check the cues against reading speed, line length, timing and text rules."
}

func (*validateCommand) usage() []string {
	return []string{"<input>... [--max-cpl CHARS] [--check-overlaps] [...] [options]"}
}

func (*validateCommand) details() string {
	return "Prints one line per broken rule. Cue numbers start at 1. The exit code is 1 when a file breaks a rule."
}

func (*validateCommand) options() []option {
	return slices.Concat([]option{
		valueOption("max-cps", "CHARS", "Maximum characters per second."),
		valueOption("max-cpl", "CHARS", "Maximum characters per line."),
		valueOption("max-lines", "LINES", "Maximum lines per cue."),
		valueOption("min-duration", "SECONDS", "Minimum duration of a cue."),
		valueOption("max-duration", "SECONDS", "Maximum duration of a cue."),
		valueOption("min-gap", "SECONDS", "Minimum gap between cues."),
		flagOption("check-overlaps", "Report overlapping cues."),
		flagOption("check-empty-cues", "Report cues without text."),
		valueOption("max-wpm", "WORDS", "Maximum words per minute."),
		valueOption("min-seconds-per-word", "SECONDS", "Minimum duration of a cue per word."),
		valueOption("max-speakers", "SPEAKERS", "Maximum speakers per cue, from dialogue dashes or <v> names."),
		valueOption("dialogue-dash", "STYLE", `Report dialogue dashes in another style than STYLE, for example "- " or "-".`),
		valueOption("allowed-characters", "LIST", `Report other characters. LIST is a list or a class such as "[A-Za-z0-9 .,!?]".`),
		flagOption("check-double-spaces", "Report two or more spaces between words."),
		flagOption("check-leading-or-trailing-spaces", "Report lines that start or end with a space."),
		flagOption("check-unbalanced-tags", "Report formatting tags without a partner tag."),
		flagOption("check-all-caps-lines", "Report lines in upper case only."),
		flagOption("json", jsonHelp),
	}, inputOptions(true, reportFPSHelp))
}

func (c *validateCommand) run(ctx context.Context, a *args, console *console) error {
	return runFiles(ctx, a, console, c)
}

func (c *validateCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	c.report.prepare(a, r)
	r.summarizeProblems = true
	rules, err := validateRules(a)
	if err != nil {
		return err
	}
	if err := rules.Validate(); err != nil {
		return usage(message(err))
	}
	if rules == (validation.Rules{}) {
		return usage("Pass at least one rule option.")
	}
	c.rules = rules
	return nil
}

// validateRules reads the rule options. The first bad value gives the usage error.
func validateRules(a *args) (validation.Rules, error) {
	var (
		rules validation.Rules
		err   error
	)
	if rules.MaxCharactersPerSecond, _, err = a.PositiveFloat("max-cps"); err != nil {
		return rules, err
	}
	for _, o := range []struct {
		name string
		to   *int
	}{{"max-cpl", &rules.MaxCharactersPerLine}, {"max-lines", &rules.MaxLinesPerCue}} {
		n, _, err := a.PositiveInt(o.name)
		if err != nil {
			return rules, err
		}
		*o.to = int(n)
	}
	for _, o := range []struct {
		name string
		to   *time.Duration
	}{{"min-duration", &rules.MinDuration}, {"max-duration", &rules.MaxDuration}, {"min-gap", &rules.MinGap}} {
		d, _, err := a.PositiveTime(o.name)
		if err != nil {
			return rules, err
		}
		*o.to = d
	}
	if rules.DialogueDashStyle, err = dialogueDashStyle(a); err != nil {
		return rules, err
	}
	n, _, err := a.PositiveInt("max-speakers")
	if err != nil {
		return rules, err
	}
	rules.MaxSpeakersPerCue = int(n)
	if rules.MaxWordsPerMinute, _, err = a.PositiveFloat("max-wpm"); err != nil {
		return rules, err
	}
	if rules.MinDurationPerWord, _, err = a.PositiveTime("min-seconds-per-word"); err != nil {
		return rules, err
	}
	rules.AllowedCharacters, _ = a.Value("allowed-characters")
	rules.NoOverlap = a.Has("check-overlaps")
	rules.NoEmptyCues = a.Has("check-empty-cues")
	rules.NoDoubleSpaces = a.Has("check-double-spaces")
	rules.NoLeadingOrTrailingSpaces = a.Has("check-leading-or-trailing-spaces")
	rules.NoUnbalancedTags = a.Has("check-unbalanced-tags")
	rules.NoAllCapsLines = a.Has("check-all-caps-lines")
	return rules, nil
}

func dialogueDashStyle(a *args) (*model.DialogueDashStyle, error) {
	v, ok := a.Value("dialogue-dash")
	if !ok {
		return nil, nil //nolint:nilnil // nil means the option is not given
	}
	for s := model.DialogueDashHyphenSpace; s.Text() != ""; s++ {
		if s.Text() == v {
			return &s, nil
		}
	}
	return nil, usage("The option --dialogue-dash must be a hyphen, a Unicode hyphen, an en dash or an em dash, " +
		"with or without one space after it, got " + quote(v) + ".")
}

func (*validateCommand) checkInputs(*runner, []string) error          { return nil }
func (*validateCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (c *validateCommand) process(_ context.Context, r *runner, in *input) error {
	violations, err := validation.Check(in.sub, c.rules)
	if err != nil {
		return err //nolint:wrapcheck // the *InvalidArgumentError carries the message that the CLI prints
	}
	if len(violations) > 0 {
		r.results++
	}
	var text strings.Builder
	entries := make([]violationJSON, len(violations))
	for i, v := range violations {
		text.WriteString(violationLine(in.label(), v))
		entries[i] = violationToJSON(v)
	}
	if len(violations) == 0 {
		text.WriteString(in.label() + ": no problems\n")
	}
	c.report.emit(r, text.String(), validateEntry{
		File:       in.arg,
		Format:     in.info.Format,
		Valid:      len(violations) == 0,
		Violations: entries,
		Warnings:   warningsJSON(in.info.Warnings),
	})
	return nil
}

func (c *validateCommand) finish(r *runner) error { return c.report.finish(r) }

// violationLine returns the text line of v, such as "trip.srt: cue 2: noOverlap 0.5".
func violationLine(label string, v validation.Violation) string {
	line := label + ": "
	if v.Cue != validation.NoCue {
		line += "cue " + strconv.Itoa(v.Cue+1) + ": "
	}
	line += string(v.Rule) + " " + formatNumber(v.Value)
	if v.Limit != nil {
		line += ", limit " + formatNumber(*v.Limit)
	}
	return line + "\n"
}

// validateEntry is the --json entry of one input. Field order is the JSON key order.
type validateEntry struct {
	File       string          `json:"file"`
	Format     stbox.Format    `json:"format"`
	Valid      bool            `json:"valid"`
	Violations []violationJSON `json:"violations"`
	Warnings   []warningJSON   `json:"warnings"`
}

// violationJSON is one violation of a --json entry. encoding/json cannot write +Inf, so Value is nil and
// Infinite is true for it.
type violationJSON struct {
	CueIndex *int            `json:"cueIndex"` // nil prints null for a rule about the whole subtitle
	Rule     validation.Rule `json:"rule"`
	Value    *float64        `json:"value"`
	Infinite bool            `json:"infinite"`
	Limit    *float64        `json:"limit"`
}

func violationToJSON(v validation.Violation) violationJSON {
	out := violationJSON{Rule: v.Rule, Infinite: math.IsInf(v.Value, 0), Limit: v.Limit}
	if v.Cue != validation.NoCue {
		out.CueIndex = &v.Cue
	}
	if !out.Infinite {
		out.Value = &v.Value
	}
	return out
}
