package stbox

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/ass"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/lrc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/mpsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/sami"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/sbv"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/sprucestl"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/tmplayer"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// The shapes of the Read, Write and Detect functions of a format package.
type (
	readFunc[O any]     func(io.Reader, model.CommonReadOptions, O) (*model.Subtitle, []model.ParseWarning, error)
	writeFunc[O, R any] func(io.Writer, *model.Subtitle, model.CommonWriteOptions, O) (R, error)
	detectFunc          func([]byte) bool
)

var (
	_ readFunc[stboxjson.ReadOptions]                          = stboxjson.Read
	_ writeFunc[stboxjson.WriteOptions, stboxjson.WriteReport] = stboxjson.Write
	_ readFunc[webvtt.ReadOptions]                             = webvtt.Read
	_ writeFunc[webvtt.WriteOptions, webvtt.WriteReport]       = webvtt.Write
	_ detectFunc                                               = webvtt.Detect
	_ readFunc[srt.ReadOptions]                                = srt.Read
	_ writeFunc[srt.WriteOptions, srt.WriteReport]             = srt.Write
	_ detectFunc                                               = srt.Detect
	_ readFunc[ass.ReadOptions]                                = ass.Read
	_ writeFunc[ass.WriteOptions, ass.WriteReport]             = ass.Write
	_ detectFunc                                               = ass.Detect
	_ readFunc[cavena890.ReadOptions]                          = cavena890.Read
	_ writeFunc[cavena890.WriteOptions, cavena890.WriteReport] = cavena890.Write
	_ readFunc[ttml.ReadOptions]                               = ttml.Read
	_ writeFunc[ttml.WriteOptions, ttml.WriteReport]           = ttml.Write
	_ readFunc[microdvd.ReadOptions]                           = microdvd.Read
	_ writeFunc[microdvd.WriteOptions, microdvd.WriteReport]   = microdvd.Write
	_ detectFunc                                               = microdvd.Detect
	_ detectFunc                                               = ttml.Detect
	_ readFunc[sami.ReadOptions]                               = sami.Read
	_ writeFunc[sami.WriteOptions, sami.WriteReport]           = sami.Write
	_ detectFunc                                               = sami.Detect
	_ readFunc[sbv.ReadOptions]                                = sbv.Read
	_ writeFunc[sbv.WriteOptions, sbv.WriteReport]             = sbv.Write
	_ detectFunc                                               = sbv.Detect
	_ readFunc[scc.ReadOptions]                                = scc.Read
	_ writeFunc[scc.WriteOptions, scc.WriteReport]             = scc.Write
	_ detectFunc                                               = scc.Detect
	_ readFunc[mpl2.ReadOptions]                               = mpl2.Read
	_ writeFunc[mpl2.WriteOptions, mpl2.WriteReport]           = mpl2.Write
	_ detectFunc                                               = mpl2.Detect
	_ readFunc[tmplayer.ReadOptions]                           = tmplayer.Read
	_ writeFunc[tmplayer.WriteOptions, tmplayer.WriteReport]   = tmplayer.Write
	_ detectFunc                                               = tmplayer.Detect
	_ readFunc[mpsub.ReadOptions]                              = mpsub.Read
	_ writeFunc[mpsub.WriteOptions, mpsub.WriteReport]         = mpsub.Write
	_ detectFunc                                               = mpsub.Detect
	_ readFunc[pac.ReadOptions]                                = pac.Read
	_ writeFunc[pac.WriteOptions, pac.WriteReport]             = pac.Write
	_ detectFunc                                               = pac.Detect
	_ readFunc[pgs.ReadOptions]                                = pgs.Read
	_ writeFunc[pgs.WriteOptions, pgs.WriteReport]             = pgs.Write
	_ detectFunc                                               = pgs.Detect
	_ readFunc[lrc.ReadOptions]                                = lrc.Read
	_ writeFunc[lrc.WriteOptions, lrc.WriteReport]             = lrc.Write
	_ detectFunc                                               = lrc.Detect
	_ readFunc[mcc.ReadOptions]                                = mcc.Read
	_ writeFunc[mcc.WriteOptions, mcc.WriteReport]             = mcc.Write
	_ detectFunc                                               = mcc.Detect
	_ readFunc[qttext.ReadOptions]                             = qttext.Read
	_ writeFunc[qttext.WriteOptions, qttext.WriteReport]       = qttext.Write
	_ detectFunc                                               = qttext.Detect
	_ writeFunc[plaintext.WriteOptions, plaintext.WriteReport] = plaintext.Write
	_ readFunc[csv.ReadOptions]                                = csv.Read
	_ writeFunc[csv.WriteOptions, csv.WriteReport]             = csv.Write
	_ readFunc[itt.ReadOptions]                                = itt.Read
	_ writeFunc[itt.WriteOptions, itt.WriteReport]             = itt.Write
	_ readFunc[subviewer.ReadOptions]                          = subviewer.Read
	_ writeFunc[subviewer.WriteOptions, subviewer.WriteReport] = subviewer.Write
	_ detectFunc                                               = subviewer.Detect
	_ readFunc[ebustl.ReadOptions]                             = ebustl.Read
	_ writeFunc[ebustl.WriteOptions, ebustl.WriteReport]       = ebustl.Write
	_ detectFunc                                               = ebustl.Detect
	_ readFunc[sprucestl.ReadOptions]                          = sprucestl.Read
	_ writeFunc[sprucestl.WriteOptions, sprucestl.WriteReport] = sprucestl.Write
	_ detectFunc                                               = sprucestl.Detect
	_ readFunc[dcp.ReadOptions]                                = dcp.Read
	_ writeFunc[dcp.WriteOptions, dcp.WriteReport]             = dcp.Write
	_ detectFunc                                               = dcp.Detect
)

// formatFuncs holds the read and write functions of one format. A nil function means
// that the library cannot read or write the format.
type formatFuncs struct {
	format Format
	read   func(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error)
	write  func(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error)
}

// registry lists the formats in the PHP row order, because the first row that lists an
// extension owns it in FormatFromPath. The facts table of package model has the same rows.
//
//nolint:gochecknoglobals // lookup table of the format functions
var registry = []formatFuncs{
	{format: ASS, read: readASS, write: writeASS},
	{format: Cavena890, read: readCavena890, write: writeCavena890},
	{format: CSV, read: readCSV, write: writeCSV},
	{format: ITT, read: readITT, write: writeITT},
	{format: JSON, read: readJSON, write: writeJSON},
	{format: LRC, read: readLRC, write: writeLRC},
	{format: MCC, read: readMCC, write: writeMCC},
	{format: MicroDVD, read: readMicroDVD, write: writeMicroDVD},
	{format: MPSub, read: readMPSub, write: writeMPSub},
	{format: PAC, read: readPAC, write: writePAC},
	{format: PGS, read: readPGS, write: writePGS},
	{format: SAMI, read: readSAMI, write: writeSAMI},
	{format: SBV, read: readSBV, write: writeSBV},
	{format: SCC, read: readSCC, write: writeSCC},
	{format: SubRip, read: readSubRip, write: writeSubRip},
	{format: EBUSTL, read: readEBUSTL, write: writeEBUSTL},
	{format: SpruceSTL, read: readSpruceSTL, write: writeSpruceSTL},
	{format: SubViewer, read: readSubViewer, write: writeSubViewer},
	{format: TTML, read: readTTML, write: writeTTML},
	{format: DCP, read: readDCP, write: writeDCP},
	{format: TSV, read: readTSV, write: writeTSV},
	{format: PlainText, write: writePlainText},
	{format: MPL2, read: readMPL2, write: writeMPL2},
	{format: TMPlayer, read: readTMPlayer, write: writeTMPlayer},
	{format: QuickTimeText, read: readQuickTimeText, write: writeQuickTimeText},
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
	rawSignatures = []signature{
		{format: PGS, match: pgs.Detect},
	}
	signatures = []signature{
		{format: WebVTT, match: webvtt.Detect},
		{format: TTML, match: ttml.Detect},
		{format: DCP, match: dcp.Detect},
		{format: SAMI, match: sami.Detect},
		{format: ASS, match: ass.Detect},
		{format: MPSub, match: mpsub.Detect},
		{format: MicroDVD, match: microdvd.Detect},
		{format: QuickTimeText, match: qttext.Detect},
		{format: SubRip, match: srt.Detect},
		{format: SBV, match: sbv.Detect},
		{format: SubViewer, match: subviewer.Detect},
		{format: LRC, match: lrc.Detect},
		{format: EBUSTL, match: ebustl.Detect},
		{format: PAC, match: pac.Detect},
		{format: SpruceSTL, match: sprucestl.Detect},
		{format: SCC, match: scc.Detect},
		{format: MCC, match: mcc.Detect},
		{format: MPL2, match: mpl2.Detect},
		{format: TMPlayer, match: tmplayer.Detect},
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
	ext := extension(path)
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

// extension returns the file extension of path without the dot, in lower case.
func extension(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

func readCavena890(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return cavena890.Read(r, common, cavena890.ReadOptions{})
}

func writeCavena890(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := cavena890.Write(w, s, common, cavena890.WriteOptions{})
	return WriteReport{}, err
}

func readEBUSTL(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return ebustl.Read(r, common, opts.EBUSTL)
}

func readITT(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return itt.Read(r, common, itt.ReadOptions{})
}

func writeITT(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := itt.Write(w, s, common, opts.ITT)
	return WriteReport{}, err
}

func readPGS(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return pgs.Read(r, common, pgs.ReadOptions{})
}

func writePGS(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := pgs.Write(w, s, common, pgs.WriteOptions{})
	return WriteReport{}, err
}

func readPAC(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	pacOpts := opts.PAC
	if pacOpts.CodePage == pac.CodePageAuto && opts.Encoding != "" {
		codePage, ok := pac.CodePageFromEncoding(opts.Encoding)
		if !ok {
			return nil, nil, &InvalidArgumentError{
				Option: "ReadOptions.Encoding",
				Msg:    "PAC files have no code page for the encoding " + strconv.Quote(opts.Encoding) + ".",
			}
		}
		pacOpts.CodePage = codePage
	}
	return pac.Read(r, common, pacOpts)
}

func writePAC(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := pac.Write(w, s, common, pac.WriteOptions{})
	return WriteReport{}, err
}

func readJSON(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return stboxjson.Read(r, common, stboxjson.ReadOptions{})
}

func writeJSON(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := stboxjson.Write(w, s, common, opts.JSON)
	return WriteReport{}, err
}

func readWebVTT(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return webvtt.Read(r, common, webvtt.ReadOptions{})
}

func writeWebVTT(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	report, err := webvtt.Write(w, s, common, webvtt.WriteOptions{})
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{WebVTT: &report}, nil
}

func readSubRip(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return srt.Read(r, common, srt.ReadOptions{})
}

func writeSubRip(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := srt.Write(w, s, common, srt.WriteOptions{})
	return WriteReport{}, err
}

func readASS(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return ass.Read(r, common, ass.ReadOptions{})
}

func writeASS(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := ass.Write(w, s, common, opts.ASS)
	return WriteReport{}, err
}

func readTTML(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return ttml.Read(r, common, ttml.ReadOptions{})
}

func writeTTML(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := ttml.Write(w, s, common, ttml.WriteOptions{})
	return WriteReport{}, err
}

func readSAMI(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return sami.Read(r, common, opts.SAMI)
}

func writeSAMI(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := sami.Write(w, s, common, sami.WriteOptions{})
	return WriteReport{}, err
}

func readMicroDVD(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return microdvd.Read(r, common, opts.MicroDVD)
}

func writeMicroDVD(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := microdvd.Write(w, s, common, opts.MicroDVD)
	return WriteReport{}, err
}

func writeEBUSTL(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	report, err := ebustl.Write(w, s, common, opts.EBUSTL)
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{EBUSTL: &report}, nil
}

func readSCC(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return scc.Read(r, common, opts.SCC)
}

func writeSCC(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	report, err := scc.Write(w, s, common, opts.SCC)
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{SCC: &report}, nil
}

func readMCC(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return mcc.Read(r, common, opts.MCC)
}

func writeMCC(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	report, err := mcc.Write(w, s, common, opts.MCC)
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{MCC: &report}, nil
}

func readDCP(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return dcp.Read(r, common, dcp.ReadOptions{})
}

func writeDCP(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := dcp.Write(w, s, common, dcp.WriteOptions{})
	return WriteReport{}, err
}

func readSpruceSTL(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return sprucestl.Read(r, common, opts.SpruceSTL)
}

func writeSpruceSTL(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := sprucestl.Write(w, s, common, opts.SpruceSTL)
	return WriteReport{}, err
}

func readSBV(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return sbv.Read(r, common, sbv.ReadOptions{})
}

func writeSBV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := sbv.Write(w, s, common, sbv.WriteOptions{})
	return WriteReport{}, err
}

func readMPL2(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return mpl2.Read(r, common, mpl2.ReadOptions{})
}

func writeMPL2(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := mpl2.Write(w, s, common, mpl2.WriteOptions{})
	return WriteReport{}, err
}

func readTMPlayer(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return tmplayer.Read(r, common, tmplayer.ReadOptions{})
}

func writeTMPlayer(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := tmplayer.Write(w, s, common, tmplayer.WriteOptions{})
	return WriteReport{}, err
}

func readMPSub(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return mpsub.Read(r, common, mpsub.ReadOptions{})
}

func writeMPSub(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := mpsub.Write(w, s, common, opts.MPSub)
	return WriteReport{}, err
}

func readLRC(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return lrc.Read(r, common, lrc.ReadOptions{})
}

func writeLRC(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := lrc.Write(w, s, common, lrc.WriteOptions{})
	return WriteReport{}, err
}

func writePlainText(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := plaintext.Write(w, s, common, opts.PlainText)
	return WriteReport{}, err
}

func readCSV(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	return csv.Read(r, common, opts.CSV)
}

// readTSV reads with the CSV reader, which detects the delimiter, and names TSV in its errors.
func readTSV(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*Subtitle, []ParseWarning, error) {
	s, warnings, err := csv.Read(r, common, opts.CSV)
	var perr *ParseError
	if errors.As(err, &perr) && perr.Format == CSV {
		withFormat := *perr
		withFormat.Format = TSV
		return nil, nil, &withFormat
	}
	return s, warnings, err
}

// writeCSV writes a comma for a subtitle whose stored delimiter is a tab, unless the options set a delimiter.
func writeCSV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if d, _ := s.FormatData(string(CSV)); opts.CSV.Delimiter == csv.DelimiterDefault && isTab(d) {
		opts.CSV.Delimiter = csv.Comma
	}
	_, err := csv.Write(w, s, common, opts.CSV)
	return WriteReport{}, err
}

func isTab(d *model.Data) bool {
	delimiter, _ := d.String("delimiter")
	return delimiter == string(csv.Tab)
}

// writeTSV writes a tab unless the options set a delimiter.
func writeTSV(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if opts.CSV.Delimiter == csv.DelimiterDefault {
		opts.CSV.Delimiter = csv.Tab
	}
	s, err := writing.SkipImageCues(s, model.TSV, common)
	if err != nil {
		return WriteReport{}, err
	}
	_, err = csv.Write(w, s, common, opts.CSV)
	return WriteReport{}, err
}

func readSubViewer(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return subviewer.Read(r, common, subviewer.ReadOptions{})
}

func writeSubViewer(w io.Writer, s *Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	_, err := subviewer.Write(w, s, common, opts.SubViewer)
	return WriteReport{}, err
}

func readQuickTimeText(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*Subtitle, []ParseWarning, error) {
	return qttext.Read(r, common, qttext.ReadOptions{})
}

func writeQuickTimeText(w io.Writer, s *Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	_, err := qttext.Write(w, s, common, qttext.WriteOptions{})
	return WriteReport{}, err
}

func lookup(f Format) (formatFuncs, bool) {
	i := slices.IndexFunc(registry, func(row formatFuncs) bool { return row.format == f })
	if i < 0 {
		return formatFuncs{}, false
	}
	return registry[i], true
}
