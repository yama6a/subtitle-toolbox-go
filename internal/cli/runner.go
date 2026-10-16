package cli

import (
	"context"
	"errors"
	"strconv"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// fileCommand holds the hooks of a command that reads input files. Its run method calls runFiles, which drives
// the loop of docs/cli.md.
type fileCommand interface {
	command
	// prepare reads and checks every option. Each error is a usage error.
	prepare(a *args, r *runner) error
	// checkInputs checks the expanded inputs, and for write commands the output plan. Each error is a usage error.
	checkInputs(r *runner, inputs []string) error
	// loadSideFiles reads the files that options name, once per run. A failure stops the run with exit code 3.
	loadSideFiles(ctx context.Context, r *runner) error
	// process does the work of one read input.
	process(ctx context.Context, r *runner, in *input) error
	// finish runs after the loop, for example to print the JSON of a report command.
	finish(r *runner) error
}

// runner drives the input loop of one file command call.
type runner struct {
	c           *console
	readOptions stbox.ReadOptions
	files       *outputFiles
	// reportToStderr sends the report lines and the summary to stderr, because stdout gets data.
	reportToStderr bool
	// result makes the call exit with code 1 when no input failed, for a broken rule or a difference.
	result    bool
	succeeded int
	failed    int
}

func runFiles(ctx context.Context, a *args, c *console, fc fileCommand) error {
	r := &runner{c: c, files: &outputFiles{c: c}}
	inputs, err := r.prepare(a, fc)
	if err != nil {
		return asUsage(err)
	}
	if err := fc.loadSideFiles(ctx, r); err != nil {
		return err
	}
	interrupted, err := r.loop(ctx, fc, inputs, a.Has("keep-going"))
	if err != nil {
		return err
	}
	if err := fc.finish(r); err != nil {
		return err
	}
	r.summarize(len(inputs), interrupted)
	return r.exitError(interrupted)
}

// loop processes each input until one fails without keepGoing, a *fileError stops the run, or ctx ends.
func (r *runner) loop(ctx context.Context, fc fileCommand, inputs []string, keepGoing bool) (interrupted bool, err error) {
	for _, arg := range inputs {
		if ctx.Err() != nil {
			return true, nil //nolint:nilerr // an interrupt is exit code 130, not an error of the run
		}
		ok, err := r.processInput(ctx, fc, arg)
		if err != nil || !ok && !keepGoing {
			return false, err
		}
	}
	return false, nil
}

// exitError returns the exit code of the run: 130 after an interrupt, else 3 when an input failed, else 1 for
// a result, else nil.
func (r *runner) exitError(interrupted bool) error {
	switch {
	case interrupted:
		return &ExitError{Code: exitInterrupt}
	case r.failed > 0:
		return &ExitError{Code: exitFile}
	case r.result:
		return &ExitError{Code: exitResult}
	default:
		return nil
	}
}

// prepare runs the steps before the side files: the options, the expansion and the input checks.
func (r *runner) prepare(a *args, fc fileCommand) ([]string, error) {
	if err := fc.prepare(a, r); err != nil {
		return nil, err
	}
	inputs, err := r.expand(a.positionals)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, usage("Pass at least one input file, or - for standard input.")
	}
	return inputs, fc.checkInputs(r, inputs)
}

// summarize prints why the loop stopped early, and the batch summary for 2 or more inputs.
func (r *runner) summarize(total int, interrupted bool) {
	skipped := total - r.succeeded - r.failed
	switch {
	case interrupted:
		r.c.err("Stopped by a signal.\n")
	case skipped > 0:
		r.c.err("Stopped at the first failure. Pass --keep-going to process the other files.\n")
	}
	if total < 2 {
		return
	}
	summary := strconv.Itoa(total) + " files: " + strconv.Itoa(r.succeeded) + " succeeded, " + strconv.Itoa(r.failed) + " failed"
	if skipped > 0 {
		summary += ", " + strconv.Itoa(skipped) + " skipped"
	}
	r.report(summary + ".\n")
}

// processInput reads and processes one input. A failure of the input prints its message and returns false.
// Only a *fileError returns as an error, because it stops the run.
func (r *runner) processInput(ctx context.Context, fc fileCommand, arg string) (bool, error) {
	in, err := r.read(arg)
	readFailed := err != nil
	if err == nil {
		err = fc.process(ctx, r, in)
	}
	if _, ok := errors.AsType[*fileError](err); ok {
		return false, err
	}
	if err != nil {
		r.failed++
		r.c.err(label(arg) + ": " + message(err) + "\n")
		if _, ok := errors.AsType[*stbox.ParseError](err); ok && readFailed && !r.readOptions.Lenient {
			r.c.err(lenientHint + "\n")
		}
		return false, nil
	}
	r.succeeded++
	return true, nil
}

// report prints a report line, such as "trip.srt -> out/trip.vtt" or the batch summary.
func (r *runner) report(line string) {
	if r.reportToStderr {
		r.c.err(line)
		return
	}
	r.c.out(line)
}
