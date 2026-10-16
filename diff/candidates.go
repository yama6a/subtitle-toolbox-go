package diff

import (
	"slices"
	"time"
)

const (
	// sameTextWindow is the most new cues with the same text that one old cue pairs with.
	sameTextWindow = 50
	longCue        = 60 * time.Second
)

// candidate is a pair of an old and a new cue with a weight above 0.
type candidate struct {
	old, new int
	weight   float64
}

// timeList holds new cues sorted by start, with the longest duration among them. A scan back by the longest
// duration finds each cue that overlaps an old cue. Two lists keep a single long cue from widening the scan
// for all the short ones.
type timeList struct {
	order   []int
	starts  []float64
	longest float64
}

func (c *comparer) timeLists() [2]timeList {
	var groups [2][]int
	for j, cue := range c.newCues {
		g := 0
		if cue.End-cue.Start > longCue {
			g = 1
		}
		groups[g] = append(groups[g], j)
	}
	var lists [2]timeList
	for g, order := range groups {
		slices.SortStableFunc(order, func(a, b int) int {
			switch {
			case c.newStarts[a] < c.newStarts[b]:
				return -1
			case c.newStarts[a] > c.newStarts[b]:
				return 1
			default:
				return 0
			}
		})
		l := timeList{order: order, starts: make([]float64, len(order))}
		for k, j := range order {
			l.starts[k] = c.newStarts[j]
			l.longest = max(l.longest, c.newEnds[j]-c.newStarts[j])
		}
		lists[g] = l
	}
	return lists
}

// partnerSet collects the new cues that pair with one old cue, each once.
type partnerSet struct {
	// owner[j] is the last old cue that took the new cue j.
	owner []int
	old   int
	list  []int
}

func newPartnerSet(newCount int) *partnerSet {
	p := &partnerSet{owner: make([]int, newCount)}
	for j := range p.owner {
		p.owner[j] = -1
	}
	return p
}

func (p *partnerSet) start(old int) {
	p.old = old
	p.list = p.list[:0]
}

func (p *partnerSet) add(j int) {
	if p.owner[j] != p.old {
		p.owner[j] = p.old
		p.list = append(p.list, j)
	}
}

// candidates returns the pairs with the same text and the pairs that overlap in time, with a weight above 0,
// in rising old index and falling new index.
func (c *comparer) candidates() []candidate {
	newByText := make(map[string][]int)
	for j, text := range c.newTexts {
		newByText[text] = append(newByText[text], j)
	}
	lists := c.timeLists()
	partners := newPartnerSet(len(c.newCues))
	var candidates []candidate
	for i, text := range c.oldTexts {
		partners.start(i)
		for _, j := range nearest(newByText[text], i) {
			partners.add(j)
		}
		for _, l := range lists {
			c.overlapping(l, i, partners.add)
		}
		slices.SortFunc(partners.list, func(a, b int) int { return b - a })
		for _, j := range partners.list {
			if w := c.weight(i, j); w > 0 {
				candidates = append(candidates, candidate{old: i, new: j, weight: w})
			}
		}
	}
	return candidates
}

// overlapping calls add for each new cue of l that overlaps the old cue i in time.
func (c *comparer) overlapping(l timeList, i int, add func(j int)) {
	start, _ := slices.BinarySearch(l.starts, c.oldStarts[i]-l.longest)
	for k := start; k < len(l.starts) && l.starts[k] < c.oldEnds[i]; k++ {
		if j := l.order[k]; c.newEnds[j] > c.oldStarts[i] {
			add(j)
		}
	}
}

// nearest returns the sameTextWindow values of sorted that are nearest to index, the lower one first on a tie.
func nearest(sorted []int, index int) []int {
	if len(sorted) <= sameTextWindow {
		return sorted
	}
	after, _ := slices.BinarySearch(sorted, index)
	before := after - 1
	out := make([]int, 0, sameTextWindow)
	for len(out) < sameTextWindow {
		if before >= 0 && (after >= len(sorted) || index-sorted[before] <= sorted[after]-index) {
			out = append(out, sorted[before])
			before--
		} else {
			out = append(out, sorted[after])
			after++
		}
	}
	return out
}
