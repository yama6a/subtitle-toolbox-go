package testfiles

// EditDistance returns the number of single-rune inserts, deletes and replacements that turn a into b.
func EditDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ra := range a {
		cur := make([]int, len(b)+1)
		cur[0] = i + 1
		for j, rb := range b {
			cost := 1
			if ra == rb {
				cost = 0
			}
			cur[j+1] = min(prev[j+1]+1, cur[j]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
