package stbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
)

// Load reads the subtitle file at path. On error the subtitle is nil and ReadInfo is the zero value.
// Without ReadOptions.Format, Load detects the format and falls back to the file extension, as docs/detection.md says.
// A file that does not exist or cannot be read returns the *fs.PathError of package os, wrapped.
// Load reads a regular file through its handle, so a video file is not read whole. A pipe is read whole first.
// A VobSub read needs 2 files. Load finds the other one next to path, as LoadFS says.
func Load(path string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	return load(path, func(name string) (fs.File, error) { return os.Open(name) }, opts)
}

// LoadFS reads the subtitle file name from fsys, like Load. The name is an fs.ValidPath path.
// A VobSub read, with ReadOptions.Format VobSub or an .idx name, finds the other file of the pair in the same
// folder: an .idx name looks for .sub, then .SUB. Any other name looks for .idx, then .IDX. A name without an
// extension gets a dot first. A missing file returns an error that matches fs.ErrNotExist and names both files.
// LoadFS fills ReadOptions.VobSub.IDX with the .idx file, decoded with ReadOptions.Encoding. A MicroDVD .sub file
// never looks for an .idx file.
func LoadFS(fsys fs.FS, name string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	return load(name, fsys.Open, opts)
}

// openFunc opens a file by name, as os.Open or fs.FS.Open do.
type openFunc func(name string) (fs.File, error)

func load(path string, open openFunc, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if err := opts.Validate(); err != nil {
		return nil, ReadInfo{}, err
	}
	f, err := open(path)
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("load subtitle: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("load subtitle: %w", err)
	}
	if ra, ok := f.(io.ReaderAt); ok && st.Mode().IsRegular() {
		return parse(ra, st.Size(), path, open, opts)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("load subtitle: %w", err)
	}
	return parse(bytes.NewReader(content), int64(len(content)), path, open, opts)
}

// Parse reads a subtitle from the first size bytes of r. On error the subtitle is nil and ReadInfo is the zero value.
// Parse has no file name, so CSV and TSV input needs ReadOptions.Format.
// A video container reads only the bytes that its track needs. Every other format reads the whole input.
func Parse(r io.ReaderAt, size int64, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if err := opts.Validate(); err != nil {
		return nil, ReadInfo{}, err
	}
	return parse(r, size, "", nil, opts)
}

// noFormat is the PHP message of a read whose format detection found nothing.
const noFormat = "Format detection found no subtitle format."

// parse reads the input. path is "" for Parse, which has no extension to fall back to. open is nil for Parse, which
// finds no second file of a VobSub pair.
func parse(r io.ReaderAt, size int64, path string, open openFunc, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	head, err := readHead(r, size)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	if c, ok := containerOf(head); ok {
		subject := "The content"
		if path != "" {
			subject = path
		}
		if row, ok := containerRow(c); ok {
			return readTrack(row, r, size, subject, opts)
		}
	}
	if opts.Track != 0 {
		return nil, ReadInfo{}, &InvalidArgumentError{
			Option: "ReadOptions.Track",
			Msg:    "Only MKV, WebM, MP4 and MPEG-TS files have tracks.",
		}
	}
	content := head
	if size > ContainerHeadSize {
		if content, err = io.ReadAll(io.NewSectionReader(r, 0, size)); err != nil {
			return nil, ReadInfo{}, fmt.Errorf("read subtitle: %w", err)
		}
	}
	return parseContent(content, path, open, opts)
}

// readHead returns the first ContainerHeadSize bytes of the input, or all of it when it is shorter. It returns fewer
// bytes than size when r holds less data than size says.
func readHead(r io.ReaderAt, size int64) ([]byte, error) {
	if size < 0 {
		return nil, &InvalidArgumentError{Msg: fmt.Sprintf("The size must be 0 or more, got %d.", size)}
	}
	head := make([]byte, min(size, ContainerHeadSize))
	if len(head) == 0 {
		return head, nil
	}
	n, err := r.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read subtitle: %w", err)
	}
	return head[:n], nil
}

// parseContent reads the whole content of a subtitle file.
func parseContent(content []byte, path string, open openFunc, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if opts.Format == "" {
		return parseDetected(content, path, open, opts)
	}
	row, err := readable(opts.Format)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	if opts.Format.IsBinary() {
		return readBinary(row, content, path, open, opts)
	}
	text, err := decode(content, opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return read(row, text, opts)
}

func decode(content []byte, opts ReadOptions) (textenc.Decoded, error) {
	text, err := textenc.Decode(content, opts.Encoding)
	if err != nil {
		return textenc.Decoded{}, &ParseError{Format: opts.Format, Msg: err.Error(), Err: err}
	}
	return text, nil
}

// parseDetected reads content of an unknown format: a binary format by content or extension first, else the
// decoded text by detection and extension, as docs/detection.md says.
func parseDetected(content []byte, path string, open openFunc, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	raw := textenc.Decoded{Text: content}
	f, binary := binaryFormat(content, path)
	var text textenc.Decoded
	var empty bool
	if binary {
		text, empty = raw, isBlank(content)
	} else {
		var err error
		if text, err = decode(content, opts); err != nil {
			return nil, ReadInfo{}, err
		}
		if f, empty, err = formatOf(text.Text, path); err != nil {
			return nil, ReadInfo{}, err
		}
		if f.IsBinary() {
			text = raw
		}
	}
	if empty {
		return &Subtitle{}, readInfo(f, text, nil, opts), nil
	}
	row, err := readable(f)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	if f.IsBinary() {
		return readBinary(row, content, path, open, opts)
	}
	return read(row, text, opts)
}

// readBinary reads the raw bytes of a binary format. VobSub reads 2 files, so it first finds the other one.
func readBinary(row formatFuncs, content []byte, path string, open openFunc, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	text := textenc.Decoded{Text: content}
	if row.format == VobSub && open != nil {
		var err error
		if text, opts, err = pairedVobSub(content, path, open, opts); err != nil {
			return nil, ReadInfo{}, err
		}
	}
	return read(row, text, opts)
}

// pairedVobSub returns the .sub content, and opts with the .idx content, of the VobSub pair of path. content is the
// content of path. The .idx decodes as text with opts.Encoding. The decode warning of a guess goes to the read as
// the decode warning of a text format does.
func pairedVobSub(content []byte, path string, open openFunc, opts ReadOptions) (textenc.Decoded, ReadOptions, error) {
	candidates, isIndex := extfallback.Partners(path)
	partner, err := readPartner(candidates, path, open)
	if err != nil {
		return textenc.Decoded{}, opts, err
	}
	idx, sub := partner, content
	if isIndex {
		idx, sub = content, partner
	}
	text, err := textenc.Decode(idx, opts.Encoding)
	if err != nil {
		return textenc.Decoded{}, opts, &ParseError{Format: VobSub, Msg: err.Error(), Err: err}
	}
	opts.VobSub.IDX = string(text.Text)
	return textenc.Decoded{Text: sub, Warning: text.Warning}, opts, nil
}

// readPartner reads the first candidate that is a file. Without one, it returns an error that matches fs.ErrNotExist
// and names path and the first candidate.
func readPartner(candidates []string, path string, open openFunc) ([]byte, error) {
	for _, name := range candidates {
		f, err := open(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("load subtitle: %w", err)
		}
		content, isDir, err := readFile(f)
		if err != nil {
			return nil, err
		}
		if !isDir {
			return content, nil
		}
	}
	return nil, fmt.Errorf("load subtitle: VobSub needs the file %s next to %s: %w", candidates[0], path, fs.ErrNotExist)
}

// readFile reads and closes f. It reads nothing from a directory.
func readFile(f fs.File) (content []byte, isDir bool, err error) {
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("load subtitle: %w", err)
	}
	if st.IsDir() {
		return nil, true, nil
	}
	content, err = io.ReadAll(f)
	if err != nil {
		return nil, false, fmt.Errorf("load subtitle: %w", err)
	}
	return content, false, nil
}

// binaryFormat returns the binary format of content that a read without a format picks before any decoding:
// the format that Detect finds in the raw bytes when it is binary. When Detect finds no format, the format of an
// extension that only binary formats list. A text format that Detect finds wins over the extension.
func binaryFormat(content []byte, path string) (Format, bool) {
	if f, ok := detectFormat(content); ok {
		return f, f.IsBinary()
	}
	if !binaryExtension(path) {
		return "", false
	}
	return formatOfExtension(path)
}

// binaryExtension reports whether every format that lists the extension of path is binary. ".890" is.
// ".stl" is not one, because Spruce STL lists it too. A .stl file with no binary signature goes through the text path.
func binaryExtension(path string) bool {
	ext := extension(path)
	found := false
	for _, row := range registry {
		if slices.Contains(row.format.Extensions(), ext) {
			if !row.format.IsBinary() {
				return false
			}
			found = true
		}
	}
	return found
}

// formatOf returns the format of text by detection, else by the extension of path. empty reports that text is
// blank and that the format comes from the extension.
func formatOf(text []byte, path string) (f Format, empty bool, err error) {
	f, ok := formatOfExtension(path)
	if ok && isBlank(text) {
		return f, true, nil
	}
	if detected, found := detectFormat(text); found {
		return Format(extfallback.Profile(string(detected), path)), false, nil
	}
	if !ok {
		return "", false, fmt.Errorf("%w: %s", ErrUnknownFormat, noFormat)
	}
	return f, false, nil
}

// containerOf returns the video container that the head of a file shows. The head is the raw bytes, before any
// BOM trim.
func containerOf(head []byte) (Container, bool) {
	switch {
	case bytes.HasPrefix(head, []byte("\x1A\x45\xDF\xA3")):
		return Matroska, true
	case len(head) >= 8 && string(head[4:8]) == "ftyp":
		return MP4, true
	case transportStream(head, 188) || transportStream(head, 192):
		return MPEGTS, true
	default:
		return "", false
	}
}

// transportStream reports whether content starts with 3 MPEG-TS packets of size bytes. An M2TS packet of 192
// bytes puts a 4-byte time stamp before the sync byte 0x47.
func transportStream(content []byte, size int) bool {
	sync := size - 188
	return len(content) > 2*size+sync && content[sync] == 0x47 && content[size+sync] == 0x47 &&
		content[2*size+sync] == 0x47
}

// formatOfExtension returns the readable format of the extension of path, when the extension may set it.
func formatOfExtension(path string) (Format, bool) {
	if !extfallback.Allowed(path) {
		return "", false
	}
	f, ok := FormatFromPath(path)
	return f, ok && f.CanRead()
}

func read(row formatFuncs, text textenc.Decoded, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	s, warnings, err := row.read(bytes.NewReader(text.Text), opts.common(), opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return s, readInfo(row.format, text, warnings, opts), nil
}

func readInfo(f Format, text textenc.Decoded, warnings []ParseWarning, opts ReadOptions) ReadInfo {
	if text.Warning != "" && opts.Lenient {
		warnings = slices.Insert(warnings, 0, decodeWarning(text.Warning))
	}
	return ReadInfo{Format: f, SourceEncoding: text.Encoding, Warnings: warnings}
}

func decodeWarning(msg string) ParseWarning {
	return ParseWarning{Message: msg, Line: NoLine, Block: NoBlock, Action: Repaired}
}

func readable(f Format) (formatFuncs, error) {
	row, ok := lookup(f)
	switch {
	case !ok:
		return formatFuncs{}, unknownFormat(f)
	case row.read != nil:
		return row, nil
	case row.write != nil:
		return formatFuncs{}, unsupported("The format " + string(f) + " can be written but not read.")
	default:
		return formatFuncs{}, unsupported("The format " + string(f) + " cannot be read.")
	}
}

// Write writes s to w in format f. The format package builds the whole output first and writes it
// in one call, so an error of w leaves incomplete output on w. LineEnding and BOM do not apply to a binary format.
func Write(w io.Writer, s *Subtitle, f Format, opts WriteOptions) (WriteReport, error) {
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	row, ok := lookup(f)
	switch {
	case !ok:
		return WriteReport{}, unknownFormat(f)
	case row.write != nil:
		return row.write(w, s, opts.common(), opts)
	case row.read != nil:
		return WriteReport{}, unsupported("The format " + string(f) + " can be read but not written.")
	default:
		return WriteReport{}, unsupported("The format " + string(f) + " cannot be written.")
	}
}

func unknownFormat(f Format) error {
	return unsupported("The format " + strconv.Quote(string(f)) + " is unknown.")
}

// unsupported keeps msg as the full sentence of the PHP library, as docs/errors.md says.
func unsupported(msg string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedFormat, msg)
}
