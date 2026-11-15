package stbox

import (
	"io"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v5/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v5/container/mp4"
	"github.com/yama6a/subtitle-toolbox-go/v5/container/mpegts"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// containerFile is an open container file. O is the ExtractOptions type of its package.
type containerFile[O any] interface {
	Tracks() []Track
	Extract(number int, common model.CommonReadOptions, opts O) (*Subtitle, []ParseWarning, error)
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
	switch c {
	case Matroska:
		f, err := mkv.Open(r, size)
		if err != nil {
			return nil, err
		}
		return f.Tracks(), nil
	case MP4:
		f, err := mp4.Open(r, size)
		if err != nil {
			return nil, err
		}
		return f.Tracks(), nil
	case MPEGTS:
		f, err := mpegts.Open(r, size)
		if err != nil {
			return nil, err
		}
		return f.Tracks(), nil
	}
	return nil, unsupported(notContainer)
}

// readTrack opens the container file and extracts the track that opts names, or the only track.
func readTrack(c Container, r io.ReaderAt, size int64, subject string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if opts.Format != "" {
		return nil, ReadInfo{}, unsupported(subject + " is an " + c.Label() + " file. Read it by track.")
	}
	switch c {
	case Matroska:
		f, err := mkv.Open(r, size)
		if err != nil {
			return nil, ReadInfo{}, err
		}
		return extractTrack[mkv.ExtractOptions](f, c, opts, mkv.ExtractOptions{})
	case MP4:
		f, err := mp4.Open(r, size)
		if err != nil {
			return nil, ReadInfo{}, err
		}
		return extractTrack[mp4.ExtractOptions](f, c, opts, mp4.ExtractOptions{SCC: opts.SCC, MCC: opts.MCC})
	case MPEGTS:
		f, err := mpegts.Open(r, size)
		if err != nil {
			return nil, ReadInfo{}, err
		}
		return extractTrack[mpegts.ExtractOptions](f, c, opts, mpegts.ExtractOptions{})
	}
	return nil, ReadInfo{}, unsupported(notContainer)
}

// extractTrack picks the track that opts names, or the only track, and extracts it with extract.
func extractTrack[O any](f containerFile[O], c Container, opts ReadOptions, extract O) (*Subtitle, ReadInfo, error) {
	track, err := pickTrack(c, f.Tracks(), opts.Track)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	s, warnings, err := f.Extract(track.Number, opts.common(), extract)
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
