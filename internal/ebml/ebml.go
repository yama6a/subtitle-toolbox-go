// Package ebml reads EBML elements (RFC 8794) from an io.ReaderAt.
package ebml

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
)

// UnknownSize is the Size of an element whose size bits are all 1.
const UnknownSize = -1

// maxHeaderLength is an element ID of 4 bytes plus a data size of 8 bytes.
const maxHeaderLength = 12

// Error is damaged element structure, for example a size that passes the end of the file.
type Error struct {
	Msg string
}

// Error returns the message.
func (e *Error) Error() string { return e.Msg }

func errorf(format string, args ...any) error {
	return &Error{Msg: fmt.Sprintf(format, args...)}
}

// Header is the header of one element.
type Header struct {
	ID     uint32
	Size   int64 // UnknownSize when every size bit is 1
	Offset int64 // the first data byte
}

// ReadVint decodes a variable-length integer. keepMarker keeps the length bit, as element IDs do.
func ReadVint(b []byte, keepMarker bool) (value uint64, length int, ok bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false
	}
	length = 1
	for uint(b[0])&(0x100>>length) == 0 {
		length++
	}
	if len(b) < length {
		return 0, 0, false
	}
	value = uint64(b[0])
	if !keepMarker {
		value &= uint64(0x100>>length) - 1
	}
	for _, c := range b[1:length] {
		value = value<<8 | uint64(c)
	}
	return value, length, true
}

// Reader reads elements from the first size bytes of an io.ReaderAt.
type Reader struct {
	r    io.ReaderAt
	size int64
}

// NewReader returns a Reader over the first size bytes of r.
func NewReader(r io.ReaderAt, size int64) *Reader {
	return &Reader{r: r, size: size}
}

// Header reads the header of the element at byte at. It returns false at the end of the file.
func (r *Reader) Header(at int64) (Header, bool, error) {
	if at < 0 || at >= r.size {
		return Header{}, false, nil
	}
	buf := make([]byte, min(maxHeaderLength, r.size-at))
	n, err := r.r.ReadAt(buf, at)
	if err != nil && !errors.Is(err, io.EOF) {
		return Header{}, false, fmt.Errorf("read EBML header at byte %d: %w", at, err)
	}
	buf = buf[:n]
	if n == 0 {
		return Header{}, false, nil
	}
	id, idLen, ok := ReadVint(buf, true)
	if !ok {
		return Header{}, false, errorf("The EBML element header at byte %d is not valid.", at)
	}
	size, sizeLen, ok := ReadVint(buf[idLen:], false)
	if !ok {
		return Header{}, false, errorf("The EBML element header at byte %d is not valid.", at)
	}
	h := Header{ID: uint32(id), Size: int64(size), Offset: at + int64(idLen+sizeLen)}
	if idLen > 4 {
		h.ID = 0 // longer than any ID of the Matroska specification, so it never matches
	}
	if size == 1<<(7*sizeLen)-1 {
		h.Size = UnknownSize
	}
	return h, true, nil
}

// Bytes reads the data of h. It checks the size against the file size before it allocates.
func (r *Reader) Bytes(h Header) ([]byte, error) {
	if h.Size == UnknownSize {
		return nil, errorf("The element 0x%X at byte %d has an unknown size, which only Segment and Cluster may have.", h.ID, h.Offset)
	}
	if h.Size > r.size-h.Offset {
		return nil, errorf("The element data at byte %d needs %d bytes, but the file ends before.", h.Offset, h.Size)
	}
	buf := make([]byte, h.Size)
	n, err := r.r.ReadAt(buf, h.Offset)
	if n < len(buf) {
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("read EBML data at byte %d: %w", h.Offset, err)
		}
		return nil, errorf("The element data at byte %d needs %d bytes, but the file ends before.", h.Offset, h.Size)
	}
	return buf, nil
}

// Unsigned reads the data of h as a big endian unsigned integer. The value must be below 2^63.
func (r *Reader) Unsigned(h Header) (uint64, error) {
	if h.Size > 8 || h.Size < 0 {
		return 0, errorf("The unsigned integer of the element 0x%X at byte %d is longer than 8 bytes or not below 2^63.", h.ID, h.Offset)
	}
	b, err := r.Bytes(h)
	if err != nil {
		return 0, err
	}
	if len(b) == 8 && b[0] >= 0x80 {
		return 0, errorf("The unsigned integer of the element 0x%X at byte %d is longer than 8 bytes or not below 2^63.", h.ID, h.Offset)
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

// String reads the data of h without trailing NUL bytes.
func (r *Reader) String(h Header) (string, error) {
	b, err := r.Bytes(h)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\x00"), nil
}

// Children yields the child elements of parent. A child that does not fit into parent ends the iteration with an error.
func (r *Reader) Children(parent Header) iter.Seq2[Header, error] {
	return func(yield func(Header, error) bool) {
		if parent.Size == UnknownSize {
			yield(Header{}, errorf("The element 0x%X at byte %d has an unknown size, which only Segment and Cluster may have.", parent.ID, parent.Offset))
			return
		}
		end := parent.Offset + parent.Size
		for pos := parent.Offset; pos < end; {
			h, ok, err := r.Header(pos)
			if err == nil && (!ok || h.Size == UnknownSize || h.Offset+h.Size > end) {
				err = errorf("The element at byte %d does not fit into its parent element.", pos)
			}
			if err != nil {
				yield(Header{}, err)
				return
			}
			if !yield(h, nil) {
				return
			}
			pos = h.Offset + h.Size
		}
	}
}
