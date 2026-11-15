package mkv

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mkv/internal/ebml"
)

var errIncompleteCue = errors.New("a CueTrackPositions without a cluster or block position")

type cuePosition struct {
	cluster  int64 // byte offset of the cluster
	relative int64 // offset of the block in the cluster data
}

type clusterInfo struct {
	dataOffset int64
	time       int64
}

// readListedBlocks reads only the blocks that the Cues of an mkvmerge file list for the track. mkvmerge lists every
// subtitle block and counts the blocks of a track in its NUMBER_OF_FRAMES tag. It returns false when the caller must
// walk all clusters.
func (f *File) readListedBlocks(t *trackEntry, budget *int64) ([]block, bool) {
	positions, ok := f.listedPositions(t)
	if !ok {
		return nil, false
	}
	clusters := map[int64]clusterInfo{}
	blocks := make([]block, 0, len(positions))
	for _, p := range positions {
		b, ok := f.listedBlock(t, p, clusters, budget)
		if !ok {
			return nil, false
		}
		blocks = append(blocks, b)
	}
	return blocks, true
}

func (f *File) listedPositions(t *trackEntry) ([]cuePosition, bool) {
	if !f.mkvmerge || f.cuesOffset == noPosition || f.tagsOffset == noPosition || t.uid == "" {
		return nil, false
	}
	positions, ok := f.cuePositions(t.Number)
	if !ok || len(positions) == 0 {
		return nil, false
	}
	frames, ok := f.frameCount(t.uid)
	return positions, ok && frames == len(positions)
}

func (f *File) listedBlock(t *trackEntry, p cuePosition, clusters map[int64]clusterInfo, budget *int64) (block, bool) {
	c, seen := clusters[p.cluster]
	if !seen {
		var ok bool
		if c, ok = f.clusterStart(p.cluster); !ok {
			return block{}, false
		}
		clusters[p.cluster] = c
	}
	h, ok, err := f.r.Header(c.dataOffset + p.relative)
	if err != nil || !ok {
		return block{}, false
	}
	b, found, err := f.readBlockElement(h, t.Number)
	if err != nil || !found || f.finishBlock(t, h.Offset, &b, c.time, budget) != nil {
		return block{}, false
	}
	return b, true
}

// cuePositions returns the blocks that the Cues list for the track in file order, without duplicates.
func (f *File) cuePositions(number int) ([]cuePosition, bool) {
	cues, ok, err := f.r.Header(f.cuesOffset)
	if err != nil || !ok || cues.ID != idCues {
		return nil, false
	}
	var positions []cuePosition
	for point, err := range f.r.Children(cues) {
		if err != nil {
			return nil, false
		}
		if point.ID != idCuePoint {
			continue
		}
		if positions, err = f.appendCuePositions(positions, point, number); err != nil {
			return nil, false
		}
	}
	slices.SortFunc(positions, func(a, b cuePosition) int {
		return cmp.Or(cmp.Compare(a.cluster, b.cluster), cmp.Compare(a.relative, b.relative))
	})
	return slices.Compact(positions), true
}

func (f *File) appendCuePositions(positions []cuePosition, point ebml.Header, number int) ([]cuePosition, error) {
	for tp, err := range f.r.Children(point) {
		if err != nil {
			return nil, err
		}
		if tp.ID != idCuePositions {
			continue
		}
		p, found, err := f.readCuePosition(tp, number)
		if err != nil {
			return nil, err
		}
		if found {
			positions = append(positions, p)
		}
	}
	return positions, nil
}

// readCuePosition reads a CueTrackPositions. found is false for another track.
func (f *File) readCuePosition(tp ebml.Header, number int) (cuePosition, bool, error) {
	c, err := f.readCueFields(tp)
	if err != nil || !c.hasTrack || c.track != uint64(number) {
		return cuePosition{}, false, err
	}
	if !c.hasCluster || !c.hasRelative {
		return cuePosition{}, false, errIncompleteCue
	}
	return cuePosition{cluster: f.segmentStart + int64(c.cluster), relative: int64(c.relative)}, true, nil
}

type cueFields struct {
	track, cluster, relative          uint64
	hasTrack, hasCluster, hasRelative bool
}

func (f *File) readCueFields(tp ebml.Header) (c cueFields, err error) {
	for e, err := range f.r.Children(tp) {
		if err != nil {
			return cueFields{}, err
		}
		switch e.ID {
		case idCueTrack:
			c.hasTrack = true
			c.track, err = f.r.Unsigned(e)
		case idCueCluster:
			c.hasCluster = true
			c.cluster, err = f.r.Unsigned(e)
		case idCueRelative:
			c.hasRelative = true
			c.relative, err = f.r.Unsigned(e)
		}
		if err != nil {
			return cueFields{}, err
		}
	}
	return c, nil
}

// frameCount returns the NUMBER_OF_FRAMES statistics tag of the track with the TrackUID uid.
func (f *File) frameCount(uid string) (int, bool) {
	tags, ok, err := f.r.Header(f.tagsOffset)
	if err != nil || !ok || tags.ID != idTags {
		return 0, false
	}
	for tag, err := range f.r.Children(tags) {
		if err != nil {
			return 0, false
		}
		if tag.ID != idTag {
			continue
		}
		n, ok, err := f.tagFrames(tag, uid)
		if ok {
			return n, true
		}
		if err != nil {
			return 0, false
		}
	}
	return 0, false
}

// tagFrames returns the NUMBER_OF_FRAMES count of a Tag that targets the track uid.
func (f *File) tagFrames(tag ebml.Header, uid string) (count int, ok bool, err error) {
	var uids []string
	hasCount := false
	for c, childErr := range f.r.Children(tag) {
		if childErr != nil {
			err = childErr
			break
		}
		switch c.ID {
		case idTargets:
			uids, err = f.targetUIDs(c, uids)
		case idSimpleTag:
			if !hasCount {
				count, hasCount = f.numberOfFrames(c)
			}
		}
		if err != nil {
			break
		}
	}
	return count, hasCount && slices.Contains(uids, uid), err
}

func (f *File) targetUIDs(targets ebml.Header, uids []string) ([]string, error) {
	for target, err := range f.r.Children(targets) {
		if err != nil {
			return uids, err
		}
		if target.ID != idTagTrackUID {
			continue
		}
		b, err := f.r.Bytes(target)
		uids = append(uids, string(bytes.TrimLeft(b, "\x00")))
		if err != nil {
			return uids, err
		}
	}
	return uids, nil
}

func (f *File) numberOfFrames(simpleTag ebml.Header) (int, bool) {
	var name, value string
	for c, err := range f.r.Children(simpleTag) {
		if err != nil {
			return 0, false
		}
		switch c.ID {
		case idTagName:
			name, err = f.r.String(c)
		case idTagString:
			value, err = f.r.String(c)
		}
		if err != nil {
			return 0, false
		}
	}
	if name != "NUMBER_OF_FRAMES" {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 31)
	return int(n), err == nil
}

// clusterStart returns the offset of the cluster data and the cluster Timestamp, which mkvmerge writes as the first child.
func (f *File) clusterStart(offset int64) (clusterInfo, bool) {
	cluster, ok, err := f.r.Header(offset)
	if err != nil || !ok || cluster.ID != idCluster {
		return clusterInfo{}, false
	}
	ts, ok, err := f.r.Header(cluster.Offset)
	if err != nil || !ok || ts.ID != idTimestamp {
		return clusterInfo{}, false
	}
	v, err := f.r.Unsigned(ts)
	if err != nil {
		return clusterInfo{}, false
	}
	return clusterInfo{dataOffset: cluster.Offset, time: int64(v)}, true
}
