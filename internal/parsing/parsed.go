package parsing

import (
	"cmp"
	"slices"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// ParsedComment is a comment and the count of cues before it in the file.
type ParsedComment struct {
	Text string
	// CuesBefore counts the cues of the file before the comment that the reader keeps.
	CuesBefore int
}

// AddParsed adds cues, in file order, to s, which holds no cues yet. Each comment stays before the cue
// that followed it in the file, also when that cue moves in the start order.
func AddParsed(s *model.Subtitle, cues []model.Cue, comments []ParsedComment) {
	order := make([]int, len(cues))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Compare(cues[a].Start.Round(time.Millisecond), cues[b].Start.Round(time.Millisecond))
	})
	sorted := make([]model.Cue, len(cues))
	position := make([]int, len(cues))
	for i, fileIndex := range order {
		sorted[i] = cues[fileIndex]
		position[fileIndex] = i
	}
	s.AddCues(sorted...)
	for _, c := range comments {
		before := len(cues)
		if c.CuesBefore < len(cues) {
			before = position[c.CuesBefore]
		}
		_ = s.AddComment(c.Text, before) // before is never negative, and AddComment fails only for that
	}
}
