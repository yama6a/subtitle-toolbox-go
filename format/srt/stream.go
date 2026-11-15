package srt

import (
	"io"
	"iter"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// StreamOptions holds the settings of a StreamReader.
type StreamOptions struct {
	// Lenient skips or repairs broken blocks and reports each as a warning. False stops at the first broken block.
	Lenient bool
}

func (o StreamOptions) common() model.CommonReadOptions {
	return model.CommonReadOptions{Lenient: o.Lenient}
}

// StreamReader reads the cues of a SubRip input one block at a time, in constant memory.
// It gives the cues and the warnings of Read, in file order.
// It reads UTF-8. Wrap the input with golang.org/x/text/transform for other encodings.
type StreamReader struct {
	st    *parsing.State
	lines *parsing.LineReader
	rd    *blockReader
	used  bool
}

// NewStreamReader returns a reader of the SubRip text in r. It reads nothing until Cues runs.
func NewStreamReader(r io.Reader, opts StreamOptions) *StreamReader {
	st := parsing.NewState(model.SubRip, opts.common())
	lines := st.NewLineReader(r)
	rd := newBlockReader(st)
	rd.CutAt(lines.CutLine)
	return &StreamReader{st: st, lines: lines, rd: rd}
}

// Cues reads the input once and yields its cues in file order, without a sort by start.
// On an error it yields a zero cue and the error, and stops. A second call yields nothing.
// Breaking out of the loop stops the read.
func (r *StreamReader) Cues() iter.Seq2[model.Cue, error] {
	return func(yield func(model.Cue, error) bool) {
		if r.used {
			return
		}
		r.used = true
		index := 0
		for b := range r.rd.Blocks(r.lines.Lines()) {
			cues, err := r.rd.ParseBlock(b, index)
			index++
			if err != nil {
				yield(model.Cue{}, r.lines.ErrOr(err))
				return
			}
			for _, c := range cues {
				if !yield(c, nil) {
					return
				}
			}
		}
		if err := r.lines.Err(); err != nil {
			yield(model.Cue{}, err)
		}
	}
}

// Warnings returns the warnings of the blocks read so far, in the order of Read.
func (r *StreamReader) Warnings() []model.ParseWarning {
	return r.st.Warnings()
}

// StreamWriter writes SubRip cue blocks one at a time.
// It writes the bytes of Write.
type StreamWriter struct {
	sink      *writing.Sink
	stripTags bool
	buf       []byte
	n         int
}

// NewStreamWriter returns a writer that writes the BOM to w at once, if the options ask for it.
// The first return value is nil when the options are invalid.
func NewStreamWriter(w io.Writer, common model.CommonWriteOptions, _ WriteOptions) (*StreamWriter, error) {
	if err := common.Validate(); err != nil {
		return nil, err
	}
	sw := &StreamWriter{sink: writing.NewSink(w, model.SubRip, common), stripTags: common.StripTags}
	if err := sw.sink.Prefix(""); err != nil {
		return nil, err
	}
	return sw, nil
}

// Write writes c as the next block. The cue numbers start at 1.
// A cue with an image and no lines returns an *model.UnwritableError, or with SkipImageCues is dropped.
// An error of the io.Writer returns wrapped, and the writer stays usable.
func (w *StreamWriter) Write(c model.Cue) error {
	if w.sink.Closed() {
		return writing.ClosedError()
	}
	if drop, err := w.sink.Gate(c); drop || err != nil {
		return err
	}
	w.buf = w.buf[:0]
	if w.n > 0 {
		w.buf = append(w.buf, '\n')
	}
	w.n++
	w.buf = appendCue(w.buf, w.n, c, w.stripTags)
	return w.sink.Next(w.buf)
}

// Close marks the writer closed. It does not close the io.Writer, which the caller owns.
func (w *StreamWriter) Close() error {
	w.sink.Close()
	return nil
}
