package stbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/extfallback"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/textenc"
)

// Load reads the subtitle file at path. On error the subtitle is nil and ReadInfo is the zero value.
// Without ReadOptions.Format, Load detects the format and falls back to the file extension, as docs/detection.md says.
// A file that does not exist or cannot be read returns the *fs.PathError of package os, wrapped.
func Load(path string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if err := opts.Validate(); err != nil {
		return nil, ReadInfo{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("load subtitle: %w", err)
	}
	return parse(content, path, opts)
}

// Parse reads a subtitle from r. On error the subtitle is nil and ReadInfo is the zero value.
// Parse has no file name, so CSV and TSV input needs ReadOptions.Format.
func Parse(r io.Reader, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if err := opts.Validate(); err != nil {
		return nil, ReadInfo{}, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("read subtitle: %w", err)
	}
	return parse(content, "", opts)
}

// noFormat is the PHP message of a read whose format detection found nothing.
const noFormat = "Format detection found no subtitle format."

// parse reads content. path is "" for Parse, which has no extension to fall back to.
func parse(content []byte, path string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if c, ok := container(content); ok {
		subject := "The content"
		if path != "" {
			subject = path
		}
		return nil, ReadInfo{}, unsupported(subject + " is " + c + " file. This version reads no video files.")
	}
	if opts.Format == "" {
		return parseDetected(content, path, opts)
	}
	row, err := readable(opts.Format)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	if opts.Format.IsBinary() {
		return read(row, textenc.Decoded{Text: content}, opts)
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
func parseDetected(content []byte, path string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
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
	return read(row, text, opts)
}

// binaryFormat returns the binary format of content that a read without a format picks before any decoding:
// the format that Detect finds in the raw bytes when it is binary. When Detect finds no format, the format of an
// extension that only binary formats list. A text format that Detect finds wins over the extension.
func binaryFormat(content []byte, path string) (Format, bool) {
	if f, ok := Detect(content); ok {
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
	if detected, found := Detect(text); found {
		return Format(extfallback.Profile(string(detected), path)), false, nil
	}
	if !ok {
		return "", false, fmt.Errorf("%w: %s", ErrUnknownFormat, noFormat)
	}
	return f, false, nil
}

// container returns the article and name of the video container that the head of content shows.
func container(content []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(content, []byte("\x1A\x45\xDF\xA3")):
		return "a Matroska", true
	case len(content) >= 8 && string(content[4:8]) == "ftyp":
		return "an MP4", true
	case transportStream(content, 188) || transportStream(content, 192):
		return "an MPEG-TS", true
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
