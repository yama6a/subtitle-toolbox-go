package stbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
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
		return nil, ReadInfo{}, unsupported(subject + " is a " + c + " file. This version reads no video files.")
	}
	var row formatFuncs
	if opts.Format != "" {
		var err error
		if row, err = readable(opts.Format); err != nil {
			return nil, ReadInfo{}, err
		}
	}
	text, err := textenc.Decode(content, opts.Encoding)
	if err != nil {
		return nil, ReadInfo{}, &ParseError{Format: opts.Format, Msg: err.Error(), Err: err}
	}
	if opts.Format == "" {
		f, empty, err := formatOf(text.Text, path)
		if err != nil {
			return nil, ReadInfo{}, err
		}
		if empty {
			return &Subtitle{}, readInfo(f, text, nil, opts), nil
		}
		if row, err = readable(f); err != nil {
			return nil, ReadInfo{}, err
		}
	}
	return read(row, text, opts)
}

// formatOf returns the format of text by detection, else by the extension of path. empty reports that text is
// blank and that the format comes from the extension.
func formatOf(text []byte, path string) (f Format, empty bool, err error) {
	f, ok := formatOfExtension(path)
	if ok && isBlank(text) {
		return f, true, nil
	}
	if detected, found := Detect(text); found {
		return detected, false, nil
	}
	if !ok {
		return "", false, fmt.Errorf("%w: %s", ErrUnknownFormat, noFormat)
	}
	return f, false, nil
}

// container returns the name of the video container that the head of content shows.
func container(content []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(content, []byte("\x1A\x45\xDF\xA3")):
		return "Matroska", true
	case len(content) >= 8 && string(content[4:8]) == "ftyp":
		return "MP4", true
	case len(content) > 376 && content[0] == 0x47 && content[188] == 0x47 && content[376] == 0x47:
		return "MPEG-TS", true
	default:
		return "", false
	}
}

// formatOfExtension returns the readable format of the extension of path. The extensions json, txt and ffmeta
// also belong to formats that detection never finds, such as cloud speech JSON and chapters, so they give none.
func formatOfExtension(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".txt", ".ffmeta":
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
// in one call, so an error of w leaves incomplete output on w.
func Write(w io.Writer, s *Subtitle, f Format, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	row, ok := lookup(f)
	switch {
	case !ok:
		return unknownFormat(f)
	case row.write != nil:
		return row.write(w, s, opts.common(), opts)
	case row.read != nil:
		return unsupported("The format " + string(f) + " can be read but not written.")
	default:
		return unsupported("The format " + string(f) + " cannot be written.")
	}
}

func unknownFormat(f Format) error {
	return unsupported("The format " + strconv.Quote(string(f)) + " is unknown.")
}

// unsupported keeps msg as the full sentence of the PHP library, as docs/errors.md says.
func unsupported(msg string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedFormat, msg)
}
