// Package stboxjson reads and writes library JSON, the lossless JSON form of a subtitle.
//
// A library JSON document holds every field of the model: metadata, comments, format data and cues.
// So a trip through library JSON keeps what the source format stored, such as the styles of an ASS file.
package stboxjson

import (
	"fmt"
	"io"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Version is the library JSON version that this package reads and writes.
const Version = 1

// ReadOptions holds the library JSON read settings. Library JSON has none.
type ReadOptions struct{}

// WriteOptions holds the library JSON write settings.
type WriteOptions struct {
	// PrettyPrint indents with 4 spaces and ends the output with LF. False writes one compact line without a final LF.
	PrettyPrint bool
	// OmitFormatData leaves out formatData on the subtitle and on every cue. False writes it at both levels.
	OmitFormatData bool
}

// Read reads a library JSON document of version 1 from r. A field error names the path of the field, such as cues[3].start.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.JSON, err)
	}
	st := parsing.NewState(model.JSON, common)
	// The JSON decoder reads each bad byte as U+FFFD in both modes, so the UTF-8 check only warns. True would fail strict reads.
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	rd := &reader{st: st}
	s, err := rd.read(content)
	if err != nil {
		return nil, nil, err
	}
	return s, sortWarnings(st.Warnings()), nil
}

// Write writes s to w as a library JSON document of version 1. Negative times stay as they are, and StripTags has no effect.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	body, err := document(s, opts)
	if err != nil {
		return err
	}
	return writing.Write(w, model.JSON, body, common, false)
}
