package model_test

import (
	"slices"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	opAddCue = iota
	opSetCue
	opRemoveCue
	opAddComment
	opRemoveCuesWhere
	opCount
)

// FuzzSubtitleOps reads each 3 bytes of the input as one operation and checks the invariants after each step.
func FuzzSubtitleOps(f *testing.F) {
	f.Add([]byte{opAddCue, 5, 1, opAddCue, 1, 1, opAddComment, 1, 0, opSetCue, 0, 9, opRemoveCue, 1, 0})
	f.Add([]byte{opAddComment, 3, 0, opAddCue, 2, 0, opAddCue, 2, 1, opRemoveCuesWhere, 2, 0, opAddComment, 0, 0})
	f.Add([]byte{opAddCue, 0, 0, opAddCue, 0, 0, opSetCue, 1, 0, opSetCue, 0, 255, opRemoveCuesWhere, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		s := &model.Subtitle{}
		for step := 0; step+2 < len(data); step += 3 {
			op, a, b := data[step]%opCount, int(data[step+1]), int(data[step+2])
			at := time.Duration(a-64) * 100 * time.Millisecond
			switch op {
			case opAddCue:
				s.AddCue(model.Cue{Start: at, End: at + time.Duration(b)*time.Millisecond})
			case opSetCue:
				if s.Len() > 0 {
					_ = s.SetCue(b%s.Len(), model.Cue{Start: at, End: at + time.Second})
				}
			case opRemoveCue:
				_ = s.RemoveCue(a % (s.Len() + 1))
			case opAddComment:
				_ = s.AddComment("c", a-8)
			case opRemoveCuesWhere:
				s.RemoveCuesWhere(func(c model.Cue) bool { return c.Start/time.Second%time.Duration(b%4+2) == 0 })
			}

			cueStarts := starts(s)
			if !slices.IsSorted(cueStarts) {
				t.Fatalf("step %d: cues out of start order: %v", step/3, cueStarts)
			}
			comments := s.Comments()
			if !slices.IsSortedFunc(comments, func(x, y model.Comment) int { return x.BeforeCue - y.BeforeCue }) {
				t.Fatalf("step %d: comments out of order: %v", step/3, comments)
			}
			for _, c := range comments {
				if c.BeforeCue < 0 {
					t.Fatalf("step %d: comment with a negative index: %v", step/3, comments)
				}
			}
		}
	})
}
