package mkv

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/ebml"
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
	err = f.each(cues, func(point ebml.Header) error {
		if point.ID != idCuePoint {
			return nil
		}
		return f.each(point, func(tp ebml.Header) error {
			if tp.ID != idCuePositions {
				return nil
			}
			p, found, err := f.readCuePosition(tp, number)
			if found {
				positions = append(positions, p)
			}
			return err
		})
	})
	slices.SortFunc(positions, func(a, b cuePosition) int {
		return cmp.Or(cmp.Compare(a.cluster, b.cluster), cmp.Compare(a.relative, b.relative))
	})
	return slices.Compact(positions), err == nil
}

// readCuePosition reads a CueTrackPositions. found is false for another track.
func (f *File) readCuePosition(tp ebml.Header, number int) (p cuePosition, found bool, err error) {
	var track, cluster, relative uint64
	var hasTrack, hasCluster, hasRelative bool
	err = f.each(tp, func(c ebml.Header) error {
		switch c.ID {
		case idCueTrack:
			hasTrack = true
			return f.readUint(c, &track)
		case idCueCluster:
			hasCluster = true
			return f.readUint(c, &cluster)
		case idCueRelative:
			hasRelative = true
			return f.readUint(c, &relative)
		}
		return nil
	})
	if err != nil || !hasTrack || track != uint64(number) {
		return cuePosition{}, false, err
	}
	if !hasCluster || !hasRelative {
		return cuePosition{}, false, errIncompleteCue
	}
	return cuePosition{cluster: f.segmentStart + int64(cluster), relative: int64(relative)}, true, nil
}

// frameCount returns the NUMBER_OF_FRAMES statistics tag of the track with the TrackUID uid.
func (f *File) frameCount(uid string) (int, bool) {
	tags, ok, err := f.r.Header(f.tagsOffset)
	if err != nil || !ok || tags.ID != idTags {
		return 0, false
	}
	count, found := 0, false
	err = f.each(tags, func(tag ebml.Header) error {
		if tag.ID != idTag {
			return nil
		}
		n, ok, err := f.tagFrames(tag, uid)
		if ok {
			count, found = n, true
			return errStop
		}
		return err
	})
	return count, found && err == nil
}

// tagFrames returns the NUMBER_OF_FRAMES count of a Tag that targets the track uid.
func (f *File) tagFrames(tag ebml.Header, uid string) (count int, ok bool, err error) {
	var uids []string
	hasCount := false
	err = f.each(tag, func(c ebml.Header) error {
		switch c.ID {
		case idTargets:
			return f.each(c, func(target ebml.Header) error {
				if target.ID != idTagTrackUID {
					return nil
				}
				var b []byte
				err := f.readBytes(target, &b)
				uids = append(uids, string(bytes.TrimLeft(b, "\x00")))
				return err
			})
		case idSimpleTag:
			if n, ok := f.numberOfFrames(c); ok && !hasCount {
				count, hasCount = n, true
			}
		}
		return nil
	})
	return count, hasCount && slices.Contains(uids, uid), err
}

func (f *File) numberOfFrames(simpleTag ebml.Header) (int, bool) {
	var name, value string
	err := f.each(simpleTag, func(c ebml.Header) error {
		switch c.ID {
		case idTagName:
			return f.readString(c, &name)
		case idTagString:
			return f.readString(c, &value)
		}
		return nil
	})
	if err != nil || name != "NUMBER_OF_FRAMES" {
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
	var v uint64
	if f.readUint(ts, &v) != nil {
		return clusterInfo{}, false
	}
	return clusterInfo{dataOffset: cluster.Offset, time: int64(v)}, true
}
