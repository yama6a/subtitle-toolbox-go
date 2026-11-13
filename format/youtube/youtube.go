// Package youtube reads and writes the timed text that YouTube serves: json3, srv3, srv2 and srv1, which is also
// the transcript XML. The file format data names the variant, so a write gives the variant of the source.
package youtube

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/xmlraw"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const key = string(model.YouTubeTimedText)

// The variants. The reader stores the name as the format field of the file format data.
const (
	variantSrv1  = "srv1"
	variantSrv2  = "srv2"
	variantSrv3  = "srv3"
	variantJSON3 = "json3"
)

// writerLastCueDuration is the last cue duration that a reader with the default options uses. The writer needs
// the end that such a reader computes, so it must not read ReadOptions.
const writerLastCueDuration = 5 * time.Second

// ReadOptions holds the settings of the YouTube timed text reader.
type ReadOptions struct {
	// WordTimestamps fills Cue.Words with one word per timed segment. It needs at least one segment with an offset
	// in the caption. srv1 and srv2 have no word times.
	WordTimestamps bool
}

// WriteOptions is empty. The writer takes the variant from the file format data.
type WriteOptions struct{}

var signatureRegexp = regexp.MustCompile(xmlraw.Prolog + `<(?:timedtext|transcript)[\t\n\v\f\r />]`)

// Detect reports whether content is YouTube XML: the XML prolog, then <timedtext or <transcript, then white space,
// ">" or "/". stbox.Detect runs the same check after the MCC check.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}

// DetectJSON is JSON check 4 on the top-level keys of an object. It reports whether events is an array whose first
// item is an object with the key tStartMs.
func DetectJSON(keys map[string]json.RawMessage) bool {
	events, err := orderedjson.Decode(keys["events"])
	if err != nil || events.Kind() != orderedjson.Array || len(events.Items()) == 0 {
		return false
	}
	first := events.Items()[0]
	return first.Kind() == orderedjson.Object && first.Has("tStartMs")
}

// isJSON reports whether content is json3: it starts with "{" after BOMs and white space.
func isJSON(content []byte) bool {
	content = bytes.TrimLeft(textenc.TrimUTF8BOMs(content), markupcore.PHPTrimSet)
	return len(content) > 0 && content[0] == '{'
}

// style is the look of a segment, as a pen holds it.
type style struct {
	color                   string
	bold, italic, underline bool
}

// segment is a text of a caption with its offset from the caption start.
type segment struct {
	text      string
	offset    time.Duration
	hasOffset bool
	style     style
}

// stringOf returns the text of a string value, "" for a value of another kind. AsString alone returns the literal of
// a number too.
func stringOf(v orderedjson.Value) string {
	s, ok := v.AsString()
	if !ok {
		return ""
	}
	return s
}

func trimSpace(s string) string { return strings.Trim(s, markupcore.PHPTrimSet) }
