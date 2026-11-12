package mkv

import (
	"errors"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ebml"
)

// errStop ends an each loop without an error.
var errStop = errors.New("stop")

// each calls visit for each child of parent. A visit that returns errStop ends the loop without an error.
func (f *File) each(parent ebml.Header, visit func(ebml.Header) error) error {
	for c, err := range f.r.Children(parent) {
		if err != nil {
			return err
		}
		if err := visit(c); err != nil {
			if errors.Is(err, errStop) {
				return nil
			}
			return err
		}
	}
	return nil
}

func (f *File) readUint(h ebml.Header, dst *uint64) error {
	v, err := f.r.Unsigned(h)
	*dst = v
	return err
}

func (f *File) readString(h ebml.Header, dst *string) error {
	s, err := f.r.String(h)
	*dst = s
	return err
}

func (f *File) readBytes(h ebml.Header, dst *[]byte) error {
	b, err := f.r.Bytes(h)
	*dst = b
	return err
}
