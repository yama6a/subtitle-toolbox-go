// Package dcp reads and writes the text subtitles of digital cinema packages: Interop <DCSubtitle> from the
// CineCanvas specification and SMPTE ST 428-7 <SubtitleReel>.
//
// Each <Text> element is one line of a cue. A nested <Font> element passes its attributes to the elements inside it.
// The reader stores the root, the header, the <LoadFont> elements and the first <Font> as file data, and the
// <Subtitle> and <Text> attributes as cue data. The writer keeps them, so a retimed file keeps its dialect, header
// and fonts. A new file is SMPTE ST 428-7:2014 at 24 fps with the defaults of DCP-o-matic.
//
// DCP subtitles share the extension .xml with TTML. TTML owns it, so a write needs the format dcp.
package dcp

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.DCP)

// interopRate is the units per second of Interop times. The last field counts ticks of 4 ms.
const interopRate = 250

// defaultRate is the rate of SMPTE ST 428-7 without an EditRate, and of a new file.
const defaultRate = 24

// maxRate is the largest rate that the format data schema holds.
const maxRate = 1000

// edgeBand is the distance from the top or bottom screen edge, in percent, within which a line has the usual DCP place.
const edgeBand = 10

const rootPattern = `(?:DCSubtitle|(?:[A-Za-z_][\w.-]*:)?SubtitleReel)`

var signatureRegexp = regexp.MustCompile(xmlraw.Prolog + `<` + rootPattern + `[\t\n\v\f\r />]`)

// ReadOptions holds the settings of the DCP reader. It has no fields yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the DCP writer. It has no fields yet.
type WriteOptions struct{}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

// Detect reports whether content starts, after an XML prolog, with <DCSubtitle> or a <SubtitleReel> with or without
// prefix. The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
