package cli

import (
	"context"
	"slices"
	"strconv"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/diff"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// diffCommand lists the differences between an old and a new subtitle file.
type diffCommand struct {
	report     reportOutput
	opts       diff.Options
	newFile    string
	newOptions stbox.ReadOptions
}

var _ fileCommand = &diffCommand{}

func (*diffCommand) name() string { return "diff" }

func (*diffCommand) summary() string {
	return "List the added, removed and changed cues between two subtitle files."
}

func (*diffCommand) usage() []string { return []string{"<old> <new> [options]"} }

func (*diffCommand) details() string {
	return "The diff pairs cues by time and text, not by cue number. So one added cue does not shift the rest. " +
		"Cue numbers start at 1. The exit code is 1 when the files differ, as with the Unix diff command. " +
		"The files can have different formats. --from and --encoding apply to both files. " +
		"--new-from replaces --from for the new file."
}

func (*diffCommand) options() []option {
	return slices.Concat([]option{
		valueOption("time-tolerance", "SECONDS", "A larger difference of a start or end time is a timing change. Default: "+
			formatNumber(diff.DefaultTimeTolerance.Seconds())+"."),
		flagOption("ignore-formatting", "Compare the text without tags."),
		flagOption("ignore-whitespace", "Compare the text without spaces, tabs and line breaks."),
		flagOption("text-only", "Report no timing changes."),
		flagOption("json", "Print the differences as JSON: a list with one object for the pair of files."),
	}, inputOptions(false, reportFPSHelp), []option{
		valueOption("new-from", "FORMAT", "Format of the new file. Default: as for --from."),
	})
}

func (d *diffCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, d)
}

// prepare takes the new file out of the positional arguments, so that only the old file is expanded.
func (d *diffCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	d.report.prepare(a, r)
	d.opts = diff.Options{
		IgnoreFormatting: a.Has("ignore-formatting"),
		IgnoreWhitespace: a.Has("ignore-whitespace"),
		TextOnly:         a.Has("text-only"),
	}
	tolerance, ok, err := a.NonNegativeTime("time-tolerance")
	if err != nil {
		return err
	}
	if ok {
		d.opts.TimeTolerance = &tolerance
	}
	d.newOptions = r.readOptions
	if v, ok := a.Value("new-from"); ok {
		if d.newOptions.Format, err = readableFormat(v); err != nil {
			return err
		}
	}
	if len(a.positionals) != 2 {
		return usage("Pass two files, the old one and the new one.")
	}
	if a.positionals[1] == stdinArg {
		return usage("Standard input works only for the old file. Pass the new file by name.")
	}
	d.newFile = a.positionals[1]
	a.positionals = a.positionals[:1]
	return d.opts.Validate()
}

func (*diffCommand) checkInputs(_ *runner, inputs []string) error {
	if len(inputs) != 1 {
		return usage("The diff command takes one old file, got " + strconv.Itoa(len(inputs)) + ".")
	}
	return nil
}

func (*diffCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (d *diffCommand) process(_ context.Context, r *runner, in *input) error {
	updated, err := r.readOther(d.newFile, d.newOptions, "new-from")
	if err != nil {
		return err
	}
	differences, err := diff.Compare(in.sub, updated.sub, d.opts)
	if err != nil {
		return err
	}
	if len(differences) > 0 {
		r.result = true
	}
	d.report.emit(r, diff.Text(differences), diffEntry{
		OldFile:     in.arg,
		NewFile:     d.newFile,
		Equal:       len(differences) == 0,
		Differences: differencesJSON(differences),
		OldWarnings: warningsJSON(in.info.Warnings),
		NewWarnings: warningsJSON(updated.info.Warnings),
	})
	return nil
}

func (d *diffCommand) finish(r *runner) error { return d.report.finish(r) }

// diffEntry is the --json entry of one pair of files. Field order is the JSON key order.
type diffEntry struct {
	OldFile     string           `json:"oldFile"`
	NewFile     string           `json:"newFile"`
	Equal       bool             `json:"equal"`
	Differences []differenceJSON `json:"differences"`
	OldWarnings []warningJSON    `json:"oldWarnings"`
	NewWarnings []warningJSON    `json:"newWarnings"`
}

type differenceJSON struct {
	Kind     diff.Kind `json:"kind"`
	OldIndex *int      `json:"oldIndex"`
	NewIndex *int      `json:"newIndex"`
	Old      *cueJSON  `json:"old"`
	New      *cueJSON  `json:"new"`
}

// cueJSON is a cue of a difference. Start and End are seconds.
type cueJSON struct {
	Start  float64  `json:"start"`
	End    float64  `json:"end"`
	Lines  []string `json:"lines"`
	Forced bool     `json:"forced"`
}

func differencesJSON(differences []diff.Difference) []differenceJSON {
	out := make([]differenceJSON, len(differences))
	for i, d := range differences {
		out[i] = differenceJSON{Kind: d.Kind, Old: cueToJSON(d.Old), New: cueToJSON(d.New)}
		if d.OldIndex >= 0 {
			out[i].OldIndex = &differences[i].OldIndex
		}
		if d.NewIndex >= 0 {
			out[i].NewIndex = &differences[i].NewIndex
		}
	}
	return out
}

func cueToJSON(c *model.Cue) *cueJSON {
	if c == nil {
		return nil
	}
	lines := c.Lines
	if lines == nil {
		lines = []string{}
	}
	return &cueJSON{Start: secondsOf(c.Start), End: secondsOf(c.End), Lines: lines, Forced: c.Forced}
}

// secondsOf returns the whole milliseconds of d as seconds, so 6200 ms gives 6.2.
func secondsOf(d time.Duration) float64 {
	return float64(d.Milliseconds()) / 1000
}
