package mkv

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"
	"math"
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ebml"
)

const (
	maxBlockHeader  = 11
	blockHeaderTail = 3
	lacingMask      = 0x06
	maxInflatedSize = 64 << 20
	maxTrackInflate = 1 << 30
	webVTTAddID     = 1
	scopeFrames     = 1
	scopeCodecPriv  = 2
	algoZlib        = 0
	algoHeaderStrip = 3
	maxHours        = 100_000
	noDuration      = -1
	limitNanos      = int64(maxHours) * 3600 * 1_000_000_000
)

// block is one block of a track. Times are in ticks of the TimestampScale.
type block struct {
	start      int64
	duration   int64 // noDuration without a BlockDuration
	data       []byte
	additional []byte
}

func (f *File) track(number int) (*trackEntry, bool) {
	i := slices.IndexFunc(f.tracks, func(t trackEntry) bool { return t.Number == number })
	if i < 0 {
		return nil, false
	}
	return &f.tracks[i], true
}

// readBlocks reads the blocks of a track, through the Cues when the file allows it, else with a walk over all clusters.
func (f *File) readBlocks(t *trackEntry) ([]block, error) {
	budget := f.inflateCap
	if blocks, ok := f.readListedBlocks(t, &budget); ok {
		return blocks, nil
	}
	budget = f.inflateCap
	return f.walkBlocks(t, &budget)
}

// walkBlocks reads all clusters. A cluster of unknown size ends at the next cluster.
func (f *File) walkBlocks(t *trackEntry, budget *int64) ([]block, error) {
	var blocks []block
	var clusterTime int64
	for pos := f.firstCluster; pos < f.segmentEnd; {
		h, ok, err := f.r.Header(pos)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if h.ID == idCluster {
			clusterTime, pos = 0, h.Offset
			continue
		}
		b, found, err := f.walkStep(h, t, &clusterTime, budget)
		if err != nil {
			return nil, err
		}
		if found {
			blocks = append(blocks, b)
		}
		if h.Size == ebml.UnknownSize {
			return nil, unknownSize(h)
		}
		pos = h.Offset + h.Size
	}
	return blocks, nil
}

// walkStep handles one element of a cluster: a Timestamp sets the cluster time, a block of the track is returned.
func (f *File) walkStep(h ebml.Header, t *trackEntry, clusterTime *int64, budget *int64) (block, bool, error) {
	if h.ID == idTimestamp {
		var v uint64
		err := f.readUint(h, &v)
		*clusterTime = int64(v)
		return block{}, false, err
	}
	b, found, err := f.readBlockElement(h, t.Number)
	if err != nil || !found {
		return block{}, false, err
	}
	return b, true, f.finishBlock(t, h.Offset, &b, *clusterTime, budget)
}

// readBlockElement reads a SimpleBlock or a BlockGroup. found is false for another element and for a block of another track.
func (f *File) readBlockElement(h ebml.Header, number int) (b block, found bool, err error) {
	switch h.ID {
	case idSimpleBlock:
		return f.readBlock(h, number)
	case idBlockGroup:
		return f.readBlockGroup(h, number)
	}
	return block{}, false, nil
}

// finishBlock adds the cluster time to the block time, checks the time limit and decodes the block data.
func (f *File) finishBlock(t *trackEntry, offset int64, b *block, clusterTime int64, budget *int64) error {
	if b.start > 0 && clusterTime > math.MaxInt64-b.start {
		b.start = math.MaxInt64
	} else {
		b.start += clusterTime
	}
	if !f.withinLimit(t, b) {
		return damaged("The block of track %d at byte %d has the time %d at a TimestampScale of %d ns. Times must be below %d hours.",
			t.Number, offset, b.start, f.scale, maxHours)
	}
	data, err := f.decode(t, b.data, scopeFrames, budget)
	b.data = data
	return err
}

// withinLimit reports whether the start and the end of b are below 100,000 hours, so that later time arithmetic cannot overflow.
func (f *File) withinLimit(t *trackEntry, b *block) bool {
	start, ok := f.nanos(b.start)
	if !ok {
		return false
	}
	var span int64
	if b.duration != noDuration {
		if span, ok = f.nanos(b.duration); !ok {
			return false
		}
	} else if t.defaultDuration != noDuration {
		span = t.defaultDuration
	}
	if start > math.MaxInt64-span {
		return false
	}
	return start > -limitNanos && start+span < limitNanos
}

// nanos converts ticks to nanoseconds. It returns false on overflow.
func (f *File) nanos(ticks int64) (int64, bool) {
	if ticks == 0 || f.scale == 0 {
		return 0, true
	}
	n := ticks * f.scale
	return n, n/f.scale == ticks
}

func (f *File) readBlockGroup(group ebml.Header, number int) (block, bool, error) {
	b := block{duration: noDuration}
	found := false
	err := f.each(group, func(c ebml.Header) error {
		switch c.ID {
		case idBlock:
			blk, ok, err := f.readBlock(c, number)
			if err != nil || !ok {
				return errOr(err, errStop)
			}
			b.start, b.data, found = blk.start, blk.data, true
		case idBlockDuration:
			var v uint64
			err := f.readUint(c, &v)
			b.duration = int64(v)
			return err
		case idBlockAdditions:
			return f.readAdditions(c, &b.additional)
		}
		return nil
	})
	return b, found, err
}

// errOr returns err, or fallback when err is nil.
func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// readAdditions reads the first BlockAdditional with the BlockAddID 1, the WebVTT side data.
func (f *File) readAdditions(additions ebml.Header, dst *[]byte) error {
	return f.each(additions, func(more ebml.Header) error {
		if more.ID != idBlockMore {
			return nil
		}
		data, found, err := f.readMore(more)
		if found {
			*dst = data
			return errStop
		}
		return err
	})
}

func (f *File) readMore(more ebml.Header) (data []byte, found bool, err error) {
	id := uint64(webVTTAddID)
	err = f.each(more, func(c ebml.Header) error {
		switch c.ID {
		case idBlockAddID:
			return f.readUint(c, &id)
		case idBlockAdditional:
			return f.readBytes(c, &data)
		}
		return nil
	})
	return data, id == webVTTAddID && data != nil, err
}

// readBlock reads the header and the data of a SimpleBlock or Block. found is false for a block of another track, which
// costs at most 11 bytes of reads.
func (f *File) readBlock(h ebml.Header, number int) (b block, found bool, err error) {
	head, err := f.r.Bytes(ebml.Header{ID: h.ID, Size: min(h.Size, maxBlockHeader), Offset: h.Offset})
	if err != nil {
		return block{}, false, err
	}
	blockTrack, n, ok := ebml.ReadVint(head, false)
	if !ok || len(head) < n+blockHeaderTail {
		return block{}, false, damaged("The block header at byte %d is not valid.", h.Offset)
	}
	if blockTrack != uint64(number) {
		return block{}, false, nil
	}
	if head[n+2]&lacingMask != 0 {
		return block{}, false, damaged("A block of track %d uses lacing, which subtitle tracks do not use.", number)
	}
	dataStart := int64(n + blockHeaderTail)
	data, err := f.r.Bytes(ebml.Header{ID: h.ID, Size: h.Size - dataStart, Offset: h.Offset + dataStart})
	if err != nil {
		return block{}, false, err
	}
	rel := int16(uint16(head[n])<<8 | uint16(head[n+1]))
	return block{start: int64(rel), duration: noDuration, data: data}, true, nil
}

// decode undoes the content encodings of the track that apply to scope. budget is the number of bytes that the
// track may still inflate.
func (f *File) decode(t *trackEntry, data []byte, scope uint64, budget *int64) ([]byte, error) {
	for _, e := range t.encodings {
		if e.scope&scope == 0 {
			continue
		}
		if e.kind != 0 || (e.algo != algoZlib && e.algo != algoHeaderStrip) {
			return nil, damaged("Track %d uses encryption or a compression other than zlib and header stripping.", t.Number)
		}
		if e.algo == algoHeaderStrip {
			data = slices.Concat(e.settings, data)
			continue
		}
		inflated, err := inflate(data)
		if err != nil {
			return nil, damaged("The zlib data of track %d is not valid or inflates to more than %d MB.", t.Number, maxInflatedSize>>20)
		}
		if *budget -= int64(len(inflated)); *budget < 0 {
			return nil, damaged("The zlib data of track %d inflates to more than %d MB in total.", t.Number, f.inflateCap>>20)
		}
		data = inflated
	}
	return data, nil
}

var errInflate = errors.New("the data does not inflate")

func inflate(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errInflate
	}
	out, err := io.ReadAll(io.LimitReader(zr, maxInflatedSize+1))
	if err != nil || len(out) > maxInflatedSize {
		return nil, errInflate
	}
	return out, nil
}
