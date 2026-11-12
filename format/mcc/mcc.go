// Package mcc reads and writes MCC (MacCaption), the caption file of Telestream MacCaption and CaptionMaker.
//
// An MCC file starts with the line "File Format=MacCaption_MCC V2.0" or V1.0. Each data line holds a time code, a tab
// and one SMPTE 291 ancillary data packet in hex. The packet holds a caption distribution packet with CEA-608 byte
// pairs and CEA-708 service data. The extension is .mcc.
package mcc

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

// FitChange is one change that the writer made to a CEA-608 caption.
type FitChange = caption.FitChange

// WriteReport lists the CEA-608 fit changes of a write. The CEA-708 data holds every cue.
type WriteReport struct {
	// Changes holds each change, in cue order.
	Changes []FitChange
}

// ReadOptions holds the settings of the MCC reader.
type ReadOptions struct {
	// Service is the CEA-708 service whose captions win when it has any, 1 to 63. 0 means 1.
	Service int
	// CEA608Only reads only the CEA-608 captions of field 1.
	CEA608Only bool
	// Channel is the CEA-608 data channel. 1 reads CC1 and CC3, 2 reads CC2 and CC4. 0 means 1.
	Channel int
	// RollUp says how roll-up captions become cues. The empty value acts as RollUpScreen.
	RollUp RollUp
}

// Validate returns an error that matches model.ErrInvalidArgument for a service outside 0 to 63, a channel other
// than 0, 1 and 2, and an unknown roll-up mode.
func (o ReadOptions) Validate() error {
	if o.Service < 0 || o.Service > maxService {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.MCC.Service",
			Msg:    "The CEA-708 service must be from 1 to 63, got " + strconv.Itoa(o.Service) + ".",
		}
	}
	if o.Channel < 0 || o.Channel > 2 {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.MCC.Channel",
			Msg:    "The MCC data channel must be 1 or 2, got " + strconv.Itoa(o.Channel) + ".",
		}
	}
	switch o.RollUp {
	case "", RollUpScreen, RollUpLines:
		return nil
	default:
		return &model.InvalidArgumentError{
			Option: "ReadOptions.MCC.RollUp",
			Msg:    "The MCC roll-up mode " + strconv.Quote(string(o.RollUp)) + " is not screen or lines.",
		}
	}
}

// WriteOptions holds the settings of the MCC writer.
type WriteOptions struct {
	// FrameRate is the output rate. It must be within 0.01 of 23.976, 24, 25, 29.97, 30, 50, 59.94 or 60.
	// The zero value takes the stored rate, else 29.97.
	FrameRate model.FrameRate
	// DropFrame selects the time code labels at 29.97 and 59.94 fps. DropFrameKeep takes the drop-frame flag of
	// FrameRate, else the stored mode, else drop-frame.
	DropFrame model.DropFrameMode
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that MCC lacks and for an unknown
// drop-frame mode.
func (o WriteOptions) Validate() error {
	if !o.FrameRate.IsZero() {
		if _, ok := rateNear(o.FrameRate); !ok {
			return &model.InvalidArgumentError{
				Option: "WriteOptions.MCC.FrameRate",
				Msg: "The MCC formatter accepts the frame rates 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60, got " +
					o.FrameRate.String() + ".",
			}
		}
	}
	if o.DropFrame > model.DropFrameOff {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.MCC.DropFrame",
			Msg:    "The MCC drop-frame mode " + strconv.Itoa(int(o.DropFrame)) + " is unknown.",
		}
	}
	return nil
}

const (
	maxService    = 63
	formatDataKey = string(model.MCC)
	versionPrefix = "File Format=MacCaption_MCC V"
)

var signatureRegexp = regexp.MustCompile(`(?m)\A` + versionPrefix + `\d+\.\d+[ \t]*$`)

// Detect reports whether content starts with a line "File Format=MacCaption_MCC V<n>.<n>".
// The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}

// rate is a row of the frame rate table.
type rate struct {
	name     string
	fps      model.FrameRate
	labels   int // labels per second
	code     int // CDP frame rate code of SMPTE 334-2
	triplets int // cc_data triplets per CDP, CEA-708 table 3
}

//nolint:gochecknoglobals // the frame rate table of MCC, read-only
var rates = []rate{
	{"23.976", model.FrameRate23976, 24, 1, 25},
	{"24", model.FrameRate24, 24, 2, 25},
	{"25", model.FrameRate25, 25, 3, 24},
	{"29.97", model.FrameRate2997, 30, 4, 20},
	{"30", model.FrameRate30, 30, 5, 20},
	{"50", model.FrameRate50, 50, 6, 12},
	{"59.94", model.FrameRate5994, 60, 7, 10},
	{"60", model.FrameRate60, 60, 8, 10},
}

func rateNear(r model.FrameRate) (rate, bool) {
	for _, row := range rates {
		if row.fps.Near(r) {
			return row, true
		}
	}
	return rate{}, false
}

func rateNamed(name string) (rate, bool) {
	for _, row := range rates {
		if row.name == name {
			return row, true
		}
	}
	return rate{}, false
}

// headerText is the notice that the MCC format asks every file to hold. The first %s is the version and the second
// the list of time code rates.
//
// Copied from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
const headerText = `File Format=MacCaption_MCC V%s

///////////////////////////////////////////////////////////////////////////////////
// Computer Prompting and Captioning Company
// Ancillary Data Packet Transfer File
//
// Permission to generate this format is granted provided that
//   1. This ANC Transfer file format is used on an as-is basis and no warranty is given, and
//   2. This entire descriptive information text is included in a generated .mcc file.
//
// General file format:
//   HH:MM:SS:FF(tab)[Hexadecimal ANC data in groups of 2 characters]
//     Hexadecimal data starts with the Ancillary Data Packet DID (Data ID defined in S291M)
//       and concludes with the Check Sum following the User Data Words.
//     Each time code line must contain at most one complete ancillary data packet.
//     To transfer additional ANC Data successive lines may contain identical time code.
//     Time Code Rate=[%s]
//
//   ANC data bytes may be represented by one ASCII character according to the following schema:
//     G  FAh 00h 00h
//     H  2 x (FAh 00h 00h)
//     I  3 x (FAh 00h 00h)
//     J  4 x (FAh 00h 00h)
//     K  5 x (FAh 00h 00h)
//     L  6 x (FAh 00h 00h)
//     M  7 x (FAh 00h 00h)
//     N  8 x (FAh 00h 00h)
//     O  9 x (FAh 00h 00h)
//     P  FBh 80h 80h
//     Q  FCh 80h 80h
//     R  FDh 80h 80h
//     S  96h 69h
//     T  61h 01h
//     U  E1h 00h 00h 00h
//     Z  00h
//
///////////////////////////////////////////////////////////////////////////////////`
