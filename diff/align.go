package diff

import "slices"

// fullAlignMaxCells is the largest stretch, in cells of the table, that gets the full dynamic program. It keeps
// the memory bounded. In a larger stretch, a cue with nearly the same text that does not overlap in time is
// removed and added.
const fullAlignMaxCells = 40000

// epsilon makes a diagonal step win only when it beats the other steps by more than float rounding.
const epsilon = 1e-9

// align returns the pairs of a weighted longest common subsequence in order. Pairs with the same text split the
// lists into stretches. A stretch up to fullAlignMaxCells gets the full dynamic program. A larger stretch keeps
// the pairs of the heaviest chain inside it.
func (c *comparer) align() []pair {
	chain := c.heaviestChain(c.candidates())
	var anchors []pair
	for _, p := range chain {
		if c.oldTexts[p.old] == c.newTexts[p.new] {
			anchors = append(anchors, p)
		}
	}
	anchors = append(anchors, pair{len(c.oldTexts), len(c.newTexts)})

	var pairs []pair
	oldFrom, newFrom, next := 0, 0, 0
	for _, anchor := range anchors {
		if (anchor.old-oldFrom)*(anchor.new-newFrom) <= fullAlignMaxCells {
			pairs = append(pairs, c.fullAlign(oldFrom, anchor.old, newFrom, anchor.new)...)
		} else {
			var inside []pair
			for next < len(chain) && chain[next].old < anchor.old {
				inside = append(inside, chain[next])
				next++
			}
			pairs = append(pairs, fillGaps(inside, oldFrom, newFrom, anchor)...)
		}
		if anchor.old < len(c.oldTexts) {
			pairs = append(pairs, anchor)
		}
		for next < len(chain) && chain[next].old <= anchor.old {
			next++
		}
		oldFrom, newFrom = anchor.old+1, anchor.new+1
	}
	return pairs
}

// fillGaps adds the unpaired cues of a stretch around the pairs, removed cues before added ones.
func fillGaps(inside []pair, oldFrom, newFrom int, end pair) []pair {
	var pairs []pair
	for _, p := range append(inside, end) {
		for ; oldFrom < p.old; oldFrom++ {
			pairs = append(pairs, pair{oldFrom, -1})
		}
		for ; newFrom < p.new; newFrom++ {
			pairs = append(pairs, pair{-1, newFrom})
		}
		if p != end {
			pairs = append(pairs, p)
		}
		oldFrom, newFrom = p.old+1, p.new+1
	}
	return pairs
}

type step byte

const (
	stepUp step = iota
	stepLeft
	stepDiagonal
)

// fullAlign aligns the cues oldFrom to oldTo-1 with the cues newFrom to newTo-1. A left step is an added cue, an up
// step a removed cue. On a tie the left step wins over the up step, and a diagonal step needs a gain above epsilon.
// So removed cues come before added ones.
func (c *comparer) fullAlign(oldFrom, oldTo, newFrom, newTo int) []pair {
	n, m := oldTo-oldFrom, newTo-newFrom
	steps := c.alignTable(oldFrom, newFrom, n, m)
	pairs := make([]pair, 0, max(n, m))
	for i, j := n, m; i > 0 || j > 0; {
		direction := stepLeft
		switch {
		case i == 0:
		case j == 0:
			direction = stepUp
		default:
			direction = steps[i*(m+1)+j]
		}
		switch direction {
		case stepDiagonal:
			i, j = i-1, j-1
			pairs = append(pairs, pair{oldFrom + i, newFrom + j})
		case stepUp:
			i--
			pairs = append(pairs, pair{oldFrom + i, -1})
		case stepLeft:
			j--
			pairs = append(pairs, pair{-1, newFrom + j})
		}
	}
	slices.Reverse(pairs)
	return pairs
}

// alignTable returns the best step of each cell of the (n+1) by (m+1) table, row by row.
func (c *comparer) alignTable(oldFrom, newFrom, n, m int) []step {
	steps := make([]step, (n+1)*(m+1))
	previous := make([]float64, m+1)
	current := make([]float64, m+1)
	for i := 1; i <= n; i++ {
		current[0] = 0
		row := steps[i*(m+1) : (i+1)*(m+1)]
		for j := 1; j <= m; j++ {
			best, direction := current[j-1], stepLeft
			if previous[j] > best {
				best, direction = previous[j], stepUp
			}
			if w := c.weight(oldFrom+i-1, newFrom+j-1); w > 0 && previous[j-1]+w > best+epsilon {
				best, direction = previous[j-1]+w, stepDiagonal
			}
			current[j] = best
			row[j] = direction
		}
		previous, current = current, previous
	}
	return steps
}
