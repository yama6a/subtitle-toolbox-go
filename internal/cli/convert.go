package cli

import (
	"context"
	"io"
	"slices"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// convertCommand reads subtitle files, runs the edits of its option groups in pipeline order, and writes them.
type convertCommand struct {
	out   writeOutput
	edits []edit
	// ass holds the write options of the ass group, nil without them.
	ass *ass.WriteOptions
}

var (
	_ fileCommand = &convertCommand{}
	_ topicHelper = &convertCommand{}
)

func (*convertCommand) name() string { return "convert" }

func (*convertCommand) summary() string {
	return "Convert subtitle files to another format, and run text, structure and timing edits on the way."
}

func (*convertCommand) usage() []string {
	return []string{"<input> [--to FORMAT] [-o FILE] [options]", "<input>... [--to FORMAT] --output-dir DIR [options]"}
}

func (*convertCommand) details() string {
	return "Without --to, the output keeps the format of the input. Without -o or --output-dir, the output goes to " +
		`standard output. An input argument can be a file, a directory, a glob such as "season1/*.srt", or -. ` +
		"The tool never overwrites a file."
}

func languageOption() option {
	return valueOption("language", "CODE", "Language for --case and --errors-fix, for example en, de-AT or tr. "+
		"Without it, --errors-fix takes the language metadata of the input.")
}

// options lists the options in the order of the parser: --language, the groups, then the output and input options.
func (*convertCommand) options() []option {
	opts := []option{languageOption()}
	for _, g := range convertGroups() {
		opts = append(opts, g.options...)
	}
	return slices.Concat(opts, commonConvertOptions())
}

func commonConvertOptions() []option {
	return slices.Concat(outputOptions("Output format. Default: the input format."), inputOptions(true, writeFPSHelp))
}

func (cc *convertCommand) help(c *console, topic string) (string, error) {
	groups := convertGroups()
	i := slices.IndexFunc(groups, func(g group) bool { return g.name == topic })
	common := helpHeader(cc) + "\nOptions:\n" +
		optionTable(slices.Concat(commonConvertOptions(), []option{languageOption(), helpOption()}))
	switch {
	case i >= 0:
		return groupHelp(groups[i]), nil
	case topic == "all":
		var b strings.Builder
		b.WriteString(common)
		for _, g := range groups {
			b.WriteString("\n" + groupHelp(g))
		}
		return b.String(), nil
	case topic == "" || isFileTopic(c, topic):
		return common + groupList(groups), nil
	default:
		names := make([]string, len(groups))
		for i, g := range groups {
			names[i] = g.name
		}
		return "", usage("Unknown option group " + quote(topic) + ". The groups are " + strings.Join(names, ", ") +
			", and all for every option.")
	}
}

// isFileTopic reports whether the word after -h is a file, as in "convert in.srt -h out.srt".
func isFileTopic(c *console, topic string) bool {
	return strings.ContainsAny(topic, `./\`) || exists(c.path(topic))
}

func groupList(groups []group) string {
	rows := make([][]string, len(groups))
	for i, g := range groups {
		rows[i] = []string{g.name, g.summary}
	}
	return "\nOption groups, in the order that convert runs their edits:\n" + table(rows, 2, 2) +
		"\nRun \"" + programName + " convert --help GROUP\" for the options of a group,\n" +
		"or \"" + programName + " convert --help all\" for all options.\n"
}

func groupHelp(g group) string {
	return wrap(g.name+": "+g.summary) + optionTable(g.options)
}

func (cc *convertCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, cc)
}

// prepare runs the checks in the order of docs/cli.md: the input options, the output options, the --language
// rule, each group in pipeline order, then the --to ass rule.
func (cc *convertCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	if err := cc.out.prepare(a, r); err != nil {
		return err
	}
	if err := needsOneOf(a, []string{"case", "errors-fix"}, "language"); err != nil {
		return err
	}
	if err := cc.prepareGroups(a); err != nil {
		return err
	}
	cc.out.encode = func(in *input, out io.Writer, s *stbox.Subtitle, f stbox.Format) error {
		if f != stbox.WebVTT {
			opts := cc.out.optionsFor(f)
			if cc.ass != nil {
				opts.ASS = *cc.ass
			}
			return stbox.Write(out, s, f, opts)
		}
		common := model.CommonWriteOptions{LineEnding: cc.out.opts.LineEnding, BOM: cc.out.opts.BOM, StripTags: cc.out.opts.StripTags}
		report, err := webvtt.WriteWithReport(out, s, common, webvtt.WriteOptions{})
		if err != nil {
			return err
		}
		for _, d := range report.DroppedColors {
			r.c.err(in.label() + ": " + d.Message + "\n")
		}
		return nil
	}
	return nil
}

// prepareGroups checks the groups in pipeline order and builds their edits, then checks the --to ass rule.
func (cc *convertCommand) prepareGroups(a *args) error {
	add := func(e edit) { cc.edits = append(cc.edits, e) }
	for _, g := range convertGroups() {
		if g.writeOptions == nil {
			if err := g.build(a, add); err != nil {
				return err
			}
			continue
		}
		opts, ok, err := g.writeOptions(a)
		if err != nil {
			return err
		}
		if ok {
			cc.ass = &opts
		}
	}
	return needsASSOutput(a, cc.out.to)
}

func (cc *convertCommand) checkInputs(r *runner, inputs []string) error {
	return cc.out.checkInputs(r, inputs, inputs)
}

func (*convertCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (cc *convertCommand) process(ctx context.Context, r *runner, in *input) error {
	return cc.out.write(r, in, func(s *stbox.Subtitle) (*stbox.Subtitle, error) {
		for _, e := range cc.edits {
			var err error
			if s, err = e.apply(ctx, s, r.c, in.label()); err != nil {
				return nil, err
			}
		}
		return s, nil
	})
}

func (*convertCommand) finish(*runner) error { return nil }
