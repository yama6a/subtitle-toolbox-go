package writing

import (
	"fmt"
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// ClosedError returns the error of a stream writer that the caller closed.
func ClosedError() *model.InvalidArgumentError {
	return &model.InvalidArgumentError{Msg: "the writer is closed"}
}

// Sink writes the parts of one output and remembers whether the caller closed the writer.
type Sink struct {
	w      io.Writer
	f      model.Format
	common model.CommonWriteOptions
	closed bool
	cues   int // the cues passed to Write, also the dropped ones
}

// NewSink returns a sink that writes the parts of a file of format f to w.
func NewSink(w io.Writer, f model.Format, common model.CommonWriteOptions) *Sink {
	return &Sink{w: w, f: f, common: common}
}

// Close marks the sink closed.
func (s *Sink) Close() { s.closed = true }

// Closed reports whether Close ran.
func (s *Sink) Closed() bool { return s.closed }

// Gate counts c and reports whether to drop it, because it holds an image and no lines and common.SkipImageCues is
// set. Without SkipImageCues it returns the error of a text writer for such a cue.
func (s *Sink) Gate(c model.Cue) (drop bool, err error) {
	i := s.cues
	s.cues++
	switch {
	case !c.IsImageWithoutText():
		return false, nil
	case s.common.SkipImageCues:
		return true, nil
	}
	return false, ImageCueError(s.f, i)
}

// Prefix writes the first part, which gets the BOM.
func (s *Sink) Prefix(part string) error {
	return s.Emit(Finish([]byte(part), s.common, true))
}

// Next writes a later part.
func (s *Sink) Next(part []byte) error {
	return s.Emit(LineEnding(part, s.common))
}

// Emit writes b to the io.Writer and wraps its error.
func (s *Sink) Emit(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if _, err := s.w.Write(b); err != nil {
		return fmt.Errorf("write %s: %w", s.f, err)
	}
	return nil
}
