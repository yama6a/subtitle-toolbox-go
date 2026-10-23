// Package dual merges two subtitles in two languages into one that shows both.
package dual

import (
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// FromPair returns a new subtitle with the cues of primary and secondary. Neither input changes.
//
// The primary cues set the times. The result gets the metadata, the comments and the file format data of primary.
// When both inputs have the language metadata, the language is the two joined by "+", for example "en+de".
// A secondary cue that stays a cue of its own keeps its times, lines, alignment and forced flag, but not its ID
// or its file format data, because they belong to the format of secondary.
func FromPair(primary, secondary *model.Subtitle, opts Options) (*model.Subtitle, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	result := primary.Clone()
	cues := result.Cues()
	apart := secondaryCopies(secondary, opts)
	if opts.Mode == ModeTopBottom {
		placeApart(cues, apart, opts)
	} else {
		apart = stack(result, cues, apart)
	}
	result.ReplaceCues(slices.Concat(cues, apart), func(old int) int { return old })
	if p, ok := primary.Metadata(model.MetadataLanguage); ok {
		if s, ok := secondary.Metadata(model.MetadataLanguage); ok {
			result.SetMetadata(model.MetadataLanguage, p+"+"+s)
		}
	}
	return result, nil
}

func secondaryCopies(s *model.Subtitle, opts Options) []model.Cue {
	cues := make([]model.Cue, 0, s.Len())
	for _, c := range s.All() {
		lines := make([]string, len(c.Lines))
		for i, line := range c.Lines {
			lines[i] = opts.style(line)
		}
		cues = append(cues, model.Cue{Start: c.Start, End: c.End, Lines: lines, Alignment: c.Alignment, Forced: c.Forced})
	}
	return cues
}

// stack adds the lines of each secondary cue to the primary cue that it overlaps most and returns the secondary cues
// without a partner. The partners are searched before any primary cue changes. lookup holds the same cues as primary.
func stack(lookup *model.Subtitle, primary, secondary []model.Cue) []model.Cue {
	partners := make([]int, len(secondary))
	var unpaired []model.Cue
	for i, c := range secondary {
		partners[i] = bestPartner(lookup, primary, c)
		if partners[i] < 0 {
			unpaired = append(unpaired, c)
		}
	}
	for i, c := range secondary {
		if p := partners[i]; p >= 0 {
			joined := &primary[p]
			joined.Lines = append(joined.Lines, c.Lines...)
			joined.Start = min(joined.Start, c.Start)
			joined.End = max(joined.End, c.End)
		}
	}
	return unpaired
}

// bestPartner returns the index of the primary cue with the largest positive overlap with c, the first one on a tie,
// or -1.
func bestPartner(lookup *model.Subtitle, primary []model.Cue, c model.Cue) int {
	if c.End <= c.Start {
		return -1
	}
	candidates, _ := lookup.CuesBetween(c.Start, c.End) // the error needs a start after the end
	best, bestOverlap := -1, time.Duration(0)
	for _, i := range candidates {
		if overlap := min(primary[i].End, c.End) - max(primary[i].Start, c.Start); overlap > bestOverlap {
			best, bestOverlap = i, overlap
		}
	}
	return best
}

// placeApart moves the secondary cues to the secondary alignment and snaps their times to the primary times.
func placeApart(primary, secondary []model.Cue, opts Options) {
	times := make([]time.Duration, 0, 2*len(primary))
	for _, c := range primary {
		times = append(times, c.Start, c.End)
	}
	slices.Sort(times)

	tolerance := opts.timeTolerance()
	alignment := opts.secondaryAlignment()
	for i := range secondary {
		c := &secondary[i]
		start := snap(c.Start, times, tolerance)
		end := snap(c.End, times, tolerance)
		// Snapping both times of a short cue to the same primary time would hide the cue.
		if end > start {
			c.Start, c.End = start, end
		}
		c.Alignment = alignment
	}
}

// snap returns the time of times that is closest to t and at most tolerance away, the earlier one on a tie.
// Without such a time it returns t.
func snap(t time.Duration, times []time.Duration, tolerance time.Duration) time.Duration {
	i, _ := slices.BinarySearch(times, t)
	snapped, distance, found := t, time.Duration(0), false
	if i > 0 && t-times[i-1] <= tolerance {
		snapped, distance, found = times[i-1], t-times[i-1], true
	}
	if i < len(times) && times[i]-t <= tolerance && (!found || times[i]-t < distance) {
		snapped = times[i]
	}
	return snapped
}
