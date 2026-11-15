// Package pgs reads Blu-ray PGS (Presentation Graphic Stream) subtitles, stored as .sup files.
// Each display set that shows objects becomes one image cue.
//
// The segment layout follows the description at http://blog.thescorpius.com/index.php/2017/07/15/presentation-graphic-stream-sup-files-bluray-subtitle-format/.
// Composition states, cropping and the forced flag follow US patent application US 2009/0185789 A1.
// Run-length decoding and the colour matrix by video height follow the behaviour of FFmpeg libavcodec/pgssubdec.c at
// 5d4d3bdc61412641883a45e060e810f80ea7f4b5. The position of a cropped object follows the behaviour of libbluray
// graphics_controller.c at 0247557842050c8dfc0ae9293d76c3fb7386429a. No code of FFmpeg or libbluray is used.
package pgs

// ReadOptions holds the settings of the PGS reader. PGS has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the PGS writer. PGS has none yet.
type WriteOptions struct{}

// WriteReport lists the changes of a PGS write. PGS reports none yet.
type WriteReport struct{}

// Detect reports whether content starts with a PGS segment header of a known segment type: the bytes PG, and at byte 10
// one of 14h, 15h, 16h, 17h and 80h. The content needs at least 11 bytes.
//
// The check reads the PTS and the DTS at bytes 2 to 9, which can hold 0Dh or 0Ah. So the caller passes the raw
// bytes, without the BOM removal and line end change that stbox.Detect applies to text.
func Detect(content []byte) bool {
	if len(content) < 11 || content[0] != 'P' || content[1] != 'G' {
		return false
	}
	switch content[10] {
	case 0x14, 0x15, 0x16, 0x17, 0x80:
		return true
	}
	return false
}
