package pgs

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/displayset"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const headerLength = 13

// Read reads a PGS (.sup) stream. Each display set that shows objects becomes one image cue.
// The reader uses Lenient and LastCueDuration of common.
//
// Strict mode fails on the first damage. Lenient mode keeps the cues before damage in the segment framing, which is
// usually the end of a cut-off file, and records a Skipped warning. Damage inside a complete segment skips only its
// display set, see the package displayset.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	d := displayset.NewDecoder(common)
	br := bufio.NewReader(r)
	data := make([]byte, 1<<16)
	cutOff := false
	for offset := 0; ; {
		var header [headerLength]byte
		size, msg, err := readSegment(br, offset, header[:], data)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if msg != "" {
			if err := d.Skip(&model.ParseError{Format: model.PGS, Msg: msg}); err != nil {
				return nil, nil, err
			}
			cutOff = true
			break
		}
		if err := d.Add(int64(binary.BigEndian.Uint32(header[2:])), header[10], data[:size]); err != nil {
			return nil, nil, err
		}
		offset += headerLength + size
	}
	cues, err := d.Finish(cutOff)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	parsing.AddParsed(s, cues, nil)
	return s, d.Warnings(), nil
}

// readSegment reads the header into header and the data of the segment at offset into data. It returns the size of the
// data, or the message of the framing damage, or io.EOF when no byte is left. The checks run in the order of PHP.
func readSegment(r io.Reader, offset int, header, data []byte) (size int, msg string, err error) {
	if msg, err = readHeader(r, offset, header); msg != "" || err != nil {
		return 0, msg, err
	}
	size = int(binary.BigEndian.Uint16(header[11:]))
	n, err := io.ReadFull(r, data[:size])
	if err != nil && !isShort(err) {
		return 0, "", fmt.Errorf("read %s: %w", model.PGS, err)
	}
	if n < size {
		return 0, "The segment at byte " + strconv.Itoa(offset) + " has " + strconv.Itoa(size) +
			" bytes of data, but the file ends before.", nil
	}
	return size, "", nil
}

func readHeader(r io.Reader, offset int, header []byte) (msg string, err error) {
	n, err := io.ReadFull(r, header)
	switch {
	case n == 0 && errors.Is(err, io.EOF):
		return "", io.EOF
	case err != nil && !isShort(err):
		return "", fmt.Errorf("read %s: %w", model.PGS, err)
	case n < 2 || header[0] != 'P' || header[1] != 'G':
		return "The segment at byte " + strconv.Itoa(offset) + " does not start with the PG magic bytes.", nil
	case n < headerLength:
		return "The segment header at byte " + strconv.Itoa(offset) + " is cut off.", nil
	}
	return "", nil
}

// isShort reports whether err says that the input ended before the buffer was full.
func isShort(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
