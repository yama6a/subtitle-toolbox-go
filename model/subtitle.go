package model

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sort"
	"strconv"
	"time"
)

// The named metadata keys. Other keys are free.
const (
	MetadataTitle    = "title"
	MetadataAuthor   = "author"
	MetadataArtist   = "artist"
	MetadataAlbum    = "album"
	MetadataLanguage = "language"
)

// Subtitle is a list of cues in start order, with comments, metadata and format data.
// The zero value is an empty subtitle.
//
// A Subtitle stores copies of the cues it takes and hands out copies, so a change to a Cue value
// takes effect only through SetCue. A cue index is valid until the next change: SetCue, AddCue,
// RemoveCue and the other changes can put another cue at the same index.
//
// A Subtitle is not safe for concurrent use. The first lookup after a change builds an index, so
// concurrent lookups need a lock too.
type Subtitle struct {
	cues     []Cue
	comments []Comment
	metadata map[string]string
	index    *cueIndex
}

// Comment is text that a writer puts before the cue at BeforeCue, or after the last cue.
type Comment struct {
	// Text is the comment text, without the comment syntax of a format.
	Text string
	// BeforeCue is the index of the cue that the comment comes before. The cue count or more means after the last cue.
	BeforeCue int
}

// Len returns the number of cues.
func (s *Subtitle) Len() int {
	return len(s.cues)
}

// Cue returns a copy of the cue at index i, or false when no cue has that index.
func (s *Subtitle) Cue(i int) (Cue, bool) {
	if i < 0 || i >= len(s.cues) {
		return Cue{}, false
	}
	return s.cues[i].Clone(), true
}

// Cues returns copies of the cues in start order.
func (s *Subtitle) Cues() []Cue {
	if len(s.cues) == 0 {
		return nil
	}
	out := make([]Cue, len(s.cues))
	for i, c := range s.cues {
		out[i] = c.Clone()
	}
	return out
}

// All returns the cue indexes and copies of the cues in start order.
// The loop sees the cues as they were when it started. Changes inside the loop do not affect it.
func (s *Subtitle) All() iter.Seq2[int, Cue] {
	return func(yield func(int, Cue) bool) {
		for i, c := range s.Cues() {
			if !yield(i, c) {
				return
			}
		}
	}
}

// AddCue adds a rounded and cleaned copy of c after the cues that start at or before c.
// Each comment stays before its cue.
func (s *Subtitle) AddCue(c Cue) {
	s.AddCues(c)
}

// AddCues adds rounded and cleaned copies of cues and sorts all cues once with a stable sort, so an added cue
// goes after the stored cues with the same start. Each comment stays before its cue.
func (s *Subtitle) AddCues(cues ...Cue) {
	if len(cues) == 0 {
		return
	}
	added := make([]Cue, len(cues))
	for i, c := range cues {
		added[i] = c.normalized()
	}

	n := len(s.cues)
	if (n == 0 || s.cues[n-1].Start <= added[0].Start) && startsInOrder(added) {
		s.cues = append(s.cues, added...)
		s.index = nil
		for k := len(s.comments) - 1; k >= 0 && s.comments[k].BeforeCue >= n; k-- {
			s.comments[k].BeforeCue = len(s.cues)
		}
		return
	}

	if len(added) == 1 {
		s.insert(added[0])
		return
	}

	s.place(append(s.cues, added...), s.anchors(nil))
}

// anchors returns the old cue index of each comment, mapped through anchor when it is not nil,
// or -1 for a comment after the last cue.
func (s *Subtitle) anchors(anchor func(old int) int) []int {
	out := make([]int, len(s.comments))
	for k, c := range s.comments {
		switch {
		case c.BeforeCue >= len(s.cues):
			out[k] = -1
		case anchor == nil:
			out[k] = c.BeforeCue
		default:
			out[k] = anchor(c.BeforeCue)
		}
	}
	return out
}

// insert puts c after the stored cues that start at or before it.
func (s *Subtitle) insert(c Cue) {
	n := len(s.cues)
	at := sort.Search(n, func(j int) bool { return s.cues[j].Start > c.Start })
	s.cues = slices.Insert(s.cues, at, c)
	s.index = nil
	for k := range s.comments {
		switch b := &s.comments[k].BeforeCue; {
		case *b >= n:
			*b = n + 1
		case *b >= at:
			*b++
		}
	}
}

// SetCue replaces the cue at index i with a rounded and cleaned copy of c.
// The cues end up in the order of a stable sort of the changed list, so c can move to another index.
// Each comment stays before its cue. SetCue returns ErrCueNotFound when no cue has the index i.
func (s *Subtitle) SetCue(i int, c Cue) error {
	if i < 0 || i >= len(s.cues) {
		return cueNotFound(i, len(s.cues))
	}
	c = c.normalized()
	s.cues[i] = c

	to := s.sortedPosition(i)
	if to == i {
		if s.index != nil {
			s.index.update(i, c)
		}
		return nil
	}

	if to < i {
		copy(s.cues[to+1:i+1], s.cues[to:i])
	} else {
		copy(s.cues[i:to], s.cues[i+1:to+1])
	}
	s.cues[to] = c
	s.index = nil
	s.moveComments(i, to)
	return nil
}

// sortedPosition returns the index that the cue at i takes in a stable sort of the cues.
// All other cues must be in start order.
func (s *Subtitle) sortedPosition(i int) int {
	start := s.cues[i].Start
	if i > 0 && start < s.cues[i-1].Start {
		return sort.Search(i, func(j int) bool { return s.cues[j].Start > start })
	}
	if i+1 < len(s.cues) && start > s.cues[i+1].Start {
		later := s.cues[i+1:]
		return i + sort.Search(len(later), func(j int) bool { return later[j].Start >= start })
	}
	return i
}

func (s *Subtitle) moveComments(from, to int) {
	for k := range s.comments {
		b := &s.comments[k].BeforeCue
		switch {
		case *b >= len(s.cues):
			*b = len(s.cues)
		case *b == from:
			*b = to
		case to < from && *b >= to && *b < from:
			*b++
		case to > from && *b > from && *b <= to:
			*b--
		}
	}
	sortComments(s.comments)
}

// RemoveCue removes the cue at index i. The later cues move down by one index.
// A comment before the removed cue then comes before the cue that followed it.
// RemoveCue returns ErrCueNotFound when no cue has the index i.
func (s *Subtitle) RemoveCue(i int) error {
	if i < 0 || i >= len(s.cues) {
		return cueNotFound(i, len(s.cues))
	}
	s.cues = slices.Delete(s.cues, i, i+1)
	s.index = nil
	for k := range s.comments {
		if b := s.comments[k].BeforeCue; b > i {
			s.comments[k].BeforeCue = min(b-1, len(s.cues))
		}
	}
	return nil
}

// RemoveCuesWhere removes the cues for which match returns true. Match gets a copy of each cue.
// A comment before a removed cue moves to the next kept cue, or after the last cue.
func (s *Subtitle) RemoveCuesWhere(match func(Cue) bool) {
	n := len(s.cues)
	keep := make([]bool, n)
	kept := 0
	for i, c := range s.cues {
		if !match(c.Clone()) {
			keep[i] = true
			kept++
		}
	}
	if kept == n {
		return
	}

	next := make([]int, n+1)
	next[n] = kept
	for i := n - 1; i >= 0; i-- {
		next[i] = next[i+1]
		if keep[i] {
			next[i]--
		}
	}
	for k := range s.comments {
		s.comments[k].BeforeCue = next[min(s.comments[k].BeforeCue, n)]
	}

	out := s.cues[:0]
	for i, c := range s.cues {
		if keep[i] {
			out = append(out, c)
		}
	}
	clear(s.cues[len(out):])
	s.cues = out
	s.index = nil
}

// ReplaceCues replaces the cues of s with cues, rounded, cleaned and sorted once with a stable sort.
// A comment that stood before old cue i moves before cues[anchor(i)]. A nil anchor keeps index i.
// An index out of range puts the comment after the last cue.
// With a non-nil anchor, a comment after the last old cue stays after the last cue, and anchor is not called for it.
func (s *Subtitle) ReplaceCues(cues []Cue, anchor func(old int) int) {
	normalized := make([]Cue, len(cues))
	for i, c := range cues {
		normalized[i] = c.normalized()
	}
	anchors := s.anchors(anchor)
	if anchor == nil {
		for k, c := range s.comments {
			anchors[k] = c.BeforeCue
		}
	}
	s.place(normalized, anchors)
}

// place stores cues in stable start order. The comment k moves before the cue that was cues[anchors[k]],
// or after the last cue when anchors[k] is out of range.
func (s *Subtitle) place(cues []Cue, anchors []int) {
	order := make([]int, len(cues))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(cues[a].Start, cues[b].Start) })

	sorted := make([]Cue, len(cues))
	position := make([]int, len(cues))
	for p, i := range order {
		sorted[p] = cues[i]
		position[i] = p
	}
	s.cues = sorted
	s.index = nil

	for k, a := range anchors {
		if a >= 0 && a < len(cues) {
			s.comments[k].BeforeCue = position[a]
		} else {
			s.comments[k].BeforeCue = len(cues)
		}
	}
	sortComments(s.comments)
}

// CuesAt returns the ascending indexes of the cues on screen at t. A cue is on screen when Start <= t < End.
func (s *Subtitle) CuesAt(t time.Duration) []int {
	return s.lookup().overlapping(t, t, true)
}

// CueIndexAt returns the lowest index of the cues on screen at t, or false when no cue is on screen.
func (s *Subtitle) CueIndexAt(t time.Duration) (int, bool) {
	found := s.CuesAt(t)
	if len(found) == 0 {
		return 0, false
	}
	return found[0], true
}

// CuesBetween returns the ascending indexes of the cues that overlap the range from to to, that is
// the cues with Start < to and End > from. It returns an error that matches ErrInvalidArgument when from is after to.
func (s *Subtitle) CuesBetween(from, to time.Duration) ([]int, error) {
	if from > to {
		return nil, invalidArgument(fmt.Sprintf("The range start %s must not be after the range end %s.", phpSeconds(from), phpSeconds(to)))
	}
	return s.lookup().overlapping(from, to, false), nil
}

// CuesWhere returns the ascending indexes of the cues for which match returns true. Match gets a copy of each cue.
func (s *Subtitle) CuesWhere(match func(Cue) bool) []int {
	var out []int
	for i, c := range s.cues {
		if match(c.Clone()) {
			out = append(out, i)
		}
	}
	return out
}

func (s *Subtitle) lookup() *cueIndex {
	if s.index == nil {
		s.index = newCueIndex(s.cues)
	}
	return s.index
}

// Metadata returns the metadata value of key, or false when s has no value for key.
func (s *Subtitle) Metadata(key string) (string, bool) {
	v, ok := s.metadata[key]
	return v, ok
}

// SetMetadata sets the metadata value of key. An empty value is a value.
func (s *Subtitle) SetMetadata(key, value string) {
	if s.metadata == nil {
		s.metadata = map[string]string{}
	}
	s.metadata[key] = value
}

// DeleteMetadata removes the metadata value of key.
func (s *Subtitle) DeleteMetadata(key string) {
	delete(s.metadata, key)
}

// AllMetadata returns a copy of the metadata. The map is never nil.
func (s *Subtitle) AllMetadata() map[string]string {
	out := make(map[string]string, len(s.metadata))
	maps.Copy(out, s.metadata)
	return out
}

// Comments returns a copy of the comments, sorted by BeforeCue.
func (s *Subtitle) Comments() []Comment {
	return slices.Clone(s.comments)
}

// AddComment adds a comment before the cue at index beforeCue. An index at or above the cue count is stored
// as is and means after the last cue. It returns an error that matches ErrInvalidArgument for a negative index.
func (s *Subtitle) AddComment(text string, beforeCue int) error {
	if beforeCue < 0 {
		return invalidArgument(fmt.Sprintf("The cue index of a comment must not be negative, got %d.", beforeCue))
	}
	s.comments = append(s.comments, Comment{Text: text, BeforeCue: beforeCue})
	if n := len(s.comments); n > 1 && s.comments[n-2].BeforeCue > beforeCue {
		sortComments(s.comments)
	}
	return nil
}

// Clone returns a deep copy of s that shares no cue, line, comment or metadata with s.
func (s *Subtitle) Clone() *Subtitle {
	return &Subtitle{
		cues:     s.Cues(),
		comments: s.Comments(),
		metadata: maps.Clone(s.metadata),
	}
}

func startsInOrder(cues []Cue) bool {
	return slices.IsSortedFunc(cues, func(a, b Cue) int { return cmp.Compare(a.Start, b.Start) })
}

func sortComments(comments []Comment) {
	slices.SortStableFunc(comments, func(a, b Comment) int { return cmp.Compare(a.BeforeCue, b.BeforeCue) })
}

func cueNotFound(i, n int) error {
	return fmt.Errorf("%w: index %d, the subtitle has %d cues", ErrCueNotFound, i, n)
}

// phpSeconds formats d as PHP prints a float of seconds, for example "10" or "2.5", so messages keep the PHP text.
func phpSeconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64)
}

func invalidArgument(msg string) error {
	return &InvalidArgumentError{Msg: msg}
}
