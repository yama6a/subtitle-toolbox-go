package diff

// pair is an old and a new cue index. -1 marks the side without a partner.
type pair struct{ old, new int }

// fenwickEntry is the best chain score at a prefix of the new indexes, and the candidate that ends the chain.
type fenwickEntry struct {
	score float64
	cand  int
}

// heaviestChain returns the candidates, rising in both indexes, with the largest sum of weights. A Fenwick tree
// over the new index holds the best chain that ends before each index. A tie keeps the entry found first.
func (c *comparer) heaviestChain(candidates []candidate) []pair {
	size := len(c.newTexts)
	tree := make([]fenwickEntry, size+1)
	for k := range tree {
		tree[k].cand = -1
	}
	scores := make([]float64, len(candidates))
	previous := make([]int, len(candidates))
	best := fenwickEntry{cand: -1}
	for n, cand := range candidates {
		before := fenwickEntry{cand: -1}
		for k := cand.new; k > 0; k -= k & -k {
			if tree[k].score > before.score {
				before = tree[k]
			}
		}
		scores[n] = before.score + cand.weight
		previous[n] = before.cand
		for k := cand.new + 1; k <= size; k += k & -k {
			if scores[n] > tree[k].score {
				tree[k] = fenwickEntry{score: scores[n], cand: n}
			}
		}
		if scores[n] > best.score {
			best = fenwickEntry{score: scores[n], cand: n}
		}
	}
	var chain []pair
	for n := best.cand; n >= 0; n = previous[n] {
		chain = append(chain, pair{candidates[n].old, candidates[n].new})
	}
	for l, r := 0, len(chain)-1; l < r; l, r = l+1, r-1 {
		chain[l], chain[r] = chain[r], chain[l]
	}
	return chain
}
