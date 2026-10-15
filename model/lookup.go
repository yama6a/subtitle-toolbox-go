package model

import (
	"math"
	"sort"
	"time"
)

// cueIndex holds the starts of the cues in cue order and a binary tree over their ends.
// Node 1 is the root, node n has the children 2n and 2n+1, and each node holds the latest end of its leaves.
type cueIndex struct {
	starts    []time.Duration
	maxEnds   []time.Duration
	leafCount int
}

const noEnd = time.Duration(math.MinInt64)

func newCueIndex(cues []Cue) *cueIndex {
	leafCount := 1
	for leafCount < len(cues) {
		leafCount <<= 1
	}
	x := &cueIndex{
		starts:    make([]time.Duration, len(cues)),
		maxEnds:   make([]time.Duration, 2*leafCount),
		leafCount: leafCount,
	}
	for i, c := range cues {
		x.starts[i] = c.Start
		x.maxEnds[leafCount+i] = c.End
	}
	for i := len(cues); i < leafCount; i++ {
		x.maxEnds[leafCount+i] = noEnd
	}
	for n := leafCount - 1; n >= 1; n-- {
		x.maxEnds[n] = max(x.maxEnds[2*n], x.maxEnds[2*n+1])
	}
	return x
}

// update writes the new times of the cue at position i, which must stay in start order.
func (x *cueIndex) update(i int, c Cue) {
	x.starts[i] = c.Start
	n := x.leafCount + i
	x.maxEnds[n] = c.End
	for n >>= 1; n >= 1; n >>= 1 {
		x.maxEnds[n] = max(x.maxEnds[2*n], x.maxEnds[2*n+1])
	}
}

// overlapping returns the ascending positions of the cues that end after from and start before to,
// or at to when includeTo is set.
func (x *cueIndex) overlapping(from, to time.Duration, includeTo bool) []int {
	limit := sort.Search(len(x.starts), func(i int) bool {
		if includeTo {
			return x.starts[i] > to
		}
		return x.starts[i] >= to
	})
	var out []int
	x.collect(1, 0, x.leafCount, limit, from, &out)
	return out
}

func (x *cueIndex) collect(node, first, size, limit int, from time.Duration, out *[]int) {
	if first >= limit || x.maxEnds[node] <= from {
		return
	}
	if size == 1 {
		*out = append(*out, first)
		return
	}
	half := size >> 1
	x.collect(2*node, first, half, limit, from, out)
	x.collect(2*node+1, first+half, half, limit, from, out)
}
