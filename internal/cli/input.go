package cli

import (
	"cmp"
	"slices"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/scc"
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
			"When detection finds no format, the file extension sets it. Chapters and cloud speech-to-text JSON need --from."),
		valueOption("encoding", "NAME", "Encoding of input that is not UTF-8, for example Windows-1252. "+
			"It replaces code page detection. A BOM, valid UTF-8 and UTF-16 input win over it."),
		flagOption("lenient", "Skip or repair broken cues and print a warning for each. Chapter input ignores it."),
		valueOption("input-fps", "RATE", "Frame rate of a MicroDVD input without a {1}{1}<fps> first line, "+
			"of Spruce STL timecodes, and of CSV or TSV times in hh:mm:ss:ff."),
		valueOption("fps", "RATE", fpsHelp),
		flagOption("word-timestamps", "Keep the word times of speech-to-text JSON, YouTube timed text and Podcasting 2.0 transcript input."),
		valueOption("scc-roll-up", "MODE", "How SCC, MCC and MP4 c608 input reads roll-up captions: screen gives one cue per "+
			"screen, lines gives one cue per row. Default: "+rollUps[0]+"."),
		valueOption("track", "NUMBER", "Subtitle track of an MKV, WebM, MP4 or MPEG-TS input. "+
			`Needed when the file has 2 or more subtitle tracks. "info" lists them.`),
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
	// container and tracks are set when info lists the tracks of a video file, and sub is nil then.
	container stbox.Container
	tracks    []stbox.Track
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

// prepareRead builds the read options of --input-fps, --word-timestamps, --scc-roll-up, --track, --from, --encoding and --lenient and
// checks them. A command that renames --from or --track sets fromOption or trackOption first.
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
	if n, ok, err := a.PositiveInt(cmp.Or(r.trackOption, "track")); err != nil {
		return err
	} else if ok {
		r.readOptions.Track = int(n)
	}
	if v, ok := a.Value(cmp.Or(r.fromOption, "from")); ok {
		f, err := readableFormat(v)
		if err != nil {
			return err
		}
		r.readOptions.Format = f
	}
	words := a.Has("word-timestamps") || slices.ContainsFunc(r.wordTriggers, a.Has)
	r.readOptions.Transcript.WordTimestamps = words
	r.readOptions.YouTube.WordTimestamps = words
	r.readOptions.Encoding, _ = a.Value("encoding")
	r.readOptions.Lenient = a.Has("lenient")
	return r.readOptions.Validate()
}
