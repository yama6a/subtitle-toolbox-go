package cli

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/textenc"
)

const stdinArg = "-"

const vobSubStdinMessage = "VobSub needs the path of the .idx file. Standard input does not work."

// rollUps lists the values of --scc-roll-up. The first is the default.
//
//nolint:gochecknoglobals // lookup table of the --scc-roll-up values
var rollUps = []string{string(scc.RollUpScreen), string(scc.RollUpLines)}

// The help texts of --fps. A command that writes subtitles gets writeFPSHelp, a command that reports gets reportFPSHelp.
// validate has its own text, because --fps also sets --video-fps.
const (
	writeFPSHelp  = "Sets --input-fps and --output-fps. A specific option wins over --fps."
	reportFPSHelp = "Same as --input-fps."
	// convertFPSHelp also names --video-fps of the snap group.
	convertFPSHelp = "Sets --input-fps, --output-fps and --video-fps. A specific option wins over --fps."
)

// inputOptions returns the options of a command that reads subtitle files. A command that reads one input only
// has no --keep-going. fpsHelp is the help text of --fps.
func inputOptions(manyInputs bool, fpsHelp string) []option {
	opts := []option{
		valueOption("from", "FORMAT", "Input format. Default: the format detected from the content. "+
			"When detection finds no format, the file extension sets it."),
		valueOption("encoding", "NAME", "Encoding of input that is not UTF-8, for example Windows-1252. "+
			"It replaces code page detection. A BOM, valid UTF-8 and UTF-16 input win over it."),
		flagOption("lenient", "Skip or repair broken cues and print a warning for each."),
		valueOption("input-fps", "RATE", "Frame rate of a MicroDVD input without a {1}{1}<fps> first line, "+
			"of Spruce STL timecodes, and of CSV or TSV times in hh:mm:ss:ff."),
		valueOption("fps", "RATE", fpsHelp),
		valueOption("scc-roll-up", "MODE", "How SCC and MCC input reads roll-up captions: screen gives one cue per screen, "+
			"lines gives one cue per row. Default: "+rollUps[0]+"."),
	}
	if manyInputs {
		opts = append(opts, flagOption("keep-going",
			"Go on with the next file after a file fails. Default: stop at the first failure."))
	}
	return opts
}

// findFormat returns the format of a --from or --to value: a format name or a file extension, in any ASCII case
// and with leading dots. A format that can be neither read nor written is unknown. An extension names the format
// that stbox.FormatFromPath gives, so txt names plain text, not MPL2.
func findFormat(v string) (stbox.Format, error) {
	key := strings.TrimLeft(asciiLower(v), ".")
	usable := func(f stbox.Format) bool { return f.CanRead() || f.CanWrite() }
	for _, f := range stbox.Formats() {
		if string(f) == key && usable(f) {
			return f, nil
		}
	}
	if f, ok := stbox.FormatFromPath("file." + key); ok && usable(f) {
		return f, nil
	}
	return "", usage("Unknown format " + quote(v) + `. Run "stbox formats" for the list.`)
}

func readableFormat(v string) (stbox.Format, error) {
	f, err := findFormat(v)
	if err == nil && !f.CanRead() {
		return "", usage("The format " + string(f) + " can be written but not read.")
	}
	return f, err
}

func writableFormat(v string) (stbox.Format, error) {
	f, err := findFormat(v)
	if err == nil && !f.CanWrite() {
		return "", usage("The format " + string(f) + " can be read but not written.")
	}
	return f, err
}

// input is one read input of a file command.
type input struct {
	// arg is the input as given: a path, or "-" for standard input.
	arg string
	sub *stbox.Subtitle
	// info holds the format, the source encoding and the warnings of the read.
	info stbox.ReadInfo
}

// label names the input in messages: the path as given, or "stdin".
func (in *input) label() string {
	return label(in.arg)
}

func label(arg string) string {
	if arg == stdinArg {
		return "stdin"
	}
	return arg
}

// prepareRead builds the read options of --input-fps, --scc-roll-up, --from, --encoding and --lenient and checks them.
// A command that renames --from sets fromOption first.
func (r *runner) prepareRead(a *args) error {
	fps, ok, err := a.Rate("input-fps")
	if err != nil {
		return err
	}
	if ok {
		r.readOptions.MicroDVD.FrameRate = fps
		r.readOptions.CSV.FrameRate = fps
		r.readOptions.SpruceSTL.FrameRate = fps
	}
	if rollUp, ok, err := a.Choice("scc-roll-up", rollUps...); err != nil {
		return err
	} else if ok {
		r.readOptions.SCC.RollUp = scc.RollUp(rollUp)
		r.readOptions.MCC.RollUp = mcc.RollUp(rollUp)
	}
	if v, ok := a.Value(cmp.Or(r.fromOption, "from")); ok {
		f, err := readableFormat(v)
		if err != nil {
			return err
		}
		r.readOptions.Format = f
	}
	r.readOptions.Encoding, _ = a.Value("encoding")
	r.readOptions.Lenient = a.Has("lenient")
	return r.readOptions.Validate()
}

// read reads one input with the read options of the call and prints its warnings.
func (r *runner) read(arg string) (*input, error) {
	return r.readWith(arg, r.readOptions)
}

// readWith reads one input with opts and prints its warnings.
func (r *runner) readWith(arg string, opts stbox.ReadOptions) (*input, error) {
	in := &input{arg: arg}
	var err error
	if arg == stdinArg {
		if opts.Format == stbox.VobSub {
			return nil, usage(vobSubStdinMessage)
		}
		var content []byte
		if content, err = io.ReadAll(r.c.stdin); err != nil {
			return nil, fmt.Errorf("read standard input: %w", err)
		}
		in.sub, in.info, err = stbox.Parse(bytes.NewReader(content), int64(len(content)), opts)
	} else {
		path := r.c.path(arg)
		if st, statErr := os.Stat(path); statErr != nil || !st.Mode().IsRegular() {
			return nil, usage(missingFileMessage)
		}
		in.sub, in.info, err = stbox.Load(path, opts)
	}
	if err != nil {
		return nil, err
	}
	r.printWarnings(in)
	return in, nil
}

// readOther reads a file other than the input of the call, such as the new file of diff, and prints its warnings.
// A failure carries the path as its label. A file with no detectable format gets the advice for fromOption.
func (r *runner) readOther(path string, opts stbox.ReadOptions, fromOption string) (*input, error) {
	in, err := r.readWith(path, opts)
	if errors.Is(err, stbox.ErrUnknownFormat) {
		err = usage(unknownFormatAdvice(fromOption))
	}
	if err != nil {
		return nil, &labeledError{label: path, err: err}
	}
	return in, nil
}

// printWarnings prints the warnings of a read as "LABEL: line N: MESSAGE (ACTION)". A decode warning gets the
// --encoding hint when the call gives no --encoding.
func (r *runner) printWarnings(in *input) {
	for _, w := range in.info.Warnings {
		line := ""
		if w.Line != stbox.NoLine {
			line = "line " + strconv.Itoa(w.Line) + ": "
		}
		r.c.err(in.label() + ": " + line + warningMessage(w) + " (" + w.Action.String() + ")\n")
		if r.readOptions.Encoding == "" &&
			(strings.HasPrefix(w.Message, textenc.UTF16WarningPrefix) || strings.HasPrefix(w.Message, textenc.DetectedWarningPrefix)) {
			r.c.err(encodingHint + "\n")
		}
	}
}

// peek returns the format that the read of arg will pick, without a message: --from, else the detected format,
// else the extension fallback of docs/detection.md. It returns "" for standard input and on any error.
func (r *runner) peek(arg string) stbox.Format {
	path := r.c.path(arg)
	if st, err := os.Stat(path); arg == stdinArg || err != nil || !st.Mode().IsRegular() {
		return ""
	}
	if r.readOptions.Format != "" {
		return r.readOptions.Format
	}
	if f, ok := r.detect(path); ok {
		return stbox.Format(extfallback.Profile(string(f), path))
	}
	if f, ok := stbox.FormatFromPath(arg); ok && f.CanRead() && extfallback.Allowed(arg) {
		return f
	}
	return ""
}

// detect runs content detection on the file at path, after the decode of --encoding. A video container returns
// false after its head, without a read of the rest.
func (r *runner) detect(path string) (stbox.Format, bool) {
	content, ok := readUnlessContainer(path)
	if !ok {
		return "", false
	}
	if d, ok := stbox.Detect(content); ok && d.Format.IsBinary() {
		return d.Format, true
	}
	text, err := textenc.Decode(content, r.readOptions.Encoding)
	if err != nil {
		return "", false
	}
	d, ok := stbox.Detect(text.Text)
	return d.Format, ok && d.Format != ""
}

// readUnlessContainer returns the content of the file at path. It returns false for a file that cannot be read and
// for a video container, which it reads only as far as its head.
func readUnlessContainer(path string) ([]byte, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	head := make([]byte, stbox.ContainerHeadSize)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, false
	}
	if d, ok := stbox.Detect(head[:n]); ok && d.Container != "" {
		return nil, false
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, false
	}
	content, err := io.ReadAll(file)
	return content, err == nil
}

// expand turns the input arguments into input files: "-", the subtitle files of a directory, the matches of a
// glob, or the path as given. It removes inputs whose real path came before.
func (r *runner) expand(args []string) ([]string, error) {
	var inputs []string
	for _, arg := range args {
		files, err := r.expandArg(arg)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, files...)
	}
	seen := make(map[string]bool, len(inputs))
	unique := inputs[:0]
	for _, in := range inputs {
		key := in
		if in != stdinArg {
			if realPath, err := filepath.EvalSymlinks(r.c.path(in)); err == nil {
				key, _ = filepath.Abs(realPath)
			}
		}
		if !seen[key] {
			seen[key] = true
			unique = append(unique, in)
		}
	}
	return unique, nil
}

func (r *runner) expandArg(arg string) ([]string, error) {
	if arg == stdinArg {
		return []string{arg}, nil
	}
	st, err := os.Stat(r.c.path(arg))
	switch {
	case err == nil && st.IsDir():
		files := r.directoryFiles(arg)
		if len(files) == 0 {
			return nil, usage("The directory " + arg + " holds no file with a known subtitle extension.")
		}
		return files, nil
	case err == nil || !strings.ContainsAny(arg, "*?["):
		return []string{arg}, nil
	default:
		return r.glob(arg), nil
	}
}

// directoryFiles returns the files of dir, sorted by name, that hold a readable format by their extension. A
// .txt file stays when content detection finds a format in it. A .sub file with an .idx file of the same name, in any
// letter case, is the second file of a VobSub pair. The .idx file is the input.
func (r *runner) directoryFiles(dir string) []string {
	entries, err := os.ReadDir(r.c.path(dir))
	if err != nil {
		return nil
	}
	base := strings.TrimRight(dir, `/\`)
	var files []string
	for _, e := range withoutVobSubData(entries) {
		path := base + "/" + e.Name()
		if st, err := os.Stat(r.c.path(path)); err != nil || !st.Mode().IsRegular() {
			continue
		}
		f, ok := stbox.FormatFromPath(e.Name())
		if ok && f.CanRead() {
			files = append(files, path)
			continue
		}
		if strings.ToLower(filepath.Ext(e.Name())) == ".txt" {
			if _, found := r.detect(r.c.path(path)); found {
				files = append(files, path)
			}
		}
	}
	return files
}

// glob returns the regular files that match pattern, relative to the directory of the call again. Without a
// match, the pattern stays as the input, and the read fails later as a missing file.
func (r *runner) glob(pattern string) []string {
	matches, _ := filepath.Glob(r.c.path(pattern))
	var files []string
	for _, m := range matches {
		if st, err := os.Stat(m); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if r.c.dir != "" && !filepath.IsAbs(pattern) {
			if rel, err := filepath.Rel(r.c.dir, m); err == nil {
				m = rel
			}
		}
		files = append(files, m)
	}
	if len(files) == 0 {
		return []string{pattern}
	}
	return files
}

// vobSubPartners returns the other file of the pair of each VobSub input that exists. A VobSub read opens both files,
// so no output may overwrite either.
func (r *runner) vobSubPartners(inputs []string) []string {
	var partners []string
	for _, arg := range inputs {
		if r.peek(arg) != stbox.VobSub {
			continue
		}
		candidates, _ := extfallback.Partners(arg)
		for _, name := range candidates {
			if st, err := os.Stat(r.c.path(name)); err == nil && st.Mode().IsRegular() {
				partners = append(partners, name)
				break
			}
		}
	}
	return partners
}

// withoutVobSubData returns entries without the .sub files that have an .idx file of the same name in any letter case.
func withoutVobSubData(entries []os.DirEntry) []os.DirEntry {
	indexes := make(map[string]bool)
	for _, e := range entries {
		if strings.EqualFold(filepath.Ext(e.Name()), ".idx") {
			indexes[lowerStem(e.Name())] = true
		}
	}
	return slices.DeleteFunc(slices.Clone(entries), func(e os.DirEntry) bool {
		return strings.EqualFold(filepath.Ext(e.Name()), ".sub") && indexes[lowerStem(e.Name())]
	})
}

func lowerStem(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
}
