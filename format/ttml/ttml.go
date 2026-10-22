// Package ttml reads TTML, the W3C XML format for timed text, with its IMSC profiles and the DFXP drafts.
//
// The reader turns the timing model of <body>, <div>, <p> and <span> into cues, the styles into core markup, the
// region into an alignment and the agents into speakers. It stores the namespaces, the <head> and the attributes
// as format data, so a writer can keep them.
package ttml

import (
	"regexp"

	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const formatDataKey = string(model.TTML)

// ReadOptions holds the settings of the TTML reader. It has no fields yet.
type ReadOptions struct{}

// The root namespaces of TTML and of the 2 DFXP drafts.
const (
	nsTTML       = "http://www.w3.org/ns/ttml"
	nsDFXP       = "http://www.w3.org/2006/10/ttaf1"
	nsDFXP200604 = "http://www.w3.org/2006/04/ttaf1"
	nsIMSCStyle  = "http://www.w3.org/ns/ttml/profile/imsc1#styling"
)

//nolint:gochecknoglobals // lookup tables of namespaces
var (
	stylingNamespaces = []string{
		"http://www.w3.org/ns/ttml#styling",
		"http://www.w3.org/2006/10/ttaf1#style",
		"http://www.w3.org/2006/10/ttaf1#styling",
		"http://www.w3.org/2006/04/ttaf1#styling",
		"http://www.w3.org/2006/04/ttaf1#style",
	}
	parameterNamespaces = []string{
		"http://www.w3.org/ns/ttml#parameter",
		"http://www.w3.org/2006/10/ttaf1#parameter",
		"http://www.w3.org/2006/04/ttaf1#parameter",
	}
	metadataNamespaces = []string{
		"http://www.w3.org/ns/ttml#metadata",
		"http://www.w3.org/2006/10/ttaf1#metadata",
		"http://www.w3.org/2006/04/ttaf1#metadata",
	}
)

const rootPattern = `(?:[A-Za-z_][\w.-]*:)?tt`

var signatureRegexp = regexp.MustCompile(xmlraw.Prolog + `<` + rootPattern + `[\t\n\v\f\r />]`)

// Detect reports whether content has the TTML signature of stbox.Detect: the XML prolog, then a <tt> root with
// any prefix. The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
// iTT files have the same signature.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}
