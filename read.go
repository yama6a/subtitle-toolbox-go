package stbox

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// Load reads the subtitle file at path. On error the subtitle is nil and ReadInfo is the zero value.
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
	f := opts.Format
	if f == "" {
		var ok bool
		if f, ok = Detect(content); !ok {
			return nil, ReadInfo{}, ErrUnknownFormat
		}
	}
	row, ok := lookup(f)
	if !ok {
		return nil, ReadInfo{}, fmt.Errorf("%w: %q is no format", ErrUnsupportedFormat, f)
	}
	if row.read == nil {
		return nil, ReadInfo{}, fmt.Errorf("%w: %s cannot be read", ErrUnsupportedFormat, f)
	}
	s, warnings, err := row.read(bytes.NewReader(content), opts.common(), opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return s, ReadInfo{Format: f, Warnings: warnings}, nil
}

// Write writes s to w in format f.
func Write(w io.Writer, s *Subtitle, f Format, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	row, ok := lookup(f)
	if !ok {
		return fmt.Errorf("%w: %q is no format", ErrUnsupportedFormat, f)
	}
	if row.write == nil {
		return fmt.Errorf("%w: %s cannot be written", ErrUnsupportedFormat, f)
	}
	return row.write(w, s, opts.common(), opts)
}
