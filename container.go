package stbox

import (
	"io"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v4/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v4/container/mpegts"
)

// containerFile is an open container file. Each container has an adapter type for the file type of its package.
type containerFile interface {
	Tracks() []Track
	Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error)
}

type matroskaFile struct{ *mkv.File }

func (f matroskaFile) Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return f.File.Extract(number, opts.common())
}

type mp4File struct{ *mp4.File }

func (f mp4File) Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return f.File.Extract(number, opts.common(), mp4.ExtractOptions{SCC: opts.SCC, MCC: opts.MCC})
}

type mpegtsFile struct{ *mpegts.File }

func (f mpegtsFile) Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return f.File.Extract(number, opts.common())
}

// openContainer opens the file of container c.
func openContainer(c Container, r io.ReaderAt, size int64) (containerFile, error) { //nolint:ireturn // the 3 file types differ
	switch c {
	case Matroska:
		f, err := mkv.Open(r, size)
		if err != nil {
			return nil, err
		}
		return matroskaFile{f}, nil
	case MP4:
		f, err := mp4.Open(r, size)
		if err != nil {
			return nil, err
		}
		return mp4File{f}, nil
	case MPEGTS:
		f, err := mpegts.Open(r, size)
		if err != nil {
			return nil, err
		}
		return mpegtsFile{f}, nil
	}
	return nil, unsupported(notContainer)
}

const notContainer = "The content is not an MKV, WebM, MP4 or MPEG-TS file."

// Tracks returns the subtitle tracks of the video container in the first size bytes of r, in file order.
// A container without subtitle tracks gives an empty list and no error. Content that is no container returns
// an error that matches ErrUnsupportedFormat.
func Tracks(r io.ReaderAt, size int64) ([]Track, error) {
	head, err := readHead(r, size)
	if err != nil {
		return nil, err
	}
	c, ok := containerOf(head)
	if !ok {
		return nil, unsupported(notContainer)
	}
	f, err := openContainer(c, r, size)
	if err != nil {
		return nil, err
	}
	return f.Tracks(), nil
}

// readTrack opens the container file and extracts the track that opts names, or the only track.
func readTrack(c Container, r io.ReaderAt, size int64, subject string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if opts.Format != "" {
		return nil, ReadInfo{}, unsupported(subject + " is an " + c.Label() + " file. Read it by track.")
	}
	f, err := openContainer(c, r, size)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return extractTrack(c, f, opts)
}

// extractTrack extracts the track that opts names from the open file f, or its only track.
func extractTrack(c Container, f containerFile, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	track, err := pickTrack(c, f.Tracks(), opts.Track)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	s, warnings, err := f.Extract(track.Number, opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return s, ReadInfo{Format: track.Format, Container: c, Track: track.Number, Warnings: warnings}, nil
}

// pickTrack returns the track with the given number, or the only track when number is 0.
func pickTrack(c Container, tracks []Track, number int) (Track, error) {
	if number == 0 {
		if len(tracks) != 1 {
			return Track{}, &TrackError{Container: c, Tracks: tracks}
		}
		return tracks[0], nil
	}
	for _, t := range tracks {
		if t.Number == number {
			return t, nil
		}
	}
	return Track{}, &InvalidArgumentError{
		Option: "ReadOptions.Track",
		Msg:    "The file has no subtitle track with the number " + strconv.Itoa(number) + ".",
	}
}
