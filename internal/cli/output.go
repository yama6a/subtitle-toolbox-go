package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
)

// outputPlan plans the output file of each input of a write command from -o, --output-dir and --to, and
// checks it before the first read. No output may exist, be a file that the command reads, or be the output of 2
// inputs.
type outputPlan struct {
	c         *console
	output    string // -o, "" when not given
	outputDir string // --output-dir, "" when not given
	to        stbox.Format
	// readPaths holds the real paths of the files that the command reads.
	readPaths []string
	// planned maps an input to the real path of its output, where the check could tell it.
	planned map[string]string
	// dataOnStdout reports that the subtitle data goes to standard output or comes from standard input.
	dataOnStdout bool
}

// newOutputPlan runs the checks of the prepare step.
func newOutputPlan(c *console, a *args, to stbox.Format) (*outputPlan, error) {
	p := &outputPlan{c: c, to: to}
	p.output, _ = a.Value("output")
	p.outputDir, _ = a.Value("output-dir")
	switch {
	case a.Has("output") && a.Has("output-dir"):
		return nil, usage("Pass only one of --output and --output-dir.")
	case p.outputDir != "" && exists(c.realTarget(p.outputDir)) && !isDir(c.realTarget(p.outputDir)):
		return nil, usage("The --output-dir " + p.outputDir + " is a file. Pass a directory.")
	case a.Has("output"):
		return p, p.checkOutput()
	default:
		return p, nil
	}
}

// checkOutput fails for an -o that cannot be a new file, or whose extension names another format than --to.
func (p *outputPlan) checkOutput() error {
	out := p.output
	last := out[strings.LastIndex(out, "/")+1:]
	switch {
	case strings.HasSuffix(out, "/"):
		return usage("The output " + out + " ends with a slash. Pass a file name, or pass --output-dir " + out + ".")
	case last == "." || last == "..":
		return usage("The output " + out + " ends with " + quote(last) + ". Pass a file name, or pass --output-dir " + out + ".")
	case out == stdinArg:
		return nil
	case isDir(p.c.realTarget(out)):
		return usage("The output " + out + " is a directory. Pass --output-dir " + out + ".")
	}
	if named, ok := stbox.FormatFromPath(out); ok && p.to != "" && !slices.Contains(p.to.Extensions(), extension(out)) {
		return usage("The extension of --output " + out + " names the format " + string(named) +
			", not the --to format " + string(p.to) + ".")
	}
	return nil
}

// sideOutput is a file that a command writes besides the subtitles, such as the file of --mute-edl.
type sideOutput struct {
	option string
	path   string
}

// check plans every output before the first read, the side outputs last. peek gives the input format for
// --output-dir without --to.
func (p *outputPlan) check(inputs, readPaths []string, side []sideOutput, peek func(string) stbox.Format) error {
	if err := p.checkCount(inputs); err != nil {
		return err
	}
	p.readPaths = p.realPaths(readPaths)
	p.dataOnStdout = p.output == stdinArg || slices.Contains(inputs, stdinArg) || p.output == "" && p.outputDir == ""
	p.planned = make(map[string]string)
	writers := make(map[string]string)
	for _, in := range inputs {
		target := p.plannedTarget(in, peek)
		if target == "" || target == stdinArg {
			continue
		}
		realPath := p.c.realTarget(target)
		if first, ok := writers[realPath]; ok {
			return usage(first + " and " + in + " would both write " + target + ". Pass them in two runs.")
		}
		if err := p.checkNewFile(realPath, "The output "+target); err != nil {
			return err
		}
		writers[realPath] = in
		p.planned[in] = realPath
	}
	return p.checkSideOutputs(side, writers)
}

// checkSideOutputs checks the side outputs after the subtitle outputs. writers maps the real path of each planned
// output to the input that writes it.
func (p *outputPlan) checkSideOutputs(side []sideOutput, writers map[string]string) error {
	for _, o := range side {
		realPath := p.c.realTarget(o.path)
		subject := "The --" + o.option + " file " + o.path
		if first, ok := writers[realPath]; ok {
			return usage(subject + " is also the output of " + first + ".")
		}
		if err := p.checkNewFile(realPath, subject); err != nil {
			return err
		}
		writers[realPath] = "--" + o.option
	}
	return nil
}

// checkCount checks that the inputs fit -o and --output-dir.
func (p *outputPlan) checkCount(inputs []string) error {
	n := len(inputs)
	switch {
	case p.output != "" && n > 1:
		return usage("--output takes one input file, got " + strconv.Itoa(n) + ". Pass --output-dir DIR for several files.")
	case n > 1 && p.outputDir == "":
		return usage(strconv.Itoa(n) + " input files need --output-dir DIR. One input file goes to standard output or to -o FILE.")
	case p.outputDir != "" && slices.Contains(inputs, stdinArg):
		return usage("Standard input has no file name for --output-dir. Pass -o FILE.")
	default:
		return nil
	}
}

// realPaths returns the real paths of the paths that exist.
func (p *outputPlan) realPaths(paths []string) []string {
	var out []string
	for _, path := range paths {
		if path == stdinArg {
			continue
		}
		if realPath, err := filepath.EvalSymlinks(p.c.path(path)); err == nil {
			abs, _ := filepath.Abs(realPath)
			out = append(out, abs)
		}
	}
	return out
}

// plannedTarget returns the output of input, or "" when only the read can tell it.
func (p *outputPlan) plannedTarget(input string, peek func(string) stbox.Format) string {
	if input == stdinArg || p.outputDir == "" {
		return p.target(input, "")
	}
	f := p.to
	if f == "" {
		if f = peek(input); !f.CanWrite() {
			return ""
		}
	}
	return p.target(input, f)
}

// target returns the output of input: standard output as "-", the -o file, or a file in --output-dir with the
// base name of the input and an extension of f.
func (p *outputPlan) target(input string, f stbox.Format) string {
	if p.output != "" {
		return p.output
	}
	if input == stdinArg || p.outputDir == "" {
		return stdinArg
	}
	name := filepath.Base(input)
	exts := f.Extensions()
	if !slices.Contains(exts, extension(name)) {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + "." + exts[0]
	}
	return strings.TrimRight(p.outputDir, `/\`) + "/" + name
}

// checkTarget checks an output that check could not plan, because only the read told the format of the input.
func (p *outputPlan) checkTarget(input, target string) error {
	if target == stdinArg {
		return nil
	}
	realPath := p.c.realTarget(target)
	if realPath == p.planned[input] {
		return nil
	}
	for _, planned := range p.planned {
		if planned == realPath {
			return usage("The output " + target + " is also the output of another input.")
		}
	}
	return p.checkNewFile(realPath, "The output "+target)
}

// checkNewFile checks a new output file. subject names it at the start of the message, such as "The output x.srt".
func (p *outputPlan) checkNewFile(realPath, subject string) error {
	if slices.Contains(p.readPaths, realPath) {
		return usage(subject + " is a file that the command reads. Pass another output file or directory.")
	}
	if exists(realPath) {
		return usage(subject + " exists. The tool never overwrites a file. " +
			"Remove it, or pass another output file or directory.")
	}
	return nil
}

// checkExtension fails without --to when the extension of -o names a writable format other than the input
// format f. Writing SubRip into movie.vtt would look like a conversion that did not happen.
func (p *outputPlan) checkExtension(f stbox.Format) error {
	if p.to != "" || p.output == "" || p.output == stdinArg {
		return nil
	}
	named, ok := stbox.FormatFromPath(p.output)
	if !ok || !named.CanWrite() || slices.Contains(f.Extensions(), extension(p.output)) {
		return nil
	}
	return usage("The output " + p.output + " has the extension of the format " + string(named) +
		", not of the input format " + string(f) + ". Pass --to " + string(named) +
		" to write that format, or use the extension of the input format.")
}

// extension returns the lower-case extension of path without the dot.
func extension(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// realTarget returns the real path of a file that may not exist yet: the real path of its nearest existing
// parent, plus the rest of the path with "." and ".." folded.
func (c *console) realTarget(path string) string {
	path = c.path(path)
	if !filepath.IsAbs(path) {
		if wd, err := os.Getwd(); err == nil {
			path = wd + string(filepath.Separator) + path
		}
	}
	return realTarget(path)
}

func realTarget(path string) string {
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		abs, _ := filepath.Abs(realPath)
		return abs
	}
	trimmed := strings.TrimRightFunc(path, func(r rune) bool { return os.IsPathSeparator(uint8(r)) })
	i := strings.LastIndexFunc(trimmed, func(r rune) bool { return os.IsPathSeparator(uint8(r)) })
	if trimmed == "" || i < 0 {
		return path
	}
	parent, name := trimmed[:i], trimmed[i+1:]
	if parent == "" || strings.HasSuffix(parent, ":") {
		parent += string(filepath.Separator)
	}
	realParent := realTarget(parent)
	switch name {
	case ".":
		return realParent
	case "..":
		return filepath.Dir(realParent)
	default:
		return filepath.Join(realParent, name)
	}
}

// outputFiles creates the files of one run with exclusive create, so it never overwrites a file. When a create
// or a write fails, it removes every file and directory that it created in the run.
type outputFiles struct {
	c     *console
	files []string
	dirs  []string
}

func (o *outputFiles) create(path string, content []byte) error {
	realPath := o.c.realTarget(path)
	if err := o.createDir(filepath.Dir(realPath)); err != nil {
		return err
	}
	if o.c.beforeCreate != nil {
		o.c.beforeCreate(path)
	}
	f, err := os.OpenFile(realPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if exists(realPath) {
			return o.fail(path + " exists.")
		}
		return o.fail("Cannot create " + path + ".")
	}
	o.files = append(o.files, realPath)
	_, err = f.Write(content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return o.fail("Cannot write " + path + ".")
	}
	return nil
}

// createDir creates dir and its missing parents, deepest last.
func (o *outputFiles) createDir(dir string) error {
	if isDir(dir) {
		return nil
	}
	if parent := filepath.Dir(dir); parent != dir && !exists(parent) {
		if err := o.createDir(parent); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o777); err != nil {
		return o.fail("Cannot create the directory " + dir + ".")
	}
	o.dirs = append(o.dirs, dir)
	return nil
}

func (o *outputFiles) fail(msg string) error {
	for _, f := range o.files {
		_ = os.Remove(f)
	}
	for _, d := range slices.Backward(o.dirs) {
		_ = os.Remove(d)
	}
	switch n := len(o.files); n {
	case 0:
	case 1:
		msg += " Removed the file that this run wrote."
	default:
		msg += " Removed the " + strconv.Itoa(n) + " files that this run wrote."
	}
	o.files, o.dirs = nil, nil
	return &fileError{msg: msg}
}
