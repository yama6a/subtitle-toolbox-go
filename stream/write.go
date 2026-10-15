package stream

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/subrip"
	"github.com/yama6a/subtitle-toolbox-go/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var errClosed = &model.InvalidArgumentError{Msg: "The writer is closed."}

// sink writes the parts of one output and remembers whether the caller closed the writer.
type sink struct {
	w      io.Writer
	f      model.Format
	common model.CommonWriteOptions
	buf    []byte
	closed bool
}

// prefix writes the first part, which gets the BOM.
func (s *sink) prefix(part string) error {
	return s.emit(writing.Finish([]byte(part), s.common, true))
}

// next writes a later part.
func (s *sink) next(part []byte) error {
	return s.emit(writing.LineEnding(part, s.common))
}

func (s *sink) emit(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if _, err := s.w.Write(b); err != nil {
		return fmt.Errorf("write %s: %w", s.f, err)
	}
	return nil
}

// SubRipWriter writes SubRip cue blocks one at a time.
type SubRipWriter struct {
	sink
	n int
}

// NewSubRipWriter returns a writer that writes the BOM to w at once, if the options ask for it.
// The first return value is nil when the options are invalid.
func NewSubRipWriter(w io.Writer, common model.CommonWriteOptions, _ srt.WriteOptions) (*SubRipWriter, error) {
	if err := validateCommon(common); err != nil {
		return nil, err
	}
	sw := &SubRipWriter{sink: sink{w: w, f: model.SubRip, common: common}}
	if err := sw.prefix(""); err != nil {
		return nil, err
	}
	return sw, nil
}

// Write writes c as the next block. The cue numbers start at 1.
// An error of the io.Writer returns wrapped, and the writer stays usable.
func (w *SubRipWriter) Write(c model.Cue) error {
	if w.closed {
		return errClosed
	}
	w.buf = w.buf[:0]
	if w.n > 0 {
		w.buf = append(w.buf, '\n')
	}
	w.n++
	w.buf = subrip.AppendCue(w.buf, w.n, c, w.common.StripTags)
	return w.next(w.buf)
}

// Close marks the writer closed. It does not close the io.Writer, which the caller owns.
func (w *SubRipWriter) Close() error {
	w.closed = true
	return nil
}

// WebVTTWriter writes WebVTT cue blocks one at a time.
type WebVTTWriter struct {
	sink
	cw        vtt.Writer
	n         int
	hasBlocks bool
	dropped   []webvtt.DroppedColor
}

// NewWebVTTWriter returns a writer that writes the BOM and the header to w at once.
// header holds the header text, header lines, STYLE and REGION blocks, as WebVTTReader.Header gives them.
// Nil writes the signature line only. The first return value is nil when the options are invalid.
func NewWebVTTWriter(w io.Writer, common model.CommonWriteOptions, _ webvtt.WriteOptions, header *model.Data) (*WebVTTWriter, error) {
	if err := validateCommon(common); err != nil {
		return nil, err
	}
	ww := &WebVTTWriter{sink: sink{w: w, f: model.WebVTT, common: common}}
	ww.cw = vtt.Writer{
		StripTags: common.StripTags,
		OnDroppedColor: func(cueIndex int, color, message string) {
			ww.dropped = append(ww.dropped, webvtt.DroppedColor{CueIndex: cueIndex, Color: color, Message: message})
		},
	}
	prefix := vtt.Header(header) + "\n\n"
	if blocks := vtt.FileBlocks(header); len(blocks) > 0 {
		prefix += strings.Join(blocks, "\n\n") + "\n"
		ww.hasBlocks = true
	}
	if err := ww.prefix(prefix); err != nil {
		return nil, err
	}
	return ww, nil
}

// Write writes c as the next block.
// An error of the io.Writer returns wrapped, and the writer stays usable.
func (w *WebVTTWriter) Write(c model.Cue) error {
	if w.closed {
		return errClosed
	}
	w.buf = w.buf[:0]
	if w.hasBlocks {
		w.buf = append(w.buf, '\n')
	}
	w.hasBlocks = true
	w.buf = append(w.buf, w.cw.CueBlock(c, w.n)...)
	w.buf = append(w.buf, '\n')
	w.n++
	return w.next(w.buf)
}

// DroppedColors returns the font colors that no WebVTT color class has, so the output dropped them,
// for the cues written so far.
func (w *WebVTTWriter) DroppedColors() []webvtt.DroppedColor {
	return slices.Clone(w.dropped)
}

// Close marks the writer closed. It does not close the io.Writer, which the caller owns.
func (w *WebVTTWriter) Close() error {
	w.closed = true
	return nil
}
