// Package wordgroup holds the word helpers that the transcript readers and writers share.
// A reader groups the timed words of a speech-to-text service into cues and fills Cue.Words.
// A writer finds the source words that a cue still holds, so it keeps their ids and confidences.
package wordgroup

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

// Options holds the read settings that the helpers use.
type Options struct {
	WordTimestamps bool // fill Cue.Words from the word times of the source
	SpeakerVoices  bool // set Cue.Speaker from the speaker of the source
}

// Word is one word of a source transcript as a reader reads it.
type Word struct {
	Text       string // trimmed
	Start      time.Duration
	Timed      bool // false: the source gives no valid start, so the word gets no entry in Cue.Words
	End        time.Duration
	HasEnd     bool // false: the end comes from the next start, and Group and CuesFromWords take the start as the end
	Speaker    string
	HasSpeaker bool                // a missing speaker and an empty one differ, as in PHP
	Confidence *float64            // nil when the source gives none or a value outside 0 to 1
	Source     []orderedjson.Value // the raw source objects of the word
}

func (w Word) end() time.Duration {
	if w.HasEnd {
		return w.End
	}
	return w.Start
}
