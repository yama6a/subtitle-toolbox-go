// Package extfallback holds the extension rules of docs/detection.md steps 7 and 8. Step 7 lets the extension pick a
// profile of the detected format. Step 8 lets the extension set the format of a read when content detection finds
// none. The root package and the CLI both read it, so a read and the output plan of the CLI pick the same format.
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

// Profile returns the format that the extension of path names when that format is a profile of the detected format:
// "itt" for "ttml" and a .itt file. Otherwise it returns detected.
func Profile(detected, path string) string {
	if detected == "ttml" && strings.EqualFold(filepath.Ext(path), ".itt") {
		return "itt"
	}
	return detected
}
