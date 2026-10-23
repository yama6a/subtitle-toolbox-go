// Package subviewer reads and writes SubViewer 1 and 2, the .sub formats of old DivX releases.
//
// SubViewer 2 has a header of [TAG]value lines and cue blocks of a timing line such as "00:00:01.50,00:00:04.00"
// and text lines. SubViewer 1 has a header, then a "******** START SCRIPT ********" line, then time lines such as
// "[00:00:01]" in whole seconds. One reader reads both versions, and the writer writes either.
package subviewer

import (
	"regexp"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.SubViewer)

// Version is the SubViewer version that Write writes.
type Version uint8

// The SubViewer versions. VersionDefault is the zero value.
const (
	VersionDefault Version = iota // the stored version, else SubViewer 2
	V1                            // SubViewer 1: [hh:mm:ss] time lines
	V2                            // SubViewer 2: hh:mm:ss.cc,hh:mm:ss.cc timing lines
)

// String returns "default", "1" or "2".
func (v Version) String() string {
	switch v {
	case VersionDefault:
		return "default"
	case V1:
		return "1"
	case V2:
		return "2"
	}
	return "Version(" + strconv.Itoa(int(v)) + ")"
}

// ReadOptions holds the settings of the SubViewer reader. SubViewer has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the SubViewer writer.
type WriteOptions struct {
	// Version is the version to write. The zero value writes the version that the reader stored, else SubViewer 2.
	Version Version
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown version.
func (o WriteOptions) Validate() error {
	if o.Version > V2 {
		return &model.InvalidArgumentError{
			Option: "WriteOptions.SubViewer.Version",
			Msg:    "The SubViewer version must be 1 or 2, got " + strconv.Itoa(int(o.Version)) + ".",
		}
	}
	return nil
}

var signatureRegexp = regexp.MustCompile(`(?m)^\*{8} START SCRIPT \*{8}[ \t]*$|\A(?:\[INFORMATION\]|(?:\[.*\n)*` +
	`\d{2}:\d{2}:\d{2}\.\d{2},\d{2}:\d{2}:\d{2}\.\d{2})[ \t]*$`)

// Detect reports whether content has a SubViewer 1 script line anywhere, or starts like SubViewer 2. That is a first
// line [INFORMATION], or lines that start with [ followed by a timing line with 2 fraction digits, such as
// 00:00:01.50,00:00:04.00. The content must have no leading BOM or white space and LF line endings, as stbox.Detect
// passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
