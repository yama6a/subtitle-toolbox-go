package cli

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
)

// read reads one input with the read options of the call and prints its warnings. info lists the tracks of a
// video file in place of a read.
func (r *runner) read(arg string) (*input, error) {
	return r.readWith(arg, r.readOptions, "--"+cmp.Or(r.trackOption, "track"))
}

// source is an input that readWith has opened: the path of a file, or the bytes of standard input.
type source struct {
	// path is the file path, "" for standard input.
	path string
	// content holds the bytes of standard input.
	content []byte
	// head holds the first bytes, enough to detect a video container.
	head []byte
}

// open finds the input arg. A file is read only as far as its head, and standard input is read whole.
func (r *runner) open(arg string, opts stbox.ReadOptions) (source, error) {
	if arg == stdinArg {
		if opts.Format == stbox.VobSub {
			return source{}, usage(vobSubStdinMessage)
		}
		content, err := io.ReadAll(r.c.stdin)
		if err != nil {
			return source{}, fmt.Errorf("read standard input: %w", err)
		}
		return source{content: content, head: content[:min(len(content), stbox.ContainerHeadSize)]}, nil
	}
	path := r.c.path(arg)
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return source{}, usage(missingFileMessage)
	}
	head, err := readFileHead(path)
	return source{path: path, head: head}, err
}

// listInput returns the track list of a video file for info, when the call lists tracks, names no track, and the
// extension of arg names no subtitle format. A file that cannot be parsed is not listed, so the normal read
// reports the failure.
func (r *runner) listInput(arg string, src source, container stbox.Container, track int) (in *input, listed bool, err error) {
	if !r.listTracks || container == "" || track != 0 || !namesNoSubtitle(arg) {
		return nil, false, nil
	}
	tracks, err := tracksOf(src.path, src.content)
	switch {
	case err == nil:
		return &input{arg: arg, container: container, tracks: tracks}, true, nil
	case errors.Is(err, stbox.ErrParse):
		return nil, false, nil
	default:
		return nil, true, err
	}
}

// readWith reads one input with opts and prints its warnings. trackOption names the track option of the input,
// with its dashes, for the messages of a video file.
func (r *runner) readWith(arg string, opts stbox.ReadOptions, trackOption string) (*input, error) {
	src, err := r.open(arg, opts)
	if err != nil {
		return nil, err
	}
	container := containerOfHead(src.head)
	if in, listed, err := r.listInput(arg, src, container, opts.Track); listed {
		return in, err
	}
	in := &input{arg: arg}
	if opts, err = trackOptions(container, opts, trackOption); err != nil {
		return nil, err
	}
	if src.path == "" {
		in.sub, in.info, err = stbox.Parse(bytes.NewReader(src.content), int64(len(src.content)), opts)
	} else {
		in.sub, in.info, err = stbox.Load(src.path, opts)
	}
	if err != nil {
		return nil, trackFailure(err, trackOption)
	}
	r.printWarnings(in)
	return in, nil
}

// trackOptions adjusts opts for an input in a video container. A track read has no format, because the track
// decides it. A read of a container with a format but no track fails.
func trackOptions(container stbox.Container, opts stbox.ReadOptions, trackOption string) (stbox.ReadOptions, error) {
	switch {
	case container == "":
	case opts.Track != 0:
		opts.Format = ""
	case opts.Format != "":
		return opts, usage("The input is an " + container.Label() + " file. Pass " + trackOption + " N.")
	}
	return opts, nil
}

// trackFailure turns a *stbox.TrackError into a usage error with the track listing. Another error stays as it is.
func trackFailure(err error, trackOption string) error {
	if e, ok := errors.AsType[*stbox.TrackError](err); ok {
		return usage(trackErrorMessage(e, trackOption))
	}
	return err
}

// namesNoSubtitle reports whether the extension of arg names no subtitle format, as for "-" and "movie.mkv".
func namesNoSubtitle(arg string) bool {
	_, ok := stbox.FormatFromPath(arg)
	return !ok
}

// tracksOf lists the tracks of the video file at path, or of content when path is empty.
func tracksOf(path string, content []byte) ([]stbox.Track, error) {
	if path == "" {
		return stbox.Tracks(bytes.NewReader(content), int64(len(content)))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err //nolint:wrapcheck // the message names no Go call
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return nil, err //nolint:wrapcheck // the message names no Go call
	}
	return stbox.Tracks(file, st.Size())
}

// readFileHead returns the first bytes of the file at path that container detection needs.
func readFileHead(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err //nolint:wrapcheck // the message names no Go call
	}
	defer file.Close()
	head := make([]byte, stbox.ContainerHeadSize)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err //nolint:wrapcheck // the message names no Go call
	}
	return head[:n], nil
}

// containerOfHead returns the video container that the first bytes of a file show, "" for a subtitle file.
func containerOfHead(head []byte) stbox.Container {
	d, ok := stbox.Detect(head)
	if !ok {
		return ""
	}
	return d.Container
}

// readOther reads a file other than the input of the call, such as the new file of diff, and prints its warnings.
// A failure carries the path as its label. A file with no detectable format gets the advice for fromOption, and a
// video file with many tracks gets the advice for trackOption.
func (r *runner) readOther(path string, opts stbox.ReadOptions, fromOption, trackOption string) (*input, error) {
	in, err := r.readWith(path, opts, "--"+trackOption)
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

// peek returns the format that the read of arg will pick, without a message: for a video container the format of the
// track, else --from, else the detected format, else the extension fallback of docs/detection.md. It returns "" for
// standard input and on any error.
func (r *runner) peek(arg string) stbox.Format {
	path := r.c.path(arg)
	if st, err := os.Stat(path); arg == stdinArg || err != nil || !st.Mode().IsRegular() {
		return ""
	}
	if f, ok := r.trackFormat(path); ok {
		return f
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

// isVideoFile reports whether the file at path is a video container.
func (r *runner) isVideoFile(path string) bool {
	head, err := readFileHead(r.c.path(path))
	return err == nil && containerOfHead(head) != ""
}

// trackFormat returns the format of the track that --track names in the video file at path, or of its only track.
// The second result is false for a file that is no video container.
func (r *runner) trackFormat(path string) (stbox.Format, bool) {
	head, err := readFileHead(path)
	if err != nil || containerOfHead(head) == "" {
		return "", false
	}
	tracks, err := tracksOf(path, nil)
	if err != nil {
		return "", true
	}
	for _, t := range tracks {
		if t.Number == r.readOptions.Track || r.readOptions.Track == 0 && len(tracks) == 1 {
			return t.Format, true
		}
	}
	return "", true
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
	head, err := readFileHead(path)
	if err != nil || containerOfHead(head) != "" {
		return nil, false
	}
	content, err := os.ReadFile(path)
	return content, err == nil
}
