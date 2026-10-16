package stbox

import (
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// The shapes of the Read, Write and Detect functions of a format package.
type (
	readFunc[O any]  func(io.Reader, model.CommonReadOptions, O) (*model.Subtitle, []model.ParseWarning, error)
	writeFunc[O any] func(io.Writer, *model.Subtitle, model.CommonWriteOptions, O) error
	detectFunc       func([]byte) bool
)

var (
	_ readFunc[stboxjson.ReadOptions]   = stboxjson.Read
	_ writeFunc[stboxjson.WriteOptions] = stboxjson.Write
	_ readFunc[webvtt.ReadOptions]      = webvtt.Read
	_ writeFunc[webvtt.WriteOptions]    = webvtt.Write
	_ detectFunc                        = webvtt.Detect
	_ readFunc[srt.ReadOptions]         = srt.Read
	_ writeFunc[srt.WriteOptions]       = srt.Write
	_ detectFunc                        = srt.Detect
	_ readFunc[sbv.ReadOptions]         = sbv.Read
	_ writeFunc[sbv.WriteOptions]       = sbv.Write
	_ detectFunc                        = sbv.Detect
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
	{format: JSON, read: readJSON, write: writeJSON},
	{format: LRC},
	{format: MicroDVD},
	{format: MPSub},
	{format: SAMI},
	{format: SBV, read: readSBV, write: writeSBV},
	{format: SubRip, read: readSubRip, write: writeSubRip},
	{format: SubViewer},
	{format: TTML},
	{format: TSV},
	{format: PlainText},
	{format: MPL2},
	{format: TMPlayer},
	{format: QuickTimeText},
	{format: WebVTT, read: readWebVTT, write: writeWebVTT},
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
	jsonChecks = []jsonCheck{
		{format: JSON, match: stboxjson.DetectJSON},
	}
	signatures = []signature{
		{format: WebVTT, match: webvtt.Detect},
		{format: SubRip, match: srt.Detect},
		{format: SBV, match: sbv.Detect},
	}
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

func readJSON(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return stboxjson.Read(r, common, stboxjson.ReadOptions{})
}

func writeJSON(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	return stboxjson.Write(w, s, common, opts.JSON)
}

func readWebVTT(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return webvtt.Read(r, common, webvtt.ReadOptions{})
}

func writeWebVTT(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	return webvtt.Write(w, s, common, webvtt.WriteOptions{})
}

func readSubRip(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return srt.Read(r, common, srt.ReadOptions{})
}

func writeSubRip(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	return srt.Write(w, s, common, srt.WriteOptions{})
}

func readSBV(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return sbv.Read(r, common, sbv.ReadOptions{})
}

func writeSBV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	return sbv.Write(w, s, common, sbv.WriteOptions{})
}

func lookup(f Format) (formatFuncs, bool) {
	i := slices.IndexFunc(registry, func(row formatFuncs) bool { return row.format == f })
	if i < 0 {
		return formatFuncs{}, false
	}
	return registry[i], true
}
