package wordgroup

import (
	"math/bits"
	"sort"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	hashMod  = 1<<61 - 1
	hashBase = 1_000_003_457
)

func mulMod(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	r := (lo & hashMod) + (lo >> 61) + (hi << 3)
	r = (r & hashMod) + (r >> 61)
	if r >= hashMod {
		r -= hashMod
	}
	return r
}

func extend(h uint64, b byte) uint64 {
	h = mulMod(h, hashBase) + uint64(b) + 1
	if h >= hashMod {
		h -= hashMod
	}
	return h
}

func extendString(h uint64, s string) uint64 {
	for i := range len(s) {
		h = extend(h, s[i])
	}
	return h
}

// runKey names the joined text of one run of source words: the start of its first word, the length and a hash of the text.
type runKey struct {
	start  time.Duration
	length int
	hash   uint64
}

// runIndex holds, for each run that starts at a source word with a start, the index of that word. A run is the word
// and the source words without a start after it. The lists are in ascending order. The runs of all words together
// hold each source word once, so the index has one entry for each source word.
type runIndex map[runKey][]int

func newRunIndex(source []SourceWord) runIndex {
	idx := runIndex{}
	for i, s := range source {
		if s.Start == nil {
			continue
		}
		start := s.Start.Round(time.Millisecond)
		var h uint64
		length := 0
		for j := i; j < len(source); j++ {
			if j > i {
				if source[j].Start != nil {
					break
				}
				h = extend(h, ' ')
				length++
			}
			h = extendString(h, source[j].Text)
			length += len(source[j].Text)
			k := runKey{start, length, h}
			idx[k] = append(idx[k], i)
		}
	}
	return idx
}

// find returns the first source word at or after next that starts a run with this start that matches text, and the
// index after that run. A run matches when it is the text, or the text is the run followed by punctuation.
func (idx runIndex) find(text string, start time.Duration, source []SourceWord, next int) (first, end int, ok bool) {
	cuts, hashes := cutHashes(text)
	first = -1
	for k, cut := range cuts {
		list := idx[runKey{start, cut, hashes[k]}]
		for _, i := range list[sort.SearchInts(list, next):] {
			if first >= 0 && i >= first {
				break
			}
			if e, match := runMatch(text, source, i); match {
				first, end = i, e
				break
			}
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return first, end, true
}

// bodyLen returns the length of text without its trailing punctuation.
func bodyLen(text string) int {
	body := len(text)
	for body > 0 {
		r, n := utf8.DecodeLastRuneInString(text[:body])
		if r == utf8.RuneError || !unicode.IsPunct(r) {
			break
		}
		body -= n
	}
	return body
}

// cutHashes returns each length of text at which a run can end, with the hash of text up to it. They are the whole
// text, and each place inside its trailing punctuation that is the start of a rune.
func cutHashes(text string) (cuts []int, hashes []uint64) {
	body := bodyLen(text)
	var h uint64
	for i := range len(text) {
		h = extend(h, text[i])
		if i+1 >= body && i+1 > 0 && (i+1 == len(text) || utf8.RuneStart(text[i+1])) {
			cuts = append(cuts, i+1)
			hashes = append(hashes, h)
		}
	}
	return cuts, hashes
}

// untimedIndex holds the joined text of each maximal block of source words without a start, with a hash of each prefix, so
// a run of one block is compared with a text by 2 hashes.
type untimedIndex struct {
	from, to     []int // offsets of each word in the block text, indexed by source index
	blockEnd     []int // the last source index of the block of each word
	before, upto []uint64
	untimed      []int
}

func newUntimedIndex(source []SourceWord) *untimedIndex {
	n := len(source)
	u := &untimedIndex{from: make([]int, n), to: make([]int, n), blockEnd: make([]int, n), before: make([]uint64, n), upto: make([]uint64, n)}
	for i := 0; i < n; i++ {
		if source[i].Start != nil {
			continue
		}
		first := i
		var h uint64
		off := 0
		for ; i < n && source[i].Start == nil; i++ {
			if i > first {
				h = extend(h, ' ')
				off++
			}
			u.untimed = append(u.untimed, i)
			u.from[i], u.before[i] = off, h
			h = extendString(h, source[i].Text)
			off += len(source[i].Text)
			u.to[i], u.upto[i] = off, h
		}
		for j := first; j < i; j++ {
			u.blockEnd[j] = i - 1
		}
		i--
	}
	return u
}

// leadText is a text with the hash of each prefix and the power of the hash base for each length.
type leadText struct {
	text   string
	body   int
	prefix []uint64
	pow    []uint64
}

func newLeadText(text string) leadText {
	l := leadText{text: text, body: bodyLen(text), prefix: make([]uint64, len(text)+1), pow: make([]uint64, len(text)+1)}
	l.pow[0] = 1
	for i := range len(text) {
		l.prefix[i+1] = extend(l.prefix[i], text[i])
		l.pow[i+1] = mulMod(l.pow[i], hashBase)
	}
	return l
}

// matches reports whether the run that starts at the source word i is the text, or the text is the run followed by
// punctuation. The shortest run that can match ends at or after the end of the text body. A longer run would put a
// space inside the trailing punctuation, so only that run needs a check.
func (u *untimedIndex) matches(l leadText, i int) bool {
	last := u.blockEnd[i]
	j := i + sort.Search(last-i+1, func(k int) bool { return u.to[i+k]-u.from[i] >= l.body })
	if j > last {
		return false
	}
	cut := u.to[j] - u.from[i]
	if cut == 0 || cut > len(l.text) || (cut < len(l.text) && !utf8.RuneStart(l.text[cut])) {
		return false
	}
	run := (u.upto[j] + hashMod - mulMod(u.before[i], l.pow[cut])) % hashMod
	return run == l.prefix[cut]
}

// find returns the first source word without a start at or after next whose run matches text, as runIndex.find does.
func (u *untimedIndex) find(text string, source []SourceWord, next int) (first, end int, ok bool) {
	if len(u.untimed) == 0 {
		return 0, 0, false
	}
	l := newLeadText(text)
	for _, i := range u.untimed[sort.SearchInts(u.untimed, next):] {
		if !u.matches(l, i) {
			continue
		}
		if e, match := runMatch(text, source, i); match {
			return i, e, true
		}
	}
	return 0, 0, false
}
