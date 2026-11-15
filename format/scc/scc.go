// Package scc reads Scenarist Closed Captions (SCC), the delivery format of US broadcast captions.
//
// An SCC file starts with the line "Scenarist_SCC V1.0". Each later line holds a SMPTE time code and CEA-608
// byte pairs, one pair per frame at 29.97 fps. A semicolon before the frames marks drop-frame time code.
// The extension is .scc.
package scc

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// RollUp says how the reader turns roll-up captions into cues. The empty value acts as RollUpScreen.
type RollUp = caption.RollUp

// The roll-up modes.
const (
	// RollUpScreen gives one cue per screen of roll-up captions.
	RollUpScreen = caption.RollUpScreen
	// RollUpLines gives one cue per row, from its first character until it rolls up or the screen clears.
	RollUpLines = caption.RollUpLines
)

// ReadOptions holds the settings of the SCC reader.
type ReadOptions struct {
	// Channel is the CEA-608 data channel. 1 reads CC1 and CC3, 2 reads CC2 and CC4. 0 means 1.
	Channel int
	// RollUp says how roll-up captions become cues. The empty value acts as RollUpScreen.
	RollUp RollUp
}

// Validate returns an error that matches model.ErrInvalidArgument for a channel other than 0, 1 and 2,
// and for an unknown roll-up mode.
func (o ReadOptions) Validate() error {
	return caption.CEA608Options{Channel: o.Channel, RollUp: o.RollUp}.Validate("SCC")
}

const (
	header        = "Scenarist_SCC V1.0"
	formatDataKey = string(model.SCC)
)

var signatureRegexp = regexp.MustCompile(`(?m)\AScenarist_SCC V1\.0[ \t]*$`)

// Detect reports whether content starts with the line "Scenarist_SCC V1.0", with optional spaces or tabs after it.
// The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}

// WriteOptions holds the settings of the SCC writer.
type WriteOptions struct {
	// DropFrame selects the time code labels. DropFrameKeep takes the drop-frame mode that the reader stored, else drop-frame.
	DropFrame model.DropFrameMode
	// Fit changes the text and the timing that SCC cannot hold, in place of failing. The report lists each change.
	Fit bool
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown drop-frame mode.
func (o WriteOptions) Validate() error {
	return caption.ValidateDropFrame(o.DropFrame, "SCC")
}

// FitAction is the kind of a change that Fit made. The values are "transliterated", "wrapped", "delayed" and "dropped".
type FitAction = caption.FitAction

// FitChange is one change that Fit made to a cue.
type FitChange = caption.FitChange

// WriteReport lists the changes of a write.
type WriteReport struct {
	// Changes holds each change of Fit, in cue order. It is empty without Fit.
	Changes []FitChange
}

// The fit actions.
const (
	Transliterated = caption.Transliterated
	Wrapped        = caption.Wrapped
	Delayed        = caption.Delayed
	Dropped        = caption.Dropped
)
