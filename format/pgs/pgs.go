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
