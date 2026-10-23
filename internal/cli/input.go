package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

const stdinArg = "-"

// The help texts of --fps. A command that writes subtitles gets writeFPSHelp, a command that reports gets reportFPSHelp.
const (
	writeFPSHelp  = "Sets --input-fps and --output-fps. A specific option wins over --fps."
	reportFPSHelp = "Same as --input-fps."
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
		valueOption("input-fps", "RATE", "Frame rate of a MicroDVD input without a {1}{1}<fps> first line, and of CSV or TSV times in hh:mm:ss:ff."),
		valueOption("fps", "RATE", fpsHelp),
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

// prepareRead builds the read options of --input-fps, --from, --encoding and --lenient and checks them.
func (r *runner) prepareRead(a *args) error {
	fps, ok, err := a.Rate("input-fps")
	if err != nil {
		return err
	}
	if ok {
		r.readOptions.MicroDVD.FrameRate = fps
		r.readOptions.CSV.FrameRate = fps
	}
	if v, ok := a.Value("from"); ok {
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
		var content []byte
		if content, err = io.ReadAll(r.c.stdin); err != nil {
			return nil, fmt.Errorf("read standard input: %w", err)
		}
		in.sub, in.info, err = stbox.Parse(bytes.NewReader(content), opts)
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
		return f
	}
	if f, ok := stbox.FormatFromPath(arg); ok && f.CanRead() && extfallback.Allowed(arg) {
		return f
	}
	return ""
}

// detect runs content detection on the file at path, after the decode of --encoding.
func (r *runner) detect(path string) (stbox.Format, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	text, err := textenc.Decode(content, r.readOptions.Encoding)
	if err != nil {
		return "", false
	}
	return stbox.Detect(text.Text)
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
// .txt file stays when content detection finds a format in it.
func (r *runner) directoryFiles(dir string) []string {
	entries, err := os.ReadDir(r.c.path(dir))
	if err != nil {
		return nil
	}
	base := strings.TrimRight(dir, `/\`)
	var files []string
	for _, e := range entries {
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
