package paragraphs_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/paragraphs"
)

func item(start, end time.Duration, value string) paragraphs.Item[string] {
	return paragraphs.Item[string]{Start: start, End: end, Value: value}
}

func TestByGap(t *testing.T) {
	t.Parallel()

	const s = time.Second
	tests := []struct {
		name  string
		items []paragraphs.Item[string]
		gap   time.Duration
		want  []paragraphs.Paragraph[string]
	}{
		{name: "no items", gap: 2 * s},
		{
			name:  "one item",
			items: []paragraphs.Item[string]{item(s, 2*s, "a")},
			gap:   2 * s,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a"}}},
		},
		{
			name:  "a gap below the limit joins",
			items: []paragraphs.Item[string]{item(s, 2*s, "a"), item(3*s-time.Millisecond, 4*s, "b")},
			gap:   2 * s,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a", "b"}}},
		},
		{
			name:  "a gap equal to the limit starts a paragraph",
			items: []paragraphs.Item[string]{item(s, 2*s, "a"), item(4*s, 5*s, "b")},
			gap:   2 * s,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a"}}, {Start: 4 * s, Values: []string{"b"}}},
		},
		{
			name:  "gap 0 starts a paragraph when an item starts at the latest end",
			items: []paragraphs.Item[string]{item(s, 2*s, "a"), item(2*s, 3*s, "b"), item(2500*time.Millisecond, 3500*time.Millisecond, "c")},
			gap:   0,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a"}}, {Start: 2 * s, Values: []string{"b", "c"}}},
		},
		{
			name:  "gap 0 starts a paragraph at equal starts of items without length",
			items: []paragraphs.Item[string]{item(s, s, "a"), item(s, s, "b")},
			gap:   0,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a"}}, {Start: s, Values: []string{"b"}}},
		},
		{
			name:  "equal starts join below the limit",
			items: []paragraphs.Item[string]{item(s, 2*s, "a"), item(s, 2*s, "b")},
			gap:   time.Millisecond,
			want:  []paragraphs.Paragraph[string]{{Start: s, Values: []string{"a", "b"}}},
		},
		{
			name:  "the gap counts from the latest end",
			items: []paragraphs.Item[string]{item(0, 10*s, "Long"), item(s, 2*s, "Short"), item(11*s, 12*s, "Overlapped")},
			gap:   2 * s,
			want:  []paragraphs.Paragraph[string]{{Start: 0, Values: []string{"Long", "Short", "Overlapped"}}},
		},
		{
			name:  "the largest gap never splits",
			items: []paragraphs.Item[string]{item(0, s, "a"), item(100000*time.Hour, 100001*time.Hour, "b")},
			gap:   time.Duration(1<<63 - 1),
			want:  []paragraphs.Paragraph[string]{{Start: 0, Values: []string{"a", "b"}}},
		},
		{
			name:  "a negative start",
			items: []paragraphs.Item[string]{item(-2*s, -s, "a"), item(0, s, "b")},
			gap:   s,
			want:  []paragraphs.Paragraph[string]{{Start: -2 * s, Values: []string{"a"}}, {Start: 0, Values: []string{"b"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, paragraphs.ByGap(tt.items, tt.gap, nil))
		})
	}
}

func TestByGapStartsNew(t *testing.T) {
	t.Parallel()

	const s = time.Second
	items := []paragraphs.Item[string]{item(0, s, "Anna: hi"), item(s, 2*s, "Anna: there"), item(2*s, 3*s, "Tom: hello"), item(3*s, 4*s, "Tom: you")}
	speaker := func(v string) string { return v[:4] }
	var calls [][2]string
	got := paragraphs.ByGap(items, 5*s, func(previous, value string) bool {
		calls = append(calls, [2]string{previous, value})
		return speaker(previous) != speaker(value)
	})
	assert.Equal(t, []paragraphs.Paragraph[string]{
		{Start: 0, Values: []string{"Anna: hi", "Anna: there"}},
		{Start: 2 * s, Values: []string{"Tom: hello", "Tom: you"}},
	}, got)
	assert.Equal(t, [][2]string{
		{"Anna: hi", "Anna: there"}, {"Anna: there", "Tom: hello"}, {"Tom: hello", "Tom: you"},
	}, calls, "the hook runs for each item after the first, with the item before it")
}

func TestByGapStartsNewAddsToTheGap(t *testing.T) {
	t.Parallel()

	const s = time.Second
	items := []paragraphs.Item[string]{item(0, s, "a"), item(s, 2*s, "b"), item(10*s, 11*s, "c")}
	got := paragraphs.ByGap(items, 2*s, func(_, value string) bool { return value == "b" })
	assert.Equal(t, []paragraphs.Paragraph[string]{
		{Start: 0, Values: []string{"a"}}, {Start: s, Values: []string{"b"}}, {Start: 10 * s, Values: []string{"c"}},
	}, got)
}
