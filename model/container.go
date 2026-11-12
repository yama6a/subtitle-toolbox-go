package model

import "strconv"

// Container is a video file format that holds subtitle tracks.
type Container string

// The containers that a read can name.
const (
	Matroska Container = "matroska" // Matroska and WebM
	MP4      Container = "mp4"      // MP4 and MOV
	MPEGTS   Container = "mpegts"   // MPEG-TS and M2TS
)

// Label returns the name of c in messages: "MKV or WebM", "MP4" or "MPEG-TS". Another value returns string(c).
func (c Container) Label() string {
	switch c {
	case Matroska:
		return "MKV or WebM"
	case MP4:
		return "MP4"
	case MPEGTS:
		return "MPEG-TS"
	default:
		return string(c)
	}
}

// Track is a subtitle track of a container.
type Track struct {
	Container Container
	// Number is the number that ReadOptions.Track takes.
	Number int
	// CodecID is the codec of the track, for example "S_TEXT/UTF8", "tx3g" or "dvb_teletext".
	CodecID string
	// Format is the subtitle format that a read extracts from the track, "" when the reader cannot extract this codec.
	Format   Format
	Language string
	// Name is the name of the track, "" when it has none.
	Name    string
	Default bool
	Forced  bool
}

// TrackError is the error of a container read without a track number, when the file has no subtitle track or more
// than one.
type TrackError struct {
	Container Container
	// Tracks holds the subtitle tracks of the file, empty when it has none.
	Tracks []Track
}

// Error returns the sentence of the error, for example "stbox: unsupported format: The MKV or WebM file has 6
// subtitle tracks.".
func (e *TrackError) Error() string {
	msg := "The " + e.Container.Label() + " file has no subtitle track."
	if len(e.Tracks) > 0 {
		msg = "The " + e.Container.Label() + " file has " + strconv.Itoa(len(e.Tracks)) + " subtitle tracks."
	}
	return ErrUnsupportedFormat.Error() + ": " + msg
}

// Is reports whether target is ErrUnsupportedFormat.
func (e *TrackError) Is(target error) bool { return target == ErrUnsupportedFormat }
