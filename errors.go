package stbox

import "github.com/yama6a/subtitle-toolbox-go/v5/model"

// The sentinel errors of package model. errors.Is matches both names.
var (
	ErrParse             = model.ErrParse
	ErrUnsupportedFormat = model.ErrUnsupportedFormat
	ErrNoFormatDetected  = model.ErrNoFormatDetected
	ErrInvalidArgument   = model.ErrInvalidArgument
	ErrCueNotFound       = model.ErrCueNotFound
	ErrUnwritable        = model.ErrUnwritable
	ErrInvalidUTF8       = model.ErrInvalidUTF8
)
