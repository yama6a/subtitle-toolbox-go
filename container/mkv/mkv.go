// Package mkv reads the subtitle tracks of Matroska and WebM files.
//
// Elements: https://www.matroska.org/technical/elements.html
// Subtitle codecs: https://www.matroska.org/technical/subtitles.html
package mkv

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v5/container/mkv/internal/ebml"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

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
	for c, err := range f.r.Children(header) {
		if err != nil {
			return "", err
		}
		if c.ID == idDocType {
			return f.r.String(c)
		}
	}
	return "", nil
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
	for seek, err := range f.r.Children(seekHead) {
		if err != nil {
			return err
		}
		if seek.ID != idSeek {
			continue
		}
		id, position, ok, err := f.readSeek(seek)
		if err != nil {
			return err
		}
		if _, seen := positions[id]; ok && !seen {
			positions[id] = int64(position)
		}
	}
	return nil
}

// readSeek reads a Seek entry. ok is false when the entry lacks the ID or the position.
func (f *File) readSeek(seek ebml.Header) (id, position uint64, ok bool, err error) {
	var hasID, hasPosition bool
	for c, err := range f.r.Children(seek) {
		if err != nil {
			return 0, 0, false, err
		}
		switch c.ID {
		case idSeekID:
			hasID = true
			if id, err = f.r.Unsigned(c); err != nil {
				return 0, 0, false, err
			}
		case idSeekPosition:
			hasPosition = true
			if position, err = f.r.Unsigned(c); err != nil {
				return 0, 0, false, err
			}
		}
	}
	return id, position, hasID && hasPosition, nil
}

func (f *File) readInfo(info ebml.Header) error {
	for c, err := range f.r.Children(info) {
		if err != nil {
			return err
		}
		switch c.ID {
		case idTimestampScale:
			v, err := f.r.Unsigned(c)
			if err != nil {
				return err
			}
			f.scale = int64(v)
		case idWritingApp:
			s, err := f.r.String(c)
			if err != nil {
				return err
			}
			f.mkvmerge = strings.HasPrefix(s, mkvmergeWritingApp)
		}
	}
	return nil
}
