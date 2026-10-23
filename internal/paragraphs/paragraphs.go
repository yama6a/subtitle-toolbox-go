// Package paragraphs groups timed items into paragraphs. The plain text writer uses it, and the HTML transcript
// writer of issue #101 will use it too.
package paragraphs

import "time"

// Item is a value with the time range of its cue.
type Item[V any] struct {
	Start, End time.Duration
	Value      V
}

// Paragraph holds the values of one paragraph and the start of its first item.
type Paragraph[V any] struct {
	Start  time.Duration
	Values []V
}

// ByGap groups items, which are in start order, into paragraphs. An item starts a new paragraph when it is the
// first, when its start minus the latest end so far is gap or more, or when startsNew(previous, value) is true.
// previous is the value of the item before it. A nil startsNew never starts one.
func ByGap[V any](items []Item[V], gap time.Duration, startsNew func(previous, value V) bool) []Paragraph[V] {
	var out []Paragraph[V]
	var latestEnd time.Duration
	var previous V
	for i, it := range items {
		if i == 0 || it.Start-latestEnd >= gap || startsNew != nil && startsNew(previous, it.Value) {
			out = append(out, Paragraph[V]{Start: it.Start})
		}
		last := &out[len(out)-1]
		last.Values = append(last.Values, it.Value)
		if i == 0 {
			latestEnd = it.End
		}
		latestEnd = max(latestEnd, it.End)
		previous = it.Value
	}
	return out
}
