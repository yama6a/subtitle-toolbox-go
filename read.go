package stbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

// Load reads the subtitle file at path. On error the subtitle is nil and ReadInfo is the zero value.
// A file that does not exist or cannot be read returns the *fs.PathError of package os, wrapped.
func Load(path string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if err := opts.Validate(); err != nil {
		return nil, ReadInfo{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, ReadInfo{}, fmt.Errorf("load subtitle: %w", err)
	}
	return parse(content, opts)
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
	return parse(content, opts)
}

func parse(content []byte, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	var row formatFuncs
	if opts.Format != "" {
		var err error
		if row, err = readable(opts.Format); err != nil {
			return nil, ReadInfo{}, err
		}
	}
	text, err := textenc.Decode(content, opts.Encoding)
	if err != nil {
		return nil, ReadInfo{}, &ParseError{Format: opts.Format, Msg: err.Error()}
	}
	if opts.Format == "" {
		f, ok := Detect(text.Text)
		if !ok {
			return nil, ReadInfo{}, ErrUnknownFormat
		}
		if row, err = readable(f); err != nil {
			return nil, ReadInfo{}, err
		}
	}
	return read(row, text, opts)
}

func read(row formatFuncs, text textenc.Decoded, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	s, warnings, err := row.read(bytes.NewReader(text.Text), opts.common(), opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	if text.Warning != "" && opts.Lenient {
		warnings = slices.Insert(warnings, 0, decodeWarning(text.Warning))
	}
	return s, ReadInfo{Format: row.format, SourceEncoding: text.Encoding, Warnings: warnings}, nil
}

// decodeWarning returns a Repaired warning without a line or a block, once model.ParseWarning has the fields of
// https://github.com/yama6a/subtitle-toolbox-go/issues/17.
func decodeWarning(string) ParseWarning {
	return ParseWarning{}
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
