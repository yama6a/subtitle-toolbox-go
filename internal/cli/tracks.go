package cli

import (
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

// describeTrack returns the text of a track in info and in track errors, for example `S_HDMV/PGS, eng, default, forced`.
// The name is a JSON string.
func describeTrack(t stbox.Track) string {
	var parts []string
	for _, p := range []string{t.CodecID, t.Language} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if t.Name != "" {
		parts = append(parts, jsonString(t.Name))
	}
	if t.Default {
		parts = append(parts, "default")
	}
	if t.Forced {
		parts = append(parts, "forced")
	}
	return strings.Join(parts, ", ")
}

// trackErrorMessage returns the text of a track error with the track option of the file, for example "--new-track".
func trackErrorMessage(e *stbox.TrackError, option string) string {
	label := e.Container.Label()
	if len(e.Tracks) == 0 {
		return "The " + label + " file has no subtitle track."
	}
	var b strings.Builder
	b.WriteString("The " + label + " file has " + strconv.Itoa(len(e.Tracks)) + " subtitle tracks. Pass " + option +
		" N with one of them:")
	for _, t := range e.Tracks {
		b.WriteString("\n  " + strconv.Itoa(t.Number) + ": " + describeTrack(t))
	}
	return b.String()
}

// trackEntry is one object of the tracks list of info --json.
type trackEntry struct {
	Number   int     `json:"number"`
	CodecID  string  `json:"codecId"`
	Language *string `json:"language"` // nil prints null for an empty language
	Name     *string `json:"name"`     // nil prints null for an empty name
	Default  bool    `json:"default"`
	Forced   bool    `json:"forced"`
}

// containerEntry is the info --json object of a video file that info lists by its tracks.
type containerEntry struct {
	File      string          `json:"file"`
	Container stbox.Container `json:"container"`
	Tracks    []trackEntry    `json:"tracks"`
}

func newContainerEntry(in *input) containerEntry {
	entry := containerEntry{File: in.arg, Container: in.container, Tracks: make([]trackEntry, len(in.tracks))}
	for i, t := range in.tracks {
		entry.Tracks[i] = trackEntry{
			Number: t.Number, CodecID: t.CodecID, Language: nilIfEmpty(t.Language), Name: nilIfEmpty(t.Name),
			Default: t.Default, Forced: t.Forced,
		}
	}
	return entry
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// listTracksText returns the text block of a video file that info lists by its tracks.
func listTracksText(in *input) string {
	var b strings.Builder
	b.WriteString(in.label() + "\n  Container: " + string(in.container) + "\n")
	for _, t := range in.tracks {
		b.WriteString("  Track " + strconv.Itoa(t.Number) + ": " + describeTrack(t) + "\n")
	}
	return b.String()
}
