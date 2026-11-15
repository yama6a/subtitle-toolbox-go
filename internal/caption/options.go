package caption

import (
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Validate returns an error that matches model.ErrInvalidArgument for a channel other than 0, 1 and 2, and for
// an unknown roll-up mode. name is the format in the option path and the message, "SCC" or "MCC".
func (o CEA608Options) Validate(name string) error {
	if o.Channel < 0 || o.Channel > 2 {
		return &model.InvalidArgumentError{
			Option: "ReadOptions." + name + ".Channel",
			Msg:    "The " + name + " data channel must be 1 or 2, got " + strconv.Itoa(o.Channel) + ".",
		}
	}
	switch o.RollUp {
	case "", RollUpScreen, RollUpLines:
		return nil
	default:
		return &model.InvalidArgumentError{
			Option: "ReadOptions." + name + ".RollUp",
			Msg:    "The " + name + " roll-up mode " + strconv.Quote(string(o.RollUp)) + " is not screen or lines.",
		}
	}
}

// ValidateDropFrame returns an error that matches model.ErrInvalidArgument for an unknown drop-frame mode.
func ValidateDropFrame(m model.DropFrameMode, name string) error {
	if m > model.DropFrameOff {
		return &model.InvalidArgumentError{
			Option: "WriteOptions." + name + ".DropFrame",
			Msg:    "The " + name + " drop-frame mode " + strconv.Itoa(int(m)) + " is unknown.",
		}
	}
	return nil
}

// DropFrame applies m to the "dropFrame" value that a reader stored in file. DropFrameKeep takes the stored
// value, else true.
func DropFrame(m model.DropFrameMode, file *model.Data) bool {
	switch m {
	case model.DropFrameOn:
		return true
	case model.DropFrameOff:
		return false
	case model.DropFrameKeep:
	}
	stored, ok := file.Bool("dropFrame")
	return stored || !ok
}
