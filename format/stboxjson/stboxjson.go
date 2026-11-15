// Package stboxjson reads and writes library JSON, the lossless JSON form of a subtitle.
//
// A library JSON document holds every field of the model: metadata, comments, format data and cues.
// So a trip through library JSON keeps what the source format stored, such as the styles of an ASS file.
package stboxjson

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// Version is the newest library JSON version. The reader reads versions 1 and 2. The writer writes it by default.
const Version = 2

// ReadOptions holds the library JSON read settings. It has no fields yet.
type ReadOptions struct{}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

// WriteOptions holds the library JSON write settings.
type WriteOptions struct {
	// PrettyPrint indents with 4 spaces and ends the output with LF. False writes one compact line without a final LF.
	PrettyPrint bool
	// OmitFormatData leaves out formatData on the subtitle and on every cue. False writes it at both levels.
	OmitFormatData bool
	// Version is the library JSON version to write. 0 and 2 write version 2. 1 writes version 1, with the word
	// timestamps and the speaker as tags in the lines. Another value fails the write.
	Version int
}

// Validate returns an error that matches model.ErrInvalidArgument for a Version other than 0, 1 and 2.
func (o WriteOptions) Validate() error {
	if o.Version < 0 || o.Version > Version {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.JSON.Version",
			Msg:    "The library JSON version must be 0, 1 or 2, got " + strconv.Itoa(o.Version) + ".",
		}
	}
	return nil
}

// DetectJSON reports whether the top-level keys of a JSON object match the library JSON rule: version is a JSON
// number and cues is a JSON array.
func DetectJSON(keys map[string]json.RawMessage) bool {
	version := bytes.TrimLeft(keys["version"], " \t\r\n")
	cues := bytes.TrimLeft(keys["cues"], " \t\r\n")
	return len(version) > 0 && (version[0] == '-' || version[0] >= '0' && version[0] <= '9') &&
		len(cues) > 0 && cues[0] == '['
}
