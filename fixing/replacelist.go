package fixing

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/dlclark/regexp2/v2"
	"golang.org/x/text/encoding/htmlindex"

	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// Replacement is one search text and its replacement.
type Replacement struct {
	From string
	To   string
}

// ReplaceList holds the replacements of an OCR replace list, each section in file order.
//
// Adapted from SubtitleEdit/subtitleedit, src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/e1b854665b40bf6e04271c2ec64084947060e632/src/libuilogic/Ocr/FixEngine/OcrFixReplaceList2.cs), MIT.
type ReplaceList struct {
	// WholeWords match a word between spaces, with or without the punctuation around it.
	WholeWords []Replacement
	// PartialWordsAlways match any part of any word, before the second whole word lookup.
	PartialWordsAlways []Replacement
	// WholeLines match the whole visible text of a line.
	WholeLines []Replacement
	// BeginLines match the start of a line, or of a sentence after ". ", "! " or "? ".
	BeginLines []Replacement
	// EndLines match the end of the last line of the cue.
	EndLines []Replacement
	// PartialLines match a text that starts and ends at a word boundary.
	PartialLines []Replacement
	// PartialLinesAlways match any part of a line.
	PartialLinesAlways []Replacement
	// RegularExpressions hold a .NET pattern in From and a .NET substitution in To.
	RegularExpressions []Replacement
}

// Validate returns an error that matches model.ErrInvalidArgument when a regular expression does not compile.
func (l *ReplaceList) Validate() error {
	for _, r := range l.RegularExpressions {
		if r.From == "" {
			return invalid("ReplaceList", `The regular expression "" is not valid: the pattern is empty.`)
		}
		if _, err := regexp2.Compile(r.From, regexp2.None); err != nil {
			return invalid("ReplaceList", `The regular expression "`+r.From+`" is not valid: `+err.Error()+".")
		}
	}
	return nil
}

// ReadReplaceList reads a Subtitle Edit OCR replace list. It skips the sections that need a spell checker, and
// the regular expressions that do not compile. It returns a *model.ParseError for XML that is not valid.
func ReadReplaceList(r io.Reader) (*ReplaceList, error) {
	d := xml.NewDecoder(r)
	d.CharsetReader = charsetReader
	rd := listReader{seen: map[*[]Replacement]map[string]bool{}}
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) && rd.started && rd.depth == 0 {
			return &rd.list, nil
		}
		if err != nil {
			line, _ := d.InputPos()
			return nil, notValid(err, line)
		}
		if err := rd.token(tok); err != nil {
			line, _ := d.InputPos()
			return nil, notValid(err, line)
		}
	}
}

// listReader tracks the position in the document: depth 2 is a section and depth 3 an entry.
type listReader struct {
	list    ReplaceList
	seen    map[*[]Replacement]map[string]bool
	depth   int
	started bool
	section *[]Replacement
	regex   bool
}

// token handles one token. It fails for a second root element and for text outside the root.
func (rd *listReader) token(tok xml.Token) error {
	if rd.outsideRoot(tok) {
		return errExtraContent
	}
	switch t := tok.(type) {
	case xml.StartElement:
		rd.started = true
		rd.depth++
		switch {
		case rd.depth == 2:
			rd.section, rd.regex = rd.list.section(t.Name.Local)
		case rd.depth == 3 && rd.section != nil:
			addEntry(rd.section, rd.seen, rd.regex, t.Attr)
		}
	case xml.EndElement:
		rd.depth--
	}
	return nil
}

func (rd *listReader) outsideRoot(tok xml.Token) bool {
	if rd.depth != 0 {
		return false
	}
	switch t := tok.(type) {
	case xml.CharData:
		text := bytes.TrimSpace(t)
		if !rd.started {
			text = bytes.TrimSpace(bytes.TrimPrefix(text, []byte("\ufeff")))
		}
		return len(text) > 0
	case xml.StartElement:
		return rd.started
	}
	return false
}

var errExtraContent = errors.New("extra content outside the root element")

// addEntry appends the entry that attrs hold to section, unless the section has an entry for its search text.
func addEntry(section *[]Replacement, seen map[*[]Replacement]map[string]bool, regex bool, attrs []xml.Attr) {
	read := lineEntry
	if regex {
		read = regexEntry
	}
	rep, ok := read(attrs)
	if !ok || seen[section][rep.From] {
		return
	}
	if seen[section] == nil {
		seen[section] = map[string]bool{}
	}
	seen[section][rep.From] = true
	*section = append(*section, rep)
}

// section returns the field of the section name, and whether it holds regular expressions. It returns nil for a
// section that the reader skips.
func (l *ReplaceList) section(name string) (field *[]Replacement, regex bool) {
	switch name {
	case "WholeWords":
		return &l.WholeWords, false
	case "PartialWordsAlways":
		return &l.PartialWordsAlways, false
	case "WholeLines":
		return &l.WholeLines, false
	case "BeginLines":
		return &l.BeginLines, false
	case "EndLines":
		return &l.EndLines, false
	case "PartialLines":
		return &l.PartialLines, false
	case "PartialLinesAlways":
		return &l.PartialLinesAlways, false
	case "RegularExpressions":
		return &l.RegularExpressions, true
	}
	return nil, false
}

func attr(attrs []xml.Attr, name string) (string, bool) {
	for _, a := range attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func lineEntry(attrs []xml.Attr) (Replacement, bool) {
	to, ok := attr(attrs, "to")
	from, _ := attr(attrs, "from")
	return Replacement{from, to}, ok && from != "" && from != to
}

func regexEntry(attrs []xml.Attr) (Replacement, bool) {
	to, ok := attr(attrs, "replaceWith")
	find, _ := attr(attrs, "find")
	if !ok || find == "" {
		return Replacement{}, false
	}
	if _, err := regexp2.Compile(find, regexp2.None); err != nil {
		return Replacement{}, false
	}
	return Replacement{find, to}, true
}

func charsetReader(label string, input io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return nil, fmt.Errorf("encoding %q: %w", label, err)
	}
	return enc.NewDecoder().Reader(input), nil
}

func notValid(err error, line int) error {
	const prefix = "The OCR replace list is not valid XML: "
	if se, ok := errors.AsType[*xml.SyntaxError](err); ok {
		return &model.ParseError{Line: se.Line, Msg: prefix + se.Msg + ".", Err: err}
	}
	msg := "the document has no root element."
	if !errors.Is(err, io.EOF) {
		msg = err.Error() + "."
	}
	return &model.ParseError{Line: line, Msg: prefix + msg, Err: err}
}
