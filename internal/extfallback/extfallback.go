// Package extfallback holds the rule of docs/detection.md step 5: which file extensions may set the format of a
// read when content detection finds none. The root package and the CLI both read it, so a read and the output
// plan of the CLI pick the same format.
package extfallback

import (
	"path/filepath"
	"strings"
)

// Allowed reports whether the extension of path may set the format when detection finds none. The extensions
// json, txt and ffmeta also belong to formats that detection never finds, such as cloud speech-to-text JSON and
// chapters, so they say nothing about the content.
func Allowed(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".txt", ".ffmeta":
		return false
	default:
		return true
	}
}
