// Package mkv reads the subtitle tracks of Matroska and WebM files.
//
// Elements: https://www.matroska.org/technical/elements.html
// Subtitle codecs: https://www.matroska.org/technical/subtitles.html
package mkv

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ebml"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	idEBML            = 0x1A45DFA3
	idDocType         = 0x4282
	idSegment         = 0x18538067
	idSeekHead        = 0x114D9B74
	idSeek            = 0x4DBB
	idSeekID          = 0x53AB
	idSeekPosition    = 0x53AC
	idInfo            = 0x1549A966
	idTimestampScale  = 0x2AD7B1
	idWritingApp      = 0x5741
	idTracks          = 0x1654AE6B
	idTrackEntry      = 0xAE
	idTrackNumber     = 0xD7
	idTrackUID        = 0x73C5
	idTrackType       = 0x83
	idCodecID         = 0x86
	idCodecPrivate    = 0x63A2
	idLanguage        = 0x22B59C
	idLanguageBCP47   = 0x22B59D
	idName            = 0x536E
	idFlagDefault     = 0x88
	idFlagForced      = 0x55AA
	idDefaultDuration = 0x23E383
	idEncodings       = 0x6D80
	idEncoding        = 0x6240
	idEncodingOrder   = 0x5031
	idEncodingScope   = 0x5032
	idEncodingType    = 0x5033
	idCompression     = 0x5034
	idCompAlgo        = 0x4254
	idCompSettings    = 0x4255
	idCluster         = 0x1F43B675
	idTimestamp       = 0xE7
	idSimpleBlock     = 0xA3
	idBlockGroup      = 0xA0
	idBlock           = 0xA1
	idBlockDuration   = 0x9B
	idBlockAdditions  = 0x75A1
	idBlockMore       = 0xA6
	idBlockAddID      = 0xEE
	idBlockAdditional = 0xA5
	idCues            = 0x1C53BB6B
	idCuePoint        = 0xBB
	idCuePositions    = 0xB7
	idCueTrack        = 0xF7
	idCueCluster      = 0xF1
	idCueRelative     = 0xF0
	idTags            = 0x1254C367
	idTag             = 0x7373
	idTargets         = 0x63C0
	idTagTrackUID     = 0x63C5
	idSimpleTag       = 0x67C8
	idTagName         = 0x45A3
	idTagString       = 0x4487
)

const (
	trackTypeSubtitle     = 0x11
	defaultTimestampScale = 1_000_000
	defaultLanguage       = "eng"
	mkvmergeWritingApp    = "mkvmerge "
	noPosition            = -1
)

type encoding struct {
	scope, kind, algo uint64
	settings          []byte
}

type trackEntry struct {
	model.Track
	uid             string
	codecPrivate    []byte
	defaultDuration int64 // ns, noDuration without one
	encodings       []encoding
}

// File is an open Matroska or WebM file.
type File struct {
	r            *ebml.Reader
	segmentStart int64
	segmentEnd   int64
	firstCluster int64
	cuesOffset   int64
	tagsOffset   int64
	mkvmerge     bool
	inflateCap   int64 // bytes that a track may inflate in total
	scale        int64
	tracks       []trackEntry
}

// segmentScan holds what the scan of the top-level elements of the Segment found.
type segmentScan struct {
	positions map[uint64]int64 // element ID to position relative to the Segment data, from the SeekHeads
	info      ebml.Header
	tracks    ebml.Header
	hasInfo   bool
	hasTracks bool
}

func damaged(format string, args ...any) error {
	return &model.ParseError{Msg: fmt.Sprintf(format, args...)}
}

func unknownSize(h ebml.Header) error {
	return damaged("The element 0x%X at byte %d has an unknown size, which only Segment and Cluster may have.", h.ID, h.Offset)
}

// wrap turns the structure errors of the EBML reader into parse errors.
func wrap(err error) error {
	if ee, ok := errors.AsType[*ebml.Error](err); ok {
		return &model.ParseError{Msg: ee.Msg}
	}
	return err
}

// Open reads the headers and the track list. It reads no block.
func Open(r io.ReaderAt, size int64) (*File, error) {
	f := &File{r: ebml.NewReader(r, size), cuesOffset: noPosition, tagsOffset: noPosition, scale: defaultTimestampScale, inflateCap: maxTrackInflate}
	if err := f.readHeaders(); err != nil {
		return nil, wrap(err)
	}
	return f, nil
}

// Tracks returns the subtitle tracks in file order.
func (f *File) Tracks() []model.Track {
	tracks := make([]model.Track, len(f.tracks))
	for i, t := range f.tracks {
		tracks[i] = t.Track
	}
	return tracks
}

func (f *File) readHeaders() error {
	if err := f.readPrelude(); err != nil {
		return err
	}
	scan := segmentScan{positions: map[uint64]int64{}}
	if err := f.scanSegment(&scan); err != nil {
		return err
	}
	if err := f.fromSeekHead(&scan); err != nil {
		return err
	}
	if !scan.hasTracks {
		return damaged("The file has no Tracks element.")
	}
	if scan.hasInfo {
		if err := f.readInfo(scan.info); err != nil {
			return err
		}
	}
	return f.readTracks(scan.tracks)
}

// readPrelude checks the EBML header and finds the Segment.
func (f *File) readPrelude() error {
	h, ok, err := f.r.Header(0)
	if err != nil {
		return err
	}
	if !ok || h.ID != idEBML {
		return damaged("The file is not a Matroska or WebM file.")
	}
	docType, err := f.docType(h)
	if err != nil {
		return err
	}
	segment, ok, err := f.findSegment(h.Offset + h.Size)
	if err != nil {
		return err
	}
	if !ok || (docType != "matroska" && docType != "webm") {
		return damaged("The file is not a Matroska or WebM file.")
	}
	f.segmentStart = segment.Offset
	f.segmentEnd = math.MaxInt64
	if segment.Size != ebml.UnknownSize {
		f.segmentEnd = segment.Offset + segment.Size
	}
	f.firstCluster = f.segmentEnd
	return nil
}

func (f *File) docType(header ebml.Header) (string, error) {
	var docType string
	err := f.each(header, func(c ebml.Header) error {
		if c.ID != idDocType {
			return nil
		}
		if err := f.readString(c, &docType); err != nil {
			return err
		}
		return errStop
	})
	return docType, err
}

func (f *File) findSegment(from int64) (ebml.Header, bool, error) {
	for pos := from; ; {
		h, ok, err := f.r.Header(pos)
		if err != nil || !ok {
			return ebml.Header{}, false, err
		}
		if h.ID == idSegment {
			return h, true, nil
		}
		if h.Size == ebml.UnknownSize {
			return ebml.Header{}, false, unknownSize(h)
		}
		pos = h.Offset + h.Size
	}
}

// scanSegment walks the top-level elements of the Segment until the first Cluster.
func (f *File) scanSegment(scan *segmentScan) error {
	for pos := f.segmentStart; pos < f.segmentEnd; {
		e, ok, err := f.r.Header(pos)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if e.ID == idCluster {
			f.firstCluster = pos
			return nil
		}
		if err := f.record(e, pos, scan); err != nil {
			return err
		}
		if e.Size == ebml.UnknownSize {
			return unknownSize(e)
		}
		pos = e.Offset + e.Size
	}
	return nil
}

func (f *File) record(e ebml.Header, pos int64, scan *segmentScan) error {
	switch e.ID {
	case idSeekHead:
		return f.readSeekHead(e, scan.positions)
	case idInfo:
		scan.info, scan.hasInfo = e, true
	case idTracks:
		scan.tracks, scan.hasTracks = e, true
	case idCues:
		f.cuesOffset = pos
	case idTags:
		f.tagsOffset = pos
	}
	return nil
}

// fromSeekHead takes the elements that the scan did not find before the first Cluster from the SeekHead positions.
func (f *File) fromSeekHead(scan *segmentScan) error {
	if p, ok := scan.positions[idCues]; ok && f.cuesOffset == noPosition {
		f.cuesOffset = f.segmentStart + p
	}
	if p, ok := scan.positions[idTags]; ok && f.tagsOffset == noPosition {
		f.tagsOffset = f.segmentStart + p
	}
	var err error
	if !scan.hasInfo {
		scan.info, scan.hasInfo, err = f.elementAt(scan.positions, idInfo)
	}
	if err == nil && !scan.hasTracks {
		scan.tracks, scan.hasTracks, err = f.elementAt(scan.positions, idTracks)
	}
	return err
}

func (f *File) elementAt(positions map[uint64]int64, id uint64) (ebml.Header, bool, error) {
	p, ok := positions[id]
	if !ok || p > math.MaxInt64-f.segmentStart {
		return ebml.Header{}, false, nil
	}
	return f.r.Header(f.segmentStart + p)
}

// readSeekHead adds the positions of the SeekHead to positions. The first entry per ID wins.
func (f *File) readSeekHead(seekHead ebml.Header, positions map[uint64]int64) error {
	return f.each(seekHead, func(seek ebml.Header) error {
		if seek.ID != idSeek {
			return nil
		}
		var id, position uint64
		var hasID, hasPosition bool
		err := f.each(seek, func(c ebml.Header) error {
			switch c.ID {
			case idSeekID:
				hasID = true
				return f.readUint(c, &id)
			case idSeekPosition:
				hasPosition = true
				return f.readUint(c, &position)
			}
			return nil
		})
		if _, seen := positions[id]; err == nil && hasID && hasPosition && !seen {
			positions[id] = int64(position)
		}
		return err
	})
}

func (f *File) readInfo(info ebml.Header) error {
	return f.each(info, func(c ebml.Header) error {
		switch c.ID {
		case idTimestampScale:
			var v uint64
			err := f.readUint(c, &v)
			f.scale = int64(v)
			return err
		case idWritingApp:
			var s string
			err := f.readString(c, &s)
			f.mkvmerge = strings.HasPrefix(s, mkvmergeWritingApp)
			return err
		}
		return nil
	})
}

func (f *File) readTracks(tracks ebml.Header) error {
	return f.each(tracks, func(entry ebml.Header) error {
		if entry.ID != idTrackEntry {
			return nil
		}
		t, ok, err := f.readTrackEntry(entry)
		if err != nil || !ok {
			return err
		}
		if i := slices.IndexFunc(f.tracks, func(o trackEntry) bool { return o.Number == t.Number }); i >= 0 {
			f.tracks[i] = t
		} else {
			f.tracks = append(f.tracks, t)
		}
		return nil
	})
}

// entryReader reads the children of a TrackEntry.
type entryReader struct {
	f         *File
	t         trackEntry
	number    uint64
	kind      uint64
	hasNumber bool
	language  string
	bcp47     string
	hasLang   bool
	hasBCP47  bool
}

func (f *File) readTrackEntry(entry ebml.Header) (trackEntry, bool, error) {
	e := entryReader{f: f, t: trackEntry{defaultDuration: noDuration}}
	e.t.Container = model.Matroska
	e.t.Default = true
	if err := f.each(entry, e.child); err != nil {
		return trackEntry{}, false, err
	}
	if e.kind != trackTypeSubtitle || !e.hasNumber || e.number > math.MaxInt32 {
		return trackEntry{}, false, nil
	}
	e.t.Number = int(e.number)
	e.t.Format = codecFormat(e.t.CodecID)
	switch {
	case e.hasBCP47:
		e.t.Language = e.bcp47
	case e.hasLang:
		e.t.Language = e.language
	default:
		e.t.Language = defaultLanguage
	}
	return e.t, true, nil
}

func (e *entryReader) child(c ebml.Header) error {
	switch c.ID {
	case idTrackNumber:
		e.hasNumber = true
		return e.f.readUint(c, &e.number)
	case idTrackType:
		return e.f.readUint(c, &e.kind)
	case idTrackUID:
		var b []byte
		err := e.f.readBytes(c, &b)
		e.t.uid = strings.TrimLeft(string(b), "\x00")
		return err
	case idCodecID:
		return e.f.readString(c, &e.t.CodecID)
	case idCodecPrivate:
		return e.f.readBytes(c, &e.t.codecPrivate)
	case idLanguage:
		e.hasLang = true
		return e.f.readString(c, &e.language)
	case idLanguageBCP47:
		e.hasBCP47 = true
		return e.f.readString(c, &e.bcp47)
	case idName:
		return e.f.readString(c, &e.t.Name)
	}
	return e.flags(c)
}

func (e *entryReader) flags(c ebml.Header) error {
	var v uint64
	switch c.ID {
	case idFlagDefault:
		err := e.f.readUint(c, &v)
		e.t.Default = v != 0
		return err
	case idFlagForced:
		err := e.f.readUint(c, &v)
		e.t.Forced = v != 0
		return err
	case idDefaultDuration:
		err := e.f.readUint(c, &v)
		e.t.defaultDuration = int64(v)
		return err
	case idEncodings:
		var err error
		e.t.encodings, err = e.f.readEncodings(c)
		return err
	}
	return nil
}

// codecFormat returns the format that Extract reads from a codec, "" for a codec it cannot read.
func codecFormat(codecID string) model.Format {
	switch codecID {
	case "S_TEXT/UTF8":
		return model.SubRip
	case "S_TEXT/ASS", "S_TEXT/SSA":
		return model.ASS
	case "S_TEXT/WEBVTT", "D_WEBVTT/SUBTITLES", "D_WEBVTT/CAPTIONS", "D_WEBVTT/DESCRIPTIONS":
		return model.WebVTT
	default:
		return ""
	}
}

// readEncodings returns the content encodings in decoding order, the highest order first.
func (f *File) readEncodings(encodings ebml.Header) ([]encoding, error) {
	type ordered struct {
		order uint64
		encoding
	}
	var list []ordered
	err := f.each(encodings, func(e ebml.Header) error {
		if e.ID != idEncoding {
			return nil
		}
		o := ordered{encoding: encoding{scope: 1}}
		err := f.each(e, func(c ebml.Header) error { return f.readEncodingField(c, &o.order, &o.encoding) })
		list = append(list, o)
		return err
	})
	slices.SortStableFunc(list, func(a, b ordered) int { return cmp.Compare(b.order, a.order) })
	out := make([]encoding, len(list))
	for i, o := range list {
		out[i] = o.encoding
	}
	return out, err
}

func (f *File) readEncodingField(c ebml.Header, order *uint64, e *encoding) error {
	switch c.ID {
	case idEncodingOrder:
		return f.readUint(c, order)
	case idEncodingScope:
		return f.readUint(c, &e.scope)
	case idEncodingType:
		return f.readUint(c, &e.kind)
	case idCompression:
		return f.each(c, func(c ebml.Header) error {
			switch c.ID {
			case idCompAlgo:
				return f.readUint(c, &e.algo)
			case idCompSettings:
				return f.readBytes(c, &e.settings)
			}
			return nil
		})
	}
	return nil
}
