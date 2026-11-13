// Package mp4 reads the subtitle tracks of MP4 and MOV files.
//
// It reads the moov box and the samples of one track. It reads no other media data and no fragmented file.
package mp4

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	boxHeaderSize      = 8
	largeBoxHeaderSize = 16
	fragmentedMessage  = "The MP4 file is fragmented. The reader reads only samples in the moov box."
)

// File is an open MP4 or MOV file. It holds the decoded moov box and reads samples on demand.
type File struct {
	r      io.ReaderAt
	size   int64
	tracks []*track
}

// Open reads the moov box and the track list. It reads no media data.
// A damaged file returns a *model.ParseError.
func Open(r io.ReaderAt, size int64) (*File, error) {
	moov, err := readMoov(r, size)
	if err != nil {
		return nil, err
	}
	if moov.Mvex != nil {
		return nil, parseError(fragmentedMessage)
	}
	var movieTimescale uint32
	if mvhd := first[*mp4.MvhdBox](moov.Children); mvhd != nil {
		movieTimescale = mvhd.Timescale
	}
	f := &File{r: r, size: size}
	for _, trak := range moov.Traks {
		t, err := newTrack(trak, movieTimescale)
		if err != nil {
			return nil, err
		}
		if t != nil {
			f.tracks = append(f.tracks, t)
		}
	}
	return f, nil
}

// Tracks returns the subtitle tracks in file order.
func (f *File) Tracks() []model.Track {
	tracks := make([]model.Track, len(f.tracks))
	for i, t := range f.tracks {
		tracks[i] = t.info
	}
	return tracks
}

// readMoov walks the top-level boxes up to the first moov box and decodes it.
// The mp4ff file decoder would also decode every moof box and reject the box sizes that other files use.
func readMoov(r io.ReaderAt, size int64) (*mp4.MoovBox, error) {
	total := uint64(max(size, 0))
	if total < boxHeaderSize {
		return nil, parseError("The file is not an MP4 file.")
	}
	signature, err := readAt(r, 0, boxHeaderSize)
	if err != nil {
		return nil, err
	}
	if string(signature[4:]) != "ftyp" {
		return nil, parseError("The file is not an MP4 file.")
	}
	for pos := uint64(0); total-pos >= boxHeaderSize; {
		typ, boxSize, headerLen, err := readBoxHeader(r, pos, total)
		if err != nil {
			return nil, err
		}
		switch typ {
		case "moof":
			return nil, parseError(fragmentedMessage)
		case "moov":
			return decodeMoov(r, pos, boxSize, headerLen)
		}
		pos += boxSize
	}
	return nil, parseError("The MP4 file has no moov box.")
}

// readBoxHeader returns the type, the size and the header length of the box at pos. It fails for a box that does
// not fit into the file, which has total bytes.
func readBoxHeader(r io.ReaderAt, pos, total uint64) (typ string, boxSize, headerLen uint64, err error) {
	head, err := readAt(r, pos, min(largeBoxHeaderSize, total-pos))
	if err != nil {
		return "", 0, 0, err
	}
	boxSize, headerLen = uint64(binary.BigEndian.Uint32(head)), boxHeaderSize
	switch {
	case boxSize == 1 && len(head) == largeBoxHeaderSize:
		boxSize, headerLen = binary.BigEndian.Uint64(head[8:]), largeBoxHeaderSize
	case boxSize == 0:
		boxSize = total - pos
	}
	if boxSize < headerLen || boxSize > total-pos {
		return "", 0, 0, parseError("The box " + typeName(head[4:8]) + " at byte " + strconv.FormatUint(pos, 10) +
			" does not fit into its parent box.")
	}
	return string(head[4:8]), boxSize, headerLen, nil
}

// decodeMoov decodes the moov box at pos. The checks of readMoov tie boxSize to the file size.
// DecodeMoovSR assumes a header of 8 bytes, so the call pretends one right before the payload.
// This also takes a moov box with size 0 or a 64-bit size, which DecodeBoxSR rejects.
func decodeMoov(r io.ReaderAt, pos, boxSize, headerLen uint64) (*mp4.MoovBox, error) {
	payload, err := readAt(r, pos+headerLen, boxSize-headerLen)
	if err != nil {
		return nil, err
	}
	header := mp4.BoxHeader{Name: "moov", Size: boxSize - headerLen + boxHeaderSize, Hdrlen: boxHeaderSize}
	box, err := mp4.DecodeMoovSR(header, pos+headerLen-boxHeaderSize, bits.NewFixedSliceReader(payload))
	if err != nil {
		return nil, &model.ParseError{Msg: "The MP4 file is not valid: " + err.Error() + ".", Err: err}
	}
	return box.(*mp4.MoovBox), nil //nolint:forcetypeassert // DecodeMoovSR returns a *MoovBox
}

// readAt reads exactly n bytes at pos.
func readAt(r io.ReaderAt, pos, n uint64) ([]byte, error) {
	buf := make([]byte, n)
	got, err := r.ReadAt(buf, int64(pos))
	if got == len(buf) {
		return buf, nil
	}
	if err == nil || err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return nil, fmt.Errorf("read mp4: %w", err)
}

// typeName returns a printable box type in quotes, and other bytes as 0x and 8 hex digits.
func typeName(t []byte) string {
	if printable(t) {
		return strconv.Quote(string(t))
	}
	return "0x" + hex.EncodeToString(t)
}

func printable(t []byte) bool {
	return !slices.ContainsFunc(t, func(c byte) bool { return c < 0x20 || c > 0x7E })
}

func parseError(msg string) error {
	return &model.ParseError{Msg: msg}
}

// first returns the first child of type T, or the zero value when there is none.
// The typed fields of mp4ff keep the last child.
func first[T mp4.Box](children []mp4.Box) T {
	for _, c := range children {
		if b, ok := c.(T); ok {
			return b
		}
	}
	var none T
	return none
}
