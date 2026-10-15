package stbox

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// formatFuncs holds the read and write functions of one format. A nil function means
// that the library cannot read or write the format.
type formatFuncs struct {
	format Format
	read   func(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error)
	write  func(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) error
}

// registry lists the formats in the PHP row order, because the first row that lists an
// extension owns it in FormatFromPath. The facts table of package model has the same rows.
//
//nolint:gochecknoglobals // lookup table of the format functions
var registry = []formatFuncs{
	{format: ASS},
	{format: CSV},
	{format: JSON},
	{format: LRC},
	{format: MicroDVD},
	{format: MPSub},
	{format: SAMI},
	{format: SBV},
	{format: SubRip},
	{format: SubViewer},
	{format: TTML},
	{format: TSV},
	{format: PlainText},
	{format: MPL2},
	{format: TMPlayer},
	{format: QuickTimeText},
	{format: WebVTT},
}

type jsonCheck struct {
	format Format
	match  func(keys map[string]json.RawMessage) bool
}

type signature struct {
	format Format
	match  func(content []byte) bool
}

//nolint:gochecknoglobals // ordered detection lists
var (
	jsonChecks []jsonCheck
	signatures []signature
)

// Formats returns every format of the registry, in registry order.
func Formats() []Format {
	formats := make([]Format, len(registry))
	for i, row := range registry {
		formats[i] = row.format
	}
	return formats
}

// FormatFromPath returns the format of the file extension of path, compared without case.
// When formats share an extension, the first row of the registry owns it: ".sub" is MicroDVD,
// ".txt" is plain text, ".xml" is TTML and ".json" is the library JSON.
func FormatFromPath(path string) (Format, bool) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return "", false
	}
	for _, row := range registry {
		if slices.Contains(row.format.Extensions(), ext) {
			return row.format, true
		}
	}
	return "", false
}

// Detect returns the format of content, or false when no auto-detected format matches.
// Content that is a JSON object goes to the JSON checks only.
func Detect(content []byte) (Format, bool) {
	if bytes.HasPrefix(content, []byte("{")) {
		var keys map[string]json.RawMessage
		if json.Unmarshal(content, &keys) == nil {
			for _, c := range jsonChecks {
				if c.match(keys) {
					return c.format, true
				}
			}
			return "", false
		}
	}
	for _, s := range signatures {
		if s.match(content) {
			return s.format, true
		}
	}
	return "", false
}

func lookup(f Format) (formatFuncs, bool) {
	i := slices.IndexFunc(registry, func(row formatFuncs) bool { return row.format == f })
	if i < 0 {
		return formatFuncs{}, false
	}
	return registry[i], true
}
