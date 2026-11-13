// Package ttml reads and writes TTML, the W3C XML format for timed text, with its IMSC profiles and the DFXP drafts.
//
// The reader turns the timing model of <body>, <div>, <p> and <span> into cues, the styles into core markup, the
// region into an alignment and the agents into speakers. It stores the namespaces, the <head> and the attributes
// as format data, and the writer keeps them, so a DFXP file stays DFXP after a retiming.
package ttml

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const formatDataKey = string(model.TTML)

// ReadOptions holds the settings of the TTML reader. It has no fields yet.
type ReadOptions struct{}

const rootPattern = `(?:[A-Za-z_][\w.-]*:)?tt`

var signatureRegexp = regexp.MustCompile(xmlraw.Prolog + `<` + rootPattern + `[\t\n\v\f\r />]`)

// Detect reports whether content has the TTML signature of stbox.Detect: the XML prolog, then a <tt> root with
// any prefix. The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
// iTT files have the same signature.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
