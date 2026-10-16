package refsync

import (
	"math"
	"slices"
)

// splitBlocks is the number of blocks that the split search cuts the target into. A search per span can pick
// other splits than PHP, so Go keeps the blocks.
const splitBlocks = 200

// splitNode is the start of one segment on the best path of a layer.
type splitNode struct {
	offsetIndex int
	startBlock  int
	// parent is the node of the segment before, or -1 for the first segment.
	parent int
}

type floor struct {
	value float64
	node  int
}

// splitSearch finds the highest overlap up to each block, for each layer and each coarse offset. A layer is the
// number of splits so far. A segment that starts at a block takes the best value of the layer below as its floor.
type splitSearch struct {
	values  [][]float64
	starts  [][]int
	parents [][]int
	floors  []floor
	nodes   []splitNode
}

func newSplitSearch(blocks [][]span, reference []span, p params) *splitSearch {
	layers := p.maxSplits + 1
	offsets := coarseCount(p.lo, p.hi) + 1
	s := &splitSearch{
		values:  make([][]float64, layers),
		starts:  make([][]int, layers),
		parents: make([][]int, layers),
		floors:  make([]floor, layers),
	}
	for l := range layers {
		s.values[l] = make([]float64, offsets)
		s.starts[l] = make([]int, offsets)
		s.parents[l] = slices.Repeat([]int{-1}, offsets)
		s.floors[l] = floor{math.Inf(-1), -1}
	}
	for b, block := range blocks {
		for l := layers - 1; l >= 1 && b > 0; l-- {
			s.floors[l] = s.bestNode(l - 1)
		}
		overlaps := overlapPerOffset(block, reference, p.lo, p.hi)
		for l := range layers {
			s.addBlock(l, b, overlaps)
		}
	}
	return s
}

// addBlock adds the overlaps of block b to layer l. An offset whose value is below the floor starts a new segment.
func (s *splitSearch) addBlock(l, b int, overlaps []float64) {
	f := s.floors[l]
	row := s.values[l]
	for i, v := range overlaps {
		if f.value > row[i]+epsilon {
			row[i] = f.value
			s.starts[l][i] = b
			s.parents[l][i] = f.node
		}
		row[i] += v
	}
}

// bestNode returns the best value of layer l and a new node that records it, or the floor of the layer when the
// floor is higher.
func (s *splitSearch) bestNode(l int) floor {
	row := s.values[l]
	i := firstMax(row)
	if row[i] < s.floors[l].value {
		return s.floors[l]
	}
	s.nodes = append(s.nodes, splitNode{offsetIndex: i, startBlock: s.starts[l][i], parent: s.parents[l][i]})
	return floor{row[i], len(s.nodes) - 1}
}

// link is one segment of a chain: its coarse offset index, its first span and its offset in seconds.
type link struct {
	offsetIndex int
	start       int
	offset      float64
}

// backtrack returns the segments of the best path of layer l, first segment first.
func (s *splitSearch) backtrack(l, blockSize int) []link {
	var reversed []link
	for node := s.bestNode(l).node; node >= 0; node = s.nodes[node].parent {
		n := s.nodes[node]
		if last := len(reversed) - 1; last >= 0 && reversed[last].offsetIndex == n.offsetIndex {
			reversed[last].start = n.startBlock * blockSize
			continue
		}
		reversed = append(reversed, link{offsetIndex: n.offsetIndex, start: n.startBlock * blockSize})
	}
	slices.Reverse(reversed)
	return reversed
}

// candidate is a result with splits: the segments and the overlap of all segments together.
type candidate struct {
	segments []segmentResult
	overlap  float64
}

type segmentResult struct {
	from   int64 // milliseconds
	offset int64 // milliseconds
}

// segmentCandidates returns the best segments for each number of splits from 1 to maxSplits. The splits first fall
// on block boundaries. Then each split moves to the best span near its boundary.
func segmentCandidates(unscaled, scaled, reference []span, p params) []candidate {
	blockSize := (len(scaled) + splitBlocks - 1) / splitBlocks
	search := newSplitSearch(slices.Collect(slices.Chunk(scaled, blockSize)), reference, p)
	var out []candidate
	for l := 1; l <= p.maxSplits; l++ {
		if chain := search.backtrack(l, blockSize); len(chain) >= 2 {
			out = append(out, refineChain(chain, unscaled, scaled, reference, blockSize, p))
		}
	}
	return out
}

// refineChain moves each split to its best span and refines the offset of each segment.
func refineChain(chain []link, unscaled, scaled, reference []span, blockSize int, p params) candidate {
	for i := range chain {
		chain[i].offset = p.lo + float64(chain[i].offsetIndex)*coarseStep
		if i > 0 {
			limit := len(scaled) - 1
			if i+1 < len(chain) {
				limit = chain[i+1].start - 1
			}
			chain[i].start = bestSplit(scaled, reference, chain[i-1], chain[i], blockSize, limit)
		}
	}

	var c candidate
	for i, l := range chain {
		end := len(scaled)
		if i+1 < len(chain) {
			end = chain[i+1].start
		}
		offset, v := refine(scaled[l.start:end], reference, l.offset, p.lo, p.hi)
		var from int64
		if i > 0 {
			from = milliseconds(unscaled[l.start].start)
		}
		c.segments = append(c.segments, segmentResult{from: from, offset: milliseconds(offset)})
		c.overlap += v
	}
	return c
}

// bestSplit returns the span index within one block of the split between before and after where the overlap is
// highest. The split stays after the first span of before and at or before limit.
func bestSplit(scaled, reference []span, before, after link, blockSize, limit int) int {
	from := max(before.start+1, after.start-blockSize)
	to := min(limit, after.start+blockSize)
	window := scaled[from:to]
	early := overlapPerSpan(window, reference, before.offset)
	late := overlapPerSpan(window, reference, after.offset)

	var total float64
	for _, v := range late {
		total += v
	}
	best, split := total, from
	for i := range window {
		total += early[i] - late[i]
		if total > best+epsilon {
			best, split = total, from+i+1
		}
	}
	return split
}
