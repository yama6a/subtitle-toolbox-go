// Package diff pairs the cues of two subtitles by time and text and reports the cues that were added, removed
// or changed. It pairs by time and text, not by cue number, so one added cue does not shift the later pairs.
package diff

import (
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Kind is the kind of a difference. The values are the text and JSON values.
type Kind string

// The kinds of a difference.
const (
	Added                Kind = "added"
	Removed              Kind = "removed"
	TextChanged          Kind = "text changed"
	TimingChanged        Kind = "timing changed"
	TextAndTimingChanged Kind = "text and timing changed"
)

// Difference is one added, removed or changed cue.
type Difference struct {
	// Kind is the kind of the difference.
	Kind Kind
	// OldIndex is the 0-based index in the old subtitle, -1 for an added cue.
	OldIndex int
	// NewIndex is the 0-based index in the new subtitle, -1 for a removed cue.
	NewIndex int
	// Old is a copy of the old cue, nil for an added cue.
	Old *model.Cue
	// New is a copy of the new cue, nil for a removed cue.
	New *model.Cue
}

// Compare pairs the cues of old and updated by time and text. It returns the cues without a partner and the pairs
// that differ, in the order of the cues. Neither subtitle changes. It returns an error that matches
// model.ErrInvalidArgument when opts is invalid.
func Compare(old, updated *model.Subtitle, opts Options) ([]Difference, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	c := newComparer(old.Cues(), updated.Cues(), opts)
	var differences []Difference
	for _, p := range c.align() {
		switch {
		case p.new < 0:
			differences = append(differences, Difference{Kind: Removed, OldIndex: p.old, NewIndex: -1, Old: &c.oldCues[p.old]})
		case p.old < 0:
			differences = append(differences, Difference{Kind: Added, OldIndex: -1, NewIndex: p.new, New: &c.newCues[p.new]})
		default:
			if kind, ok := c.classify(p.old, p.new); ok {
				differences = append(differences, Difference{
					Kind: kind, OldIndex: p.old, NewIndex: p.new, Old: &c.oldCues[p.old], New: &c.newCues[p.new],
				})
			}
		}
	}
	return differences, nil
}

// Equal reports whether Compare finds no difference between a and b.
func Equal(a, b *model.Subtitle, opts Options) (bool, error) {
	differences, err := Compare(a, b, opts)
	if err != nil {
		return false, err
	}
	return len(differences) == 0, nil
}

// classify returns the kind of the pair, or false when the cues do not differ.
func (c *comparer) classify(i, j int) (Kind, bool) {
	textChanged := c.oldTexts[i] != c.newTexts[j] || c.oldCues[i].Forced != c.newCues[j].Forced
	timingChanged := !c.textOnly && !c.sameTime(i, j)
	switch {
	case textChanged && timingChanged:
		return TextAndTimingChanged, true
	case textChanged:
		return TextChanged, true
	case timingChanged:
		return TimingChanged, true
	default:
		return "", false
	}
}
