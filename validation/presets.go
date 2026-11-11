package validation

import (
	"errors"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// The numbers of the presets that have no constant in package model.
const (
	// NetflixMaxCharactersPerSecond is the reading speed for adult programs in the Netflix English (USA) Timed Text
	// Style Guide. The guide gives 17 for children's programs, which no preset uses.
	NetflixMaxCharactersPerSecond = 20
	// BBCMaxCharactersPerLine is the broadcast line length of section 3.1 of the BBC Subtitle Guidelines.
	BBCMaxCharactersPerLine = 37
	// BBCMaxWordsPerMinute is the upper end of 160 to 180 words per minute in section 4 of the BBC Subtitle Guidelines.
	BBCMaxWordsPerMinute = 180
	// BBCMinDurationPerWord is 0.3 s per word, from section 4.1 of the BBC Subtitle Guidelines.
	BBCMinDurationPerWord = 300 * time.Millisecond
)

// netflixMinDuration is 20 frames at 24 fps. The check rounds it to 833 ms.
const netflixMinDuration = 5 * time.Second / 6

// NetflixEnglish returns the limits of the Netflix English (USA) Timed Text Style Guide for adult programs,
// with the 2-frame gap at the video frame rate r.
//
// The numbers and their sources:
//   - MaxCharactersPerSecond 20: English (USA) Timed Text Style Guide, reading speed for adult programs.
//   - MaxCharactersPerLine 42 (model.DefaultMaxCharactersPerLine): the same guide, character limitation.
//   - MaxLinesPerCue 2 (model.DefaultMaxLinesPerCue): Timed Text Style Guide, General Requirements.
//   - MinDuration 5/6 s, which is 20 frames at 24 fps: General Requirements, minimum duration of an event.
//   - MaxDuration 7 s (model.DefaultMaxDuration): General Requirements, maximum duration of an event.
//   - MinGap 2 frames, r.Duration(2): Subtitle Timing Guidelines, the minimum gap between events.
//   - NoOverlap on, because MinGap checks only gaps of 0 or more.
//
// A zero or invalid r returns an *model.InvalidArgumentError with Option "frameRate". A caller may change the
// fields of the result before Check. A number of this preset changes only in the next major release, or in a
// patch release that fixes a number that differs from its source.
func NetflixEnglish(r model.FrameRate) (Rules, error) {
	if err := r.Validate(); err != nil {
		msg := "The frame rate of the Netflix gap rule must be greater than 0."
		if e, ok := errors.AsType[*model.InvalidArgumentError](err); ok && r.Num > 0 && r.Den > 0 {
			msg = e.Msg
		}
		return Rules{}, &model.InvalidArgumentError{Option: "frameRate", Msg: msg}
	}
	return Rules{
		MaxCharactersPerSecond: NetflixMaxCharactersPerSecond,
		MaxCharactersPerLine:   model.DefaultMaxCharactersPerLine,
		MaxLinesPerCue:         model.DefaultMaxLinesPerCue,
		MinDuration:            netflixMinDuration,
		MaxDuration:            model.DefaultMaxDuration,
		MinGap:                 r.Duration(2),
		NoOverlap:              true,
	}, nil
}

// BBC returns the line length and reading speed limits of the BBC Subtitle Guidelines.
//
// The numbers and their sources:
//   - MaxCharactersPerLine 37: section 3.1, the broadcast line length.
//   - MaxWordsPerMinute 180: section 4, the upper end of 160 to 180 words per minute.
//   - MinDurationPerWord 300 ms: section 4.1, 0.3 s per word.
//
// 0.3 s per word allows 200 words per minute, so a cue can break MaxWordsPerMinute and still meet
// MinDurationPerWord. Characters per second, line count, durations and gaps stay off. A number of this preset
// changes only in the next major release, or in a patch release that fixes a number that differs from its source.
func BBC() Rules {
	return Rules{
		MaxCharactersPerLine: BBCMaxCharactersPerLine,
		MaxWordsPerMinute:    BBCMaxWordsPerMinute,
		MinDurationPerWord:   BBCMinDurationPerWord,
	}
}

// Structure returns the checks of a well-formed cue list: at least one cue, no negative duration, no overlap.
// These are the cue list invariants of model.Subtitle.
func Structure() Rules {
	return Rules{RequireCues: true, NoNegativeDuration: true, NoOverlap: true}
}
