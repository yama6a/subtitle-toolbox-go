package cli

import (
	"context"
	"slices"
	"strconv"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// dualCommand merges the primary file and the secondary file into one subtitle and writes it as convert does.
type dualCommand struct {
	out              writeOutput
	opts             dual.Options
	secondary        string
	secondaryOptions stbox.ReadOptions
}

var _ fileCommand = &dualCommand{}

func (*dualCommand) name() string { return "dual" }

func (*dualCommand) summary() string {
	return "Merge two subtitles in two languages into one file that shows both."
}

func (*dualCommand) usage() []string { return []string{"--primary FILE --secondary FILE [options]"} }

func (*dualCommand) details() string {
	return "stack adds the lines of each secondary cue below the primary cue that it overlaps most. " +
		"top-bottom keeps both cues and moves the secondary one to the top. " +
		"SubRip, WebVTT, ASS and TTML write the position. " +
		"The output takes the format of the primary file unless --to sets it. " +
		"Without -o or --output-dir, the output goes to standard output."
}

func (*dualCommand) options() []option {
	files := []option{
		valueOption("primary", "FILE", "The subtitle whose cues set the times, or - for standard input."),
		valueOption("secondary", "FILE", "The subtitle in the second language."),
		valueOption("mode", "MODE", "stack or top-bottom. Default: "+string(dual.ModeStack)+"."),
		valueOption("secondary-style", "TAG", `Tag around each secondary line: b, i, u, s or 'font color="#ffff00"'. Default: none.`),
		valueOption("secondary-alignment", "1-9", "Position of the secondary cues for top-bottom, as on a numeric keypad. Default: "+
			strconv.Itoa(dual.DefaultSecondaryAlignment.Numpad())+"."),
		valueOption("snap-tolerance", "SECONDS", "top-bottom moves a secondary time to a primary time this close. Default: "+
			formatNumber(dual.DefaultTimeTolerance.Seconds())+"."),
	}
	output := outputOptions("Output format. Default: the format of the primary file.")
	for i, o := range output {
		if o.name == "output-dir" {
			output[i].help = "Write the output into this directory, with the base name of the primary file."
		}
	}
	input := inputOptions(false, writeFPSHelp)
	for i, o := range input {
		if o.name == "from" {
			input[i] = valueOption("primary-from", "FORMAT", "Format of the primary file. Default: the format detected from the content. "+
				"When detection finds no format, the file extension sets it.")
		}
	}
	return slices.Concat(files, output, input, []option{
		valueOption("secondary-from", "FORMAT", "Format of the secondary file. Default: the format detected from the content."),
	})
}

func (d *dualCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, d)
}

// prepare takes the primary file out of the option list, so that only it goes through the input expansion.
func (d *dualCommand) prepare(a *args, r *runner) error {
	if len(a.positionals) > 0 {
		return usage("dual takes no file arguments. Pass --primary FILE and --secondary FILE.")
	}
	primary, hasPrimary := a.Value("primary")
	secondary, hasSecondary := a.Value("secondary")
	if !hasPrimary || !hasSecondary {
		return usage("Pass --primary FILE and --secondary FILE.")
	}
	if secondary == stdinArg {
		return usage("Standard input works only for --primary. Pass the secondary file by name.")
	}
	d.secondary = secondary
	r.fromOption = "primary-from"
	if err := r.prepareRead(a); err != nil {
		return err
	}
	d.secondaryOptions = r.readOptions
	d.secondaryOptions.Format = ""
	if v, ok := a.Value("secondary-from"); ok {
		var err error
		if d.secondaryOptions.Format, err = readableFormat(v); err != nil {
			return err
		}
	}
	if err := d.out.prepare(a, r); err != nil {
		return err
	}
	if err := d.prepareOptions(a); err != nil {
		return err
	}
	a.positionals = []string{primary}
	return d.opts.Validate()
}

func (d *dualCommand) prepareOptions(a *args) error {
	mode, _, err := a.Choice("mode", string(dual.ModeStack), string(dual.ModeTopBottom))
	if err != nil {
		return err
	}
	d.opts.Mode = dual.Mode(mode)
	style, hasStyle := a.Value("secondary-style")
	if hasStyle && style == "" {
		// The library reads an empty SecondaryStyle as no style, but PHP rejects an empty --secondary-style.
		return usage(`The secondary style  must be a core markup tag such as i or font color="#ffff00".`)
	}
	d.opts.SecondaryStyle = style
	n, ok, err := a.IntRange("secondary-alignment", 1, 9)
	if err != nil {
		return err
	}
	if ok {
		alignment, _ := model.AlignmentFromNumpad(int(n))
		d.opts.SecondaryAlignment = &alignment
	}
	tolerance, ok, err := a.NonNegativeTime("snap-tolerance")
	if ok {
		d.opts.TimeTolerance = &tolerance
	}
	return err
}

func (d *dualCommand) checkInputs(r *runner, inputs []string) error {
	if len(inputs) != 1 {
		return usage("--primary takes one file, got " + strconv.Itoa(len(inputs)) + ".")
	}
	return d.out.checkInputs(r, inputs, []string{inputs[0], d.secondary}, nil)
}

func (*dualCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (d *dualCommand) process(_ context.Context, r *runner, in *input) error {
	secondary, err := r.readOther(d.secondary, d.secondaryOptions, "secondary-from")
	if err != nil {
		return err
	}
	return d.out.write(r, in, func(primary *stbox.Subtitle) (*stbox.Subtitle, error) {
		return dual.FromPair(primary, secondary.sub, d.opts)
	})
}

func (*dualCommand) finish(*runner) error { return nil }
