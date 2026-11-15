// Package hearingimpaired removes hearing-impaired annotations from cue text: sound descriptions in brackets,
// speaker labels such as JOHN: and lines of music symbols. docs/text.md has the rules.
//
// Adapted from Subtitle Edit, src/libse/Forms/RemoveTextForHI.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/5b9ee8baf08c472c7a74fbc337446b656dedabb4/src/libse/Forms/RemoveTextForHI.cs), MIT.
package hearingimpaired

import (
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Report counts what Apply removed.
type Report struct {
	// RemovedLines counts the removed lines, all lines of a removed cue included.
	RemovedLines int
	// RemovedCues counts the cues that had visible text and have none left.
	RemovedCues int
}

// Apply removes the annotations that opts selects from the cues of s, and removes each cue that has no visible
// text left. The comments of a removed cue move to the next cue. Apply returns the error of Validate and changes
// nothing when opts is not valid.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	before := make([]int, s.Len())
	removed := s.EditLines(func(i int, c model.Cue) ([]string, bool) {
		before[i] = len(c.Lines)
		return removeFromLines(c.Lines, opts), true
	}, nil)

	r := Report{RemovedCues: len(removed)}
	kept := 0
	for i, n := range before {
		if len(removed) > 0 && removed[0] == i {
			removed = removed[1:]
			r.RemovedLines += n
			continue
		}
		c, _ := s.Cue(kept)
		kept++
		r.RemovedLines += max(0, n-len(c.Lines))
	}
	return r, nil
}

// IsAnnotation reports whether Apply with opts changes or removes line. It cleans line as model.Cue.SetText does
// first. It returns the error of Validate when opts is not valid.
func IsAnnotation(line string, opts Options) (bool, error) {
	if err := opts.Validate(); err != nil {
		return false, err
	}
	var before, after model.Cue
	before.SetText(line)
	for _, l := range removeFromLines(before.Lines, opts) {
		after.AddLine(l)
	}
	return !slices.Equal(before.Lines, after.Lines), nil
}
