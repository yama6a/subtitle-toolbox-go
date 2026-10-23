package cli

import (
	"context"
	"slices"
	"strconv"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/refsync"
)

const (
	lowScore      = 0.5
	scaleDecimals = 5
	scoreDecimals = 2
)

// syncCommand syncs each input to the cue times of a reference subtitle and writes it as convert does.
type syncCommand struct {
	out       writeOutput
	reference string
	opts      refsync.Options
}

var _ fileCommand = &syncCommand{}

func (*syncCommand) name() string { return "sync" }

func (*syncCommand) summary() string {
	return "Find the offset and frame-rate scale against a reference subtitle, and retime the input."
}

func (*syncCommand) usage() []string { return []string{"<input>... --reference FILE [options]"} }

func (*syncCommand) details() string {
	return "Only the cue times count, so the reference can be in another language. " +
		"The scale is 1 or a factor between 23.976, 24 and 25 fps. sync prints the scale, the offset and a score " +
		"from 0 to 1. A score below " + formatNumber(lowScore) + " means that the files likely do not match. " +
		"Without -o or --output-dir, the output goes to standard output."
}

func (*syncCommand) options() []option {
	return slices.Concat([]option{
		valueOption("reference", "FILE", "Subtitle in sync with the video, in any format that the tool reads."),
		valueOption("min-offset", "SECONDS", "Smallest offset to try. Default: "+formatNumber(refsync.DefaultMinOffset.Seconds())+"."),
		valueOption("max-offset", "SECONDS", "Largest offset to try. Default: "+formatNumber(refsync.DefaultMaxOffset.Seconds())+"."),
		flagOption("no-scale", "Keep the scale at 1 and find only the offset."),
		valueOption("max-splits", "SPLITS", "Find up to this many points where the offset jumps, for example at ad breaks. "+
			"Default: "+strconv.Itoa((refsync.Options{}).MaxSplits)+"."),
		valueOption("split-penalty", "SCORE", "Score that a split must add to stay. Default: "+formatNumber(refsync.DefaultSplitPenalty)+"."),
	}, outputOptions("Output format. Default: the input format."), inputOptions(true, writeFPSHelp))
}

func (s *syncCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, s)
}

// prepare checks the sync options with an empty stand-in for the reference, so bad offsets fail before any read.
func (s *syncCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	if err := s.out.prepare(a, r); err != nil {
		return err
	}
	var ok bool
	if s.reference, ok = a.Value("reference"); !ok {
		return usage("Pass --reference FILE.")
	}
	s.opts = refsync.Options{Reference: &stbox.Subtitle{}, NoScale: a.Has("no-scale")}
	if err := s.prepareSearch(a); err != nil {
		return err
	}
	return s.opts.Validate()
}

func (s *syncCommand) prepareSearch(a *args) error {
	for _, bound := range []struct {
		name string
		dst  **time.Duration
	}{{"min-offset", &s.opts.MinOffset}, {"max-offset", &s.opts.MaxOffset}} {
		d, ok, err := a.Time(bound.name)
		if err != nil {
			return err
		}
		if ok {
			*bound.dst = &d
		}
	}
	splits, _, err := a.IntRange("max-splits", 0, refsync.MaxSplits)
	if err != nil {
		return err
	}
	s.opts.MaxSplits = int(splits)
	penalty, ok, err := a.NonNegativeFloat("split-penalty")
	if ok {
		s.opts.SplitPenalty = &penalty
	}
	return err
}

func (s *syncCommand) checkInputs(r *runner, inputs []string) error {
	return s.out.checkInputs(r, inputs, append(slices.Clone(inputs), s.reference), nil)
}

func (s *syncCommand) loadSideFiles(_ context.Context, r *runner) error {
	ref, err := r.readSideFile(s.reference)
	s.opts.Reference = ref
	return err
}

// process prints the result lines before the write, so an input that cannot be written still shows its result.
func (s *syncCommand) process(_ context.Context, r *runner, in *input) error {
	rep, err := refsync.Apply(in.sub, s.opts)
	if err != nil {
		return err
	}
	label := in.label()
	text := label + ": scale " + formatDecimals(rep.Scale, scaleDecimals) + ", offset " + formatNumber(rep.Offset.Seconds()) +
		" s, score " + formatDecimals(rep.Score, scoreDecimals) + "\n"
	if len(rep.Segments) > 1 {
		for _, seg := range rep.Segments {
			text += label + ": from " + formatNumber(seg.From.Seconds()) + " s: offset " + formatNumber(seg.Offset.Seconds()) + " s\n"
		}
	}
	if rep.Score < lowScore {
		text += label + ": the score is below " + formatNumber(lowScore) + ", so the files likely do not match.\n"
	}
	r.c.err(text)
	return s.out.write(r, in, nil)
}

func (*syncCommand) finish(*runner) error { return nil }
