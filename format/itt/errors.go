package itt

import (
	"errors"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// ErrNeedsFrameRate matches the error of a write that has no frame rate in WriteOptions.FrameRate and none stored
// by the reader. A rate that iTT lacks does not match it.
var ErrNeedsFrameRate = errors.New("iTT output needs the frame rate of the video")

type needsFrameRateError struct{ *model.InvalidArgumentError }

func (e needsFrameRateError) Is(target error) bool {
	return target == ErrNeedsFrameRate || e.InvalidArgumentError.Is(target)
}

func (e needsFrameRateError) Unwrap() error { return e.InvalidArgumentError }
