package parsing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"iter"
	"math"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// LineReader reads the lines of a stream. The text pre-pass of Prepare runs on the bytes before the
// lines are cut, so the lines are those that Lines gives for the content that Prepare returns.
// The one difference is that LineReader never replaces invalid UTF-8.
type LineReader struct {
	pp   *prepass
	sc   *bufio.Scanner
	st   *State
	err  error
	used bool
}

// NewLineReader returns a reader of the lines of r. It reads nothing until the first call of Lines.
// The warnings of the pre-pass go to s. A warning for invalid UTF-8 or a control character comes
// first in Warnings, as in a read of the whole content.
func (s *State) NewLineReader(r io.Reader) *LineReader {
	pp := &prepass{st: s, src: r, in: make([]byte, 32*1024), atStart: true}
	sc := bufio.NewScanner(pp)
	sc.Buffer(nil, math.MaxInt)
	sc.Split(splitLine(&splitState{}))
	return &LineReader{pp: pp, sc: sc, st: s}
}

// Err returns the error that ended Lines: a read error, or a NUL character in strict mode.
func (l *LineReader) Err() error {
	return l.err
}

// Lines yields the 0-based number and the text of each line. Only the first call reads the stream.
// A line still holds the BOMs after the start of the stream, so a reader sees joined files.
// Lines ends at the first error, after the lines before it.
func (l *LineReader) Lines() iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		if l.used {
			return
		}
		l.used = true
		for n := 0; l.sc.Scan(); n++ {
			if !yield(n, string(l.sc.Bytes())) {
				return
			}
		}
		switch err := l.sc.Err(); {
		case err == nil:
		case l.pp.failed:
			l.err = err
		default:
			l.err = fmt.Errorf("read %s: %w", l.st.format, err)
		}
	}
}

// prepass is an io.Reader that applies the steps of Prepare to a stream, in the same order.
type prepass struct {
	st  *State
	src io.Reader
	in  []byte
	out []byte

	srcDone bool
	failed  bool
	err     error
	empty   int

	// The UTF-8 check works on the raw bytes and keeps the incomplete rune at the end of a chunk.
	raw     int
	carry   []byte
	seenBad bool

	// atStart holds the first bytes of the stream while they can still be a BOM.
	atStart bool
	head    []byte

	// zrun counts the Ctrl-Z bytes that would end the file, and trail the white space after them.
	zrun  int
	trail []byte

	// lines counts the line ends before the current byte, crs the CRs since the last other byte.
	lines, crs int
	seenCtrl   bool
}

func (p *prepass) Read(b []byte) (int, error) {
	for len(p.out) == 0 {
		if p.err != nil {
			return 0, p.err
		}
		p.fill()
	}
	n := copy(b, p.out)
	p.out = p.out[n:]
	if len(p.out) == 0 {
		p.out = p.out[:0:0]
	}
	return n, nil
}

func (p *prepass) fill() {
	n, err := p.src.Read(p.in)
	if n == 0 && err == nil {
		if p.empty++; p.empty > 100 {
			p.err = io.ErrNoProgress
		}
		return
	}
	p.empty = 0
	p.checkUTF8(p.in[:n], err != nil)
	for _, c := range p.in[:n] {
		p.bom(c)
		if p.failed {
			return
		}
	}
	if err != nil {
		p.finish()
		if !p.failed {
			p.err = err
		}
	}
}

func (p *prepass) checkUTF8(chunk []byte, last bool) {
	start := p.raw - len(p.carry)
	p.raw += len(chunk)
	if p.seenBad {
		return
	}
	data := chunk
	if len(p.carry) > 0 {
		data = append(p.carry, chunk...)
	}
	i := 0
	for i < len(data) {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			if !last && !utf8.FullRune(data[i:]) {
				break
			}
			p.seenBad = true
			if p.st.Lenient() {
				w := newWarning(badUTF8Message(start+i), model.NoLine, model.NoBlock, nil, model.Repaired)
				p.st.badUTF8 = &w
			}
			return
		}
		i += size
	}
	p.carry = append(p.carry[:0], data[i:]...)
}

// bom drops the BOMs at the start of the stream and passes every other byte on.
func (p *prepass) bom(c byte) {
	if !p.atStart {
		p.ctrlZ(c)
		return
	}
	const mark = "\xef\xbb\xbf"
	p.head = append(p.head, c)
	switch {
	case string(p.head) == mark:
		p.head = p.head[:0]
	case len(p.head) < len(mark) && mark[:len(p.head)] == string(p.head):
	default:
		p.atStart = false
		p.flushHead()
	}
}

func (p *prepass) flushHead() {
	head := p.head
	p.head = nil
	for _, c := range head {
		p.ctrlZ(c)
		if p.failed {
			return
		}
	}
}

// ctrlZ holds back a run of Ctrl-Z and the white space after it until a byte shows that the file goes on.
// DOS editors end files with Ctrl-Z, and the pre-pass drops it only there.
func (p *prepass) ctrlZ(c byte) {
	switch {
	case c == ctrlZ:
		if len(p.trail) > 0 {
			p.release()
		}
		p.zrun++
	case p.zrun > 0 && bytes.IndexByte([]byte(whiteSpace), c) >= 0:
		p.trail = append(p.trail, c)
	default:
		p.release()
		p.control(c)
	}
}

func (p *prepass) release() {
	zrun, trail := p.zrun, p.trail
	p.zrun, p.trail = 0, nil
	for range zrun {
		p.control(ctrlZ)
	}
	for _, c := range trail {
		p.control(c)
	}
}

// finish handles the end of the stream.
func (p *prepass) finish() {
	if p.atStart {
		p.atStart = false
		p.flushHead()
	}
	trail := p.trail
	p.zrun, p.trail = 0, nil
	for _, c := range trail {
		p.control(c)
	}
}

// control applies the NUL rule and removes the control characters.
func (p *prepass) control(c byte) {
	switch c {
	case '\r':
		p.crs++
		p.out = append(p.out, c)
		return
	case '\n':
		p.lines++
		p.crs = 0
		p.out = append(p.out, c)
		return
	}
	p.lines += p.crs
	p.crs = 0
	switch {
	case c == 0 && p.st.ReplaceNUL:
		p.out = append(p.out, replacement...)
	case c == 0 && !p.st.Lenient():
		p.failed = true
		p.err = nulError(p.st.format, p.lines+1)
	case isControl(c):
		if !p.seenCtrl && p.st.Lenient() {
			w := newWarning(controlsMessage(p.lines+1), p.lines+1, model.NoBlock, nil, model.Repaired)
			p.st.controls = &w
		}
		p.seenCtrl = true
	default:
		p.out = append(p.out, c)
	}
}

// splitState is the state of splitLine.
type splitState struct {
	// lonely is the count of CRs at the start of the next data, each a line end on its own.
	lonely int
}

// splitLine splits at LF, at a run of CR before LF, and at each lone CR. A CR at the end of the buffer
// needs more input before the split decides.
func splitLine(s *splitState) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if s.lonely > 0 {
			s.lonely--
			return 1, data[:0], nil
		}
		i := bytes.IndexAny(data, "\r\n")
		if i < 0 {
			if atEOF && len(data) > 0 {
				return len(data), data, nil
			}
			return 0, nil, nil
		}
		if data[i] == '\n' {
			return i + 1, data[:i], nil
		}
		j := i
		for j < len(data) && data[j] == '\r' {
			j++
		}
		if j == len(data) && !atEOF {
			return 0, nil, nil
		}
		if j < len(data) && data[j] == '\n' {
			return j + 1, data[:i], nil
		}
		s.lonely = j - i - 1
		return i + 1, data[:i], nil
	}
}
