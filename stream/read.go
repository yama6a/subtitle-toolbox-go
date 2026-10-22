package stream

import (
	"cmp"
	"io"
	"iter"
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/subrip"
	"github.com/yama6a/subtitle-toolbox-go/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// SubRipReader reads the cues of a SubRip input one block at a time.
type SubRipReader struct {
	st    *parsing.State
	lines *parsing.LineReader
	rd    *subrip.Reader
	used  bool
}

// NewSubRipReader returns a reader of the SubRip text in r. It reads nothing until Cues runs.
func NewSubRipReader(r io.Reader, opts ReadOptions) *SubRipReader {
	st := parsing.NewState(model.SubRip, opts.common())
	lines := st.NewLineReader(r)
	rd := subrip.NewReader(st)
	rd.CutAt(lines.CutLine)
	return &SubRipReader{st: st, lines: lines, rd: rd}
}

// Cues reads the input once and yields its cues in file order, without a sort by start.
// On an error it yields a zero cue and the error, and stops. A second call yields nothing.
// Breaking out of the loop stops the read.
func (r *SubRipReader) Cues() iter.Seq2[model.Cue, error] {
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
				yield(model.Cue{}, firstError(r.lines, err))
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

// Warnings returns the warnings of the blocks read so far, in the order of the full reader.
func (r *SubRipReader) Warnings() []model.ParseWarning {
	return r.st.Warnings()
}

// WebVTTReader reads the cues of a WebVTT input one block at a time.
type WebVTTReader struct {
	st    *parsing.State
	lines *parsing.LineReader
	rd    *vtt.Reader
	used  bool
}

// NewWebVTTReader returns a reader of the WebVTT text in r. It reads nothing until Cues runs.
func NewWebVTTReader(r io.Reader, opts ReadOptions) *WebVTTReader {
	st := parsing.NewState(model.WebVTT, opts.common())
	st.ReplaceNUL = true
	lines := st.NewLineReader(r)
	rd := vtt.NewReader(st, false)
	rd.CutAt(lines.CutLine)
	return &WebVTTReader{st: st, lines: lines, rd: rd}
}

// Cues reads the input once and yields its cues in file order, without a sort by start.
// It skips NOTE blocks. On an error it yields a zero cue and the error, and stops.
// A second call yields nothing. Breaking out of the loop stops the read.
func (r *WebVTTReader) Cues() iter.Seq2[model.Cue, error] {
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
func (r *WebVTTReader) run(emit func(model.Cue) bool) error {
	var (
		rolling vtt.Rolling
		kept    []vtt.Kept
	)
	pass := func() bool {
		for _, k := range kept {
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
			return firstError(r.lines, err)
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
func (r *WebVTTReader) Warnings() []model.ParseWarning {
	w := r.st.Warnings()
	slices.SortStableFunc(w, func(a, b model.ParseWarning) int { return cmp.Compare(a.Line, b.Line) })
	return w
}

// Header returns the vtt file format data: header, headerLines, regions and styles.
// It is complete once the reader yielded the first cue, or once Cues ends.
func (r *WebVTTReader) Header() *model.Data {
	return r.rd.File()
}

// firstError returns the error of the line reader when it has one, because it ended the lines
// and the block error may follow from that. Otherwise it returns err.
func firstError(lines *parsing.LineReader, err error) error {
	if lerr := lines.Err(); lerr != nil {
		return lerr
	}
	return err
}
