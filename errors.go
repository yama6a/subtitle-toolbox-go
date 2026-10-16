package stbox

import "github.com/yama6a/subtitle-toolbox-go/model"

// The sentinel errors of package model. errors.Is matches both names.
var (
	ErrParse             = model.ErrParse
	ErrUnsupportedFormat = model.ErrUnsupportedFormat
	ErrUnknownFormat     = model.ErrUnknownFormat
	ErrInvalidArgument   = model.ErrInvalidArgument
	ErrCueNotFound       = model.ErrCueNotFound
	ErrUnwritable        = model.ErrUnwritable
	ErrInvalidUTF8       = model.ErrInvalidUTF8
)
