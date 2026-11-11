// Package ebustl reads EBU STL, the binary subtitle exchange format of EBU Tech 3264.
package ebustl

import (
	"errors"
	"io"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// ReadOptions holds the settings of the EBU STL reader.
type ReadOptions struct {
	// SubtractStartOfProgramme subtracts the GSI start of programme (TCP) from every cue time.
	SubtractStartOfProgramme bool
}

// Read reads an EBU STL file from its raw bytes.
func Read(io.Reader, model.CommonReadOptions, ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	return nil, nil, errors.New("not implemented")
}
