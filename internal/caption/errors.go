package caption

import "errors"

// The sentinels of CEA-608 writers. A writer returns them as the Err of a model.UnwritableError, and the caller
// maps them to the edit that the cue needs first.
var (
	// ErrWrapFirst means the cue fits 4 lines of 32 characters after its lines are wrapped.
	ErrWrapFirst = errors.New("wrap the lines to 4 lines of 32 characters first")
	// ErrSplitFirst means the cue needs splitting into cues that fit 4 lines of 32 characters first.
	ErrSplitFirst = errors.New("split the cue into cues of 4 lines of 32 characters, then wrap the lines")
)
