package webvtt

import (
	"cmp"
	"io"
	"iter"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/internal/vtt"
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

// StreamReader reads the cues of a WebVTT input one block at a time, in constant memory.
// It gives the cues, the warnings and the header of Read, in file order.
// It reads UTF-8. Wrap the input with golang.org/x/text/transform for other encodings.
type StreamReader struct {
	st    *parsing.State
	lines *parsing.LineReader
	rd    *vtt.Reader
	used  bool
}

// NewStreamReader returns a reader of the WebVTT text in r. It reads nothing until Cues runs.
func NewStreamReader(r io.Reader, opts StreamOptions) *StreamReader {
	st := parsing.NewState(model.WebVTT, opts.common())
	st.ReplaceNUL = true
	lines := st.NewLineReader(r)
	rd := vtt.NewReader(st, false)
	rd.CutAt(lines.CutLine)
	return &StreamReader{st: st, lines: lines, rd: rd}
}

// Cues reads the input once and yields its cues in file order, without a sort by start.
// It skips NOTE blocks. On an error it yields a zero cue and the error, and stops.
// A second call yields nothing. Breaking out of the loop stops the read.
func (r *StreamReader) Cues() iter.Seq2[model.Cue, error] {
	return func(yield func(model.Cue, error) bool) {
		if r.used {
			return
		}
		r.used = true
		stopped := false
		err := r.run(func(c model.Cue) bool {
			stopped = !yield(c, nil)
			return !stopped
		})
		if err != nil && !stopped {
			yield(model.Cue{}, err)
		}
	}
}

// run passes the cues to emit until the input ends, a block fails or emit returns false.
// The cues that the collapse of rolling cues still holds come out before an error.
func (r *StreamReader) run(emit func(model.Cue) bool) error {
	var (
		rolling vtt.Rolling
		kept    []vtt.Kept
	)
	pass := func() bool {
		for _, k := range kept {
			vtt.ExtractWords(&k.Cue)
			if !emit(k.Cue) {
				return false
			}
		}
		return true
	}
	index := 0
	for b, err := range r.rd.Blocks(r.lines.Lines()) {
		var (
			c  model.Cue
			ok bool
		)
		if err == nil {
			c, ok, err = r.rd.ParseBlock(b, index)
			index++
		}
		if err != nil {
			kept = rolling.Flush(kept[:0])
			if !pass() {
				return nil
			}
			return r.lines.ErrOr(err)
		}
		if !ok {
			continue
		}
		kept = rolling.Push(c, kept[:0])
		if !pass() {
			return nil
		}
	}
	kept = rolling.Flush(kept[:0])
	if !pass() {
		return nil
	}
	return r.lines.Err()
}

// Warnings returns the warnings of the blocks read so far, sorted by line. A warning without a line comes first.
func (r *StreamReader) Warnings() []model.ParseWarning {
	w := r.st.Warnings()
	slices.SortStableFunc(w, func(a, b model.ParseWarning) int { return cmp.Compare(a.Line, b.Line) })
	return w
}

// Header returns the vtt file format data: header, headerLines, regions and styles.
// It is complete once the reader yielded the first cue, or once Cues ends.
func (r *StreamReader) Header() *model.Data {
	return r.rd.File()
}

// StreamWriter writes WebVTT cue blocks one at a time.
// It writes the bytes of Write.
type StreamWriter struct {
	sink      *writing.Sink
	cw        vtt.Writer
	buf       []byte
	n         int
	hasBlocks bool
}

// NewStreamWriter returns a writer that writes the BOM and the header to w at once.
// header holds the header text, header lines, STYLE and REGION blocks, as StreamReader.Header gives them.
// Nil writes the signature line only. The first return value is nil when the options are invalid.
func NewStreamWriter(w io.Writer, common model.CommonWriteOptions, _ WriteOptions, header *model.Data) (*StreamWriter, error) {
	if err := common.Validate(); err != nil {
		return nil, err
	}
	ww := &StreamWriter{sink: writing.NewSink(w, model.WebVTT, common)}
	ww.cw = vtt.Writer{StripTags: common.StripTags}
	prefix := vtt.Header(header) + "\n\n"
	if blocks := vtt.FileBlocks(header); len(blocks) > 0 {
		prefix += strings.Join(blocks, "\n\n") + "\n"
		ww.hasBlocks = true
	}
	if err := ww.sink.Prefix(prefix); err != nil {
		return nil, err
	}
	return ww, nil
}

// Write writes c as the next block.
// An error of the io.Writer returns wrapped, and the writer stays usable.
func (w *StreamWriter) Write(c model.Cue) error {
	if w.sink.Closed() {
		return writing.ClosedError()
	}
	if drop, err := w.sink.Gate(c); drop || err != nil {
		return err
	}
	w.buf = w.buf[:0]
	if w.hasBlocks {
		w.buf = append(w.buf, '\n')
	}
	w.hasBlocks = true
	w.buf = append(w.buf, w.cw.CueBlock(c, w.n)...)
	w.buf = append(w.buf, '\n')
	w.n++
	return w.sink.Next(w.buf)
}

// DroppedColors returns the font colors that no WebVTT color class has, so the output dropped them,
// for the cues written so far.
func (w *StreamWriter) DroppedColors() []DroppedColor {
	if len(w.cw.Dropped) == 0 {
		return nil
	}
	dropped := make([]DroppedColor, 0, len(w.cw.Dropped))
	for _, d := range w.cw.Dropped {
		dropped = append(dropped, DroppedColor(d))
	}
	return dropped
}

// Close marks the writer closed. It does not close the io.Writer, which the caller owns.
func (w *StreamWriter) Close() error {
	w.sink.Close()
	return nil
}
