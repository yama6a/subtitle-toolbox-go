package stbox

import (
	"io"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v3/container/mpegts"
)

// containerFile is an open container file. Each table row has an adapter type for the file type of its package.
type containerFile interface {
	Tracks() []Track
	Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error)
}

// containerFuncs is one row of the container table: the container and the function that opens a file of it.
type containerFuncs struct {
	container Container
	open      func(r io.ReaderAt, size int64) (containerFile, error)
}

// containers lists the containers that the library reads.
//
//nolint:gochecknoglobals // lookup table of the container readers
var containers = []containerFuncs{
	{container: Matroska, open: openMatroska},
	{container: MPEGTS, open: openMPEGTS},
}

type matroskaFile struct{ *mkv.File }

func (f matroskaFile) Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return f.File.Extract(number, opts.common())
}

func openMatroska(r io.ReaderAt, size int64) (containerFile, error) { //nolint:ireturn // the row type of the container table returns the interface
	f, err := mkv.Open(r, size)
	if err != nil {
		return nil, err
	}
	return matroskaFile{f}, nil
}

type mpegtsFile struct{ *mpegts.File }

func (f mpegtsFile) Extract(number int, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return f.File.Extract(number, opts.common())
}

func openMPEGTS(r io.ReaderAt, size int64) (containerFile, error) { //nolint:ireturn // the row type of the container table returns the interface
	f, err := mpegts.Open(r, size)
	if err != nil {
		return nil, err
	}
	return mpegtsFile{f}, nil
}

// Tracks returns the subtitle tracks of the video container in the first size bytes of r, in file order.
// A container without subtitle tracks gives an empty list and no error. Content that is no container, and a
// container that this version cannot read, return an error that matches ErrUnsupportedFormat.
func Tracks(r io.ReaderAt, size int64) ([]Track, error) {
	head, err := readHead(r, size)
	if err != nil {
		return nil, err
	}
	c, ok := containerOf(head)
	if !ok {
		return nil, unsupported("The content is not an MKV, WebM, MP4 or MPEG-TS file.")
	}
	row, ok := containerRow(c)
	if !ok {
		return nil, noReader("The content", c)
	}
	f, err := row.open(r, size)
	if err != nil {
		return nil, err
	}
	return f.Tracks(), nil
}

func containerRow(c Container) (containerFuncs, bool) {
	for _, row := range containers {
		if row.container == c {
			return row, true
		}
	}
	return containerFuncs{}, false
}

func noReader(subject string, c Container) error {
	return unsupported(subject + " is an " + c.Label() + " file. This version cannot read " + c.Label() + " files.")
}

// readContainer reads a track of the video container c. subject is the path, or "The content".
func readContainer(c Container, r io.ReaderAt, size int64, subject string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	row, ok := containerRow(c)
	if !ok {
		return nil, ReadInfo{}, noReader(subject, c)
	}
	return readTrack(row, r, size, subject, opts)
}

// readTrack opens the container file and extracts the track that opts names, or the only track.
func readTrack(row containerFuncs, r io.ReaderAt, size int64, subject string, opts ReadOptions) (*Subtitle, ReadInfo, error) {
	if opts.Format != "" {
		return nil, ReadInfo{}, unsupported(subject + " is an " + row.container.Label() + " file. Read it by track.")
	}
	f, err := row.open(r, size)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	track, err := pickTrack(row.container, f.Tracks(), opts.Track)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	s, warnings, err := f.Extract(track.Number, opts)
	if err != nil {
		return nil, ReadInfo{}, err
	}
	return s, ReadInfo{Format: track.Format, Container: row.container, Track: track.Number, Warnings: warnings}, nil
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
