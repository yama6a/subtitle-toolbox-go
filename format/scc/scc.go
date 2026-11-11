// Package scc reads Scenarist Closed Captions (SCC), the delivery format of US broadcast captions.
//
// An SCC file starts with the line "Scenarist_SCC V1.0". Each later line holds a SMPTE time code and CEA-608
// byte pairs, one pair per frame at 29.97 fps. A semicolon before the frames marks drop-frame time code.
// The extension is .scc.
package scc

import (
	"regexp"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
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
	if o.Channel < 0 || o.Channel > 2 {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.SCC.Channel",
			Msg:    "The SCC data channel must be 1 or 2, got " + strconv.Itoa(o.Channel) + ".",
		}
	}
	switch o.RollUp {
	case "", RollUpScreen, RollUpLines:
		return nil
	default:
		return &model.InvalidArgumentError{
			Option: "ReadOptions.SCC.RollUp",
			Msg:    "The SCC roll-up mode " + strconv.Quote(string(o.RollUp)) + " is not screen or lines.",
		}
	}
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
