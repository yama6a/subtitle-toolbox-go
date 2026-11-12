package mcc

import "github.com/yama6a/subtitle-toolbox-go/v3/internal/caption"

// The sentinels that a cue the writer rejects holds as the Err of its *model.UnwritableError. A caller maps them to
// the edit that the cue needs first.
var (
	// ErrWrapFirst means the cue fits 4 lines of 32 characters after its lines are wrapped.
	ErrWrapFirst = caption.ErrWrapFirst
	// ErrSplitFirst means the cue needs splitting into cues that fit 4 lines of 32 characters first.
	ErrSplitFirst = caption.ErrSplitFirst
)
