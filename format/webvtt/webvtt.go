// Package webvtt reads and writes WebVTT, the caption format of HTML video and of HLS streams.
//
// The reader keeps the header, the STYLE and REGION blocks, the NOTE comments, the cue identifiers and the
// cue settings, so the writer can write them back. A chapters file is a WebVTT file with one cue per chapter.
package webvtt

const signature = "WEBVTT"

// ReadOptions holds the settings of the WebVTT reader. It has no fields yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the WebVTT writer. It has no fields yet.
type WriteOptions struct{}

// WriteReport lists the changes that a WebVTT write made to fit the subtitle.
type WriteReport struct {
	// DroppedColors holds the dropped text colors in cue order.
	DroppedColors []DroppedColor
}

// DroppedColor is a text color that no WebVTT color class holds, so the output leaves it out.
type DroppedColor struct {
	// CueIndex is the 0-based index of the cue.
	CueIndex int
	// Color is the value of the color attribute as written.
	Color string
	// Message is the sentence for a user, such as `Cue #3 at 4 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.`.
	Message string
}

// Detect reports whether content has the WebVTT signature of stbox.Detect: WEBVTT, then a space, a tab, LF or the end.
func Detect(content []byte) bool {
	return hasSignature(content)
}

func hasSignature[T string | []byte](text T) bool {
	if len(text) < len(signature) || string(text[:len(signature)]) != signature {
		return false
	}
	if len(text) == len(signature) {
		return true
	}
	next := text[len(signature)]
	return next == ' ' || next == '\t' || next == '\n'
}
