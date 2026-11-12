package cli

import (
	"bytes"
	"strconv"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
)

// lineEndings maps the values of --line-ending to the library values. The first entry is the default.
//
//nolint:gochecknoglobals // lookup table of the --line-ending values
var lineEndings = []struct {
	name  string
	value stbox.LineEnding
}{
	{"lf", stbox.LF},
	{"crlf", stbox.CRLF},
}

// outputOptions returns the options that the write commands share. toHelp describes the default of --to.
func outputOptions(toHelp string) []option {
	return []option{
		valueOption("to", "FORMAT", toHelp),
		{
			name: "output", short: "o", value: "FILE",
			help: "Write the output of one input to this file in place of standard output. The file must not exist.",
		},
		valueOption("output-dir", "DIR",
			"Write each output into this directory, with the base name of its input. Several inputs need it."),
		valueOption("output-fps", "RATE", "Frame rate of MicroDVD, Spruce STL, iTT and MCC output. "+
			"Default: the frame rate of a MicroDVD, Spruce STL, iTT or MCC input."),
		valueOption("line-ending", "lf|crlf", "Line ending of the output. Default: "+lineEndings[0].name+"."),
		flagOption("bom", "Start the output with a UTF-8 BOM."),
		flagOption("no-bom", "Write no UTF-8 BOM. Default: the BOM rule of the output format."),
		flagOption("scc-fit", "Wrap, replace characters and delay or drop captions that SCC output cannot hold, "+
			"in place of failing. Prints each change."),
	}
}

// writeOutput is the part of a write command that turns each subtitle into an output file or standard output.
type writeOutput struct {
	to   stbox.Format
	plan *outputPlan
	opts stbox.WriteOptions
	// outputFPS is the rate of --output-fps or --fps, the zero value when the call gives neither.
	outputFPS stbox.FrameRate
	sccFit    bool
	// ass holds the write options of the ass group, nil without them.
	ass *ass.WriteOptions
}

// prepare reads --to, --output-fps, -o, --output-dir, --bom, --no-bom, --line-ending and --scc-fit. With --to, it
// checks the write options of that format.
func (w *writeOutput) prepare(a *args, r *runner) error {
	if v, ok := a.Value("to"); ok {
		f, err := writableFormat(v)
		if err != nil {
			return err
		}
		w.to = f
	}
	fps, _, err := a.Rate("output-fps")
	if err != nil {
		return err
	}
	w.outputFPS = fps
	w.sccFit = a.Has("scc-fit")
	plan, err := newOutputPlan(r.c, a, w.to)
	if err != nil {
		return err
	}
	w.plan = plan
	if err := w.prepareText(a); err != nil {
		return err
	}
	if w.to == "" {
		return nil
	}
	return w.optionsFor(w.to).Validate()
}

// prepareText reads --bom, --no-bom and --line-ending.
func (w *writeOutput) prepareText(a *args) error {
	switch {
	case a.Has("bom") && a.Has("no-bom"):
		return usage("Pass only one of --bom and --no-bom.")
	case a.Has("bom"):
		w.opts.BOM = stbox.BOMAdd
	case a.Has("no-bom"):
		w.opts.BOM = stbox.BOMRemove
	}
	names := make([]string, len(lineEndings))
	for i, le := range lineEndings {
		names[i] = le.name
	}
	name, _, err := a.Choice("line-ending", names...)
	for _, le := range lineEndings {
		if le.name == name {
			w.opts.LineEnding = le.value
		}
	}
	return err
}

// checkInputs plans the outputs of inputs and the side outputs. readPaths holds every file that the command reads.
// Without --to, the -o extension rule runs here when peek knows the input format.
func (w *writeOutput) checkInputs(r *runner, inputs, readPaths []string, side []sideOutput) error {
	if err := w.plan.check(inputs, readPaths, side, r.peek); err != nil {
		return err
	}
	r.reportToStderr = w.plan.dataOnStdout
	if w.to == "" && w.plan.output != "" {
		if f := r.peek(inputs[0]); f != "" {
			return w.plan.checkExtension(f)
		}
	}
	return nil
}

// write checks the output of in, runs edit on the subtitle when edit is not nil, and writes the result.
func (w *writeOutput) write(r *runner, in *input, edit func(*stbox.Subtitle) (*stbox.Subtitle, error)) error {
	f, target, err := w.output(in)
	if err != nil {
		return err
	}
	s := in.sub
	if edit != nil {
		if s, err = edit(s); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	report, err := stbox.Write(&buf, s, f, w.optionsFor(f))
	if err != nil {
		return err
	}
	printWriteReport(r.c, in.label(), report)
	if target == stdinArg {
		r.c.out(buf.String())
		return nil
	}
	if err := r.files.create(target, buf.Bytes()); err != nil {
		return err
	}
	r.report(in.label() + " -> " + target + "\n")
	return nil
}

// optionsFor returns the write options for output in format f. The field of a format is set only for that format,
// as docs/cli.md says.
func (w *writeOutput) optionsFor(f stbox.Format) stbox.WriteOptions {
	opts := w.opts
	if w.ass != nil {
		opts.ASS = *w.ass
	}
	switch f {
	case stbox.MicroDVD:
		opts.MicroDVD.FrameRate = w.outputFPS
	case stbox.SpruceSTL:
		opts.SpruceSTL.FrameRate = w.outputFPS
	case stbox.ITT:
		opts.ITT.FrameRate = w.outputFPS
	case stbox.MCC:
		opts.MCC.FrameRate = w.outputFPS
	case stbox.SCC:
		opts.SCC.Fit = w.sccFit
	case stbox.ASS, stbox.Cavena890, stbox.CSV, stbox.DCP, stbox.EBUSTL, stbox.JSON, stbox.LRC, stbox.MPL2, stbox.MPSub,
		stbox.PAC, stbox.PlainText, stbox.QuickTimeText, stbox.SAMI, stbox.SBV, stbox.SubRip, stbox.SubViewer,
		stbox.TMPlayer, stbox.TSV, stbox.TTML, stbox.WebVTT:
	}
	return opts
}

// output returns the output format and the checked target of in.
func (w *writeOutput) output(in *input) (f stbox.Format, target string, err error) {
	if err := w.plan.checkExtension(in.info.Format); err != nil {
		return "", "", err
	}
	f = w.to
	if f == "" {
		if !in.info.Format.CanWrite() {
			return "", "", usage("The format " + string(in.info.Format) + " can be read but not written. Pass --to with another format.")
		}
		f = in.info.Format
	}
	target = w.plan.target(in.arg, f)
	return f, target, w.plan.checkTarget(in.arg, target)
}

// printWriteReport prints the entries of the WebVTT, SCC, MCC and EBU STL reports on stderr, one line each, with the
// label of the input.
func printWriteReport(c *console, label string, r stbox.WriteReport) {
	if r.WebVTT != nil {
		for _, d := range r.WebVTT.DroppedColors {
			c.err(label + ": " + d.Message + "\n")
		}
	}
	if r.SCC != nil {
		printFitChanges(c, label, r.SCC.Changes)
	}
	if r.MCC != nil {
		printFitChanges(c, label, r.MCC.Changes)
	}
	if r.EBUSTL != nil {
		for _, rep := range r.EBUSTL.Replaced {
			c.err(label + ": " + replacedMessage(rep) + "\n")
		}
	}
}

func printFitChanges(c *console, label string, changes []scc.FitChange) {
	for _, ch := range changes {
		c.err(label + ": " + ch.Message + " (" + string(ch.Action) + ")\n")
	}
}

// replacedMessage describes a character that the EBU STL writer wrote as "?". The cue number counts from 1.
func replacedMessage(r ebustl.Replacement) string {
	where := "Comment"
	switch {
	case r.Field != "":
		where = "GSI field " + r.Field
	case r.CueIndex >= 0:
		where = "Cue " + strconv.Itoa(r.CueIndex+1)
	}
	return where + ": the character table has no " + quote(string(r.Character)) + `, so the file holds "?".`
}
