package mp4

import (
	"encoding/binary"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"github.com/Eyevinn/mp4ff/mp4"
)

// timeLimit is 100000 hours in seconds.
const timeLimit = 100000 * 3600

// sample is a sample with its position in the file and its times in seconds.
type sample struct {
	start, end *big.Rat
	offset     uint64
	size       uint32
}

// walker yields the samples of a track chunk by chunk, as the sample tables list them.
// It never allocates per sample, apart from the times.
type walker struct {
	t        *track
	fileSize uint64

	count   uint32 // number of samples in the size table
	uniform uint32 // the size of every sample, or 0 when sizes or packed hold them
	sizes   []uint32
	packed  []byte // the sizes of a stz2 box
	bits    uint8  // the bits of one packed size

	deltaCounts []uint32
	deltas      []uint32
	chunks      []mp4.StscEntry
	offsets32   []uint32
	offsets64   []uint64

	chunk, entry int // the next chunk, and the stsc entry that holds the chunk before it
	inChunk, per uint32
	offset       uint64
	index        uint32
	deltaAt      int
	left, delta  uint32
	ticks        uint64
}

// newWalker checks the sample tables of t. Each count is checked before anything is allocated.
func newWalker(t *track, fileSize int64) (*walker, error) {
	w := &walker{t: t, fileSize: uint64(fileSize), entry: -1}
	var children []mp4.Box
	if t.stbl != nil {
		children = t.stbl.Children
	}
	if err := w.readSizes(children); err != nil {
		return nil, err
	}
	stts := first[*mp4.SttsBox](children)
	if stts == nil {
		return nil, w.missing("stts")
	}
	w.deltaCounts, w.deltas = stts.SampleCount, stts.SampleTimeDelta
	stsc := first[*mp4.StscBox](children)
	if stsc == nil {
		return nil, w.missing("stsc")
	}
	w.chunks = stsc.Entries
	if co64 := first[*mp4.Co64Box](children); co64 != nil {
		w.offsets64 = co64.ChunkOffset
	} else if stco := first[*mp4.StcoBox](children); stco != nil {
		w.offsets32 = stco.ChunkOffset
	} else {
		return nil, w.missing("stco")
	}
	return w, nil
}

func (w *walker) missing(box string) error {
	return parseError("Track " + strconv.Itoa(w.t.info.Number) + " has no " + box + " box.")
}

func (w *walker) readSizes(children []mp4.Box) error {
	if stsz := first[*mp4.StszBox](children); stsz != nil {
		return w.readStsz(stsz)
	}
	for _, c := range children {
		if u, ok := c.(*mp4.UnknownBox); ok && u.Type() == "stz2" {
			return w.readStz2(u.Payload())
		}
	}
	return w.missing("stsz or stz2")
}

func (w *walker) readStsz(stsz *mp4.StszBox) error {
	if stsz.SampleUniformSize == 0 {
		w.count, w.sizes = uint32(len(stsz.SampleSize)), stsz.SampleSize
		return nil
	}
	if uint64(stsz.SampleNumber)*uint64(stsz.SampleUniformSize) > w.fileSize {
		return parseError("The stsz box of track " + strconv.Itoa(w.t.info.Number) + " holds " + strconv.FormatUint(uint64(stsz.SampleNumber), 10) +
			" samples of " + strconv.FormatUint(uint64(stsz.SampleUniformSize), 10) + " bytes, which do not fit into the file.")
	}
	w.count, w.uniform = stsz.SampleNumber, stsz.SampleUniformSize
	return nil
}

// readStz2 reads the payload of a stz2 box: version and flags, 3 reserved bytes, the field size, the count, the sizes.
func (w *walker) readStz2(payload []byte) error {
	if len(payload) < 12 {
		return w.badStz2()
	}
	fieldSize, count, data := payload[7], binary.BigEndian.Uint32(payload[8:]), payload[12:]
	if (fieldSize != 4 && fieldSize != 8 && fieldSize != 16) || uint64(len(data))*8 < uint64(count)*uint64(fieldSize) {
		return w.badStz2()
	}
	w.count, w.packed, w.bits = count, data, fieldSize
	return nil
}

func (w *walker) badStz2() error {
	return parseError("The stz2 box of track " + strconv.Itoa(w.t.info.Number) + " is not valid.")
}

func (w *walker) sizeOf(i uint32) uint32 {
	switch {
	case w.uniform != 0:
		return w.uniform
	case w.bits == 0:
		return w.sizes[i]
	case w.bits == 4:
		b := w.packed[i>>1]
		if i&1 == 0 {
			return uint32(b >> 4)
		}
		return uint32(b & 0x0F)
	case w.bits == 8:
		return uint32(w.packed[i])
	default:
		return uint32(binary.BigEndian.Uint16(w.packed[i*2:]))
	}
}

func (w *walker) chunkCount() int {
	if w.offsets64 != nil {
		return len(w.offsets64)
	}
	return len(w.offsets32)
}

func (w *walker) chunkOffset(i int) uint64 {
	if w.offsets64 != nil {
		return w.offsets64[i]
	}
	return uint64(w.offsets32[i])
}

// next returns the next sample that ends after 0 s. It returns false after the last sample.
func (w *walker) next() (sample, bool, error) {
	for w.index < w.count {
		if w.inChunk == w.per {
			if w.chunk >= w.chunkCount() {
				return sample{}, false, nil
			}
			w.startChunk()
			continue
		}
		for w.left == 0 {
			if w.deltaAt >= len(w.deltas) {
				return sample{}, false, parseError("The stts box of track " + strconv.Itoa(w.t.info.Number) +
					" has fewer samples than the sample size box.")
			}
			w.left, w.delta = w.deltaCounts[w.deltaAt], w.deltas[w.deltaAt]
			w.deltaAt++
		}
		s := sample{offset: w.offset, size: w.sizeOf(w.index)}
		startTicks := new(big.Int).SetUint64(w.ticks)
		endTicks := new(big.Int).Add(startTicks, new(big.Int).SetUint64(uint64(w.delta)))
		w.ticks += uint64(w.delta)
		if sum, carry := bits.Add64(w.offset, uint64(s.size), 0); carry == 0 {
			w.offset = sum
		} else {
			w.offset = math.MaxUint64
		}
		w.index++
		w.inChunk++
		w.left--

		s.end = w.seconds(endTicks)
		if s.end.Sign() <= 0 {
			continue
		}
		if s.end.Cmp(new(big.Rat).SetInt64(timeLimit)) >= 0 {
			return sample{}, false, parseError("The sample of track " + strconv.Itoa(w.t.info.Number) + " at byte " +
				strconv.FormatUint(s.offset, 10) + " ends at " + decimals(s.end) + " s. Times must be below 100000 hours.")
		}
		s.start = w.seconds(startTicks)
		if s.start.Sign() < 0 {
			s.start.SetInt64(0)
		}
		return s, true, nil
	}
	return sample{}, false, nil
}

// startChunk moves to the next chunk and reads its sample count from the stsc entry that covers it.
func (w *walker) startChunk() {
	for w.entry+1 < len(w.chunks) && uint64(w.chunks[w.entry+1].FirstChunk) <= uint64(w.chunk)+1 {
		w.entry++
	}
	w.per, w.inChunk = 0, 0
	if w.entry >= 0 {
		w.per = w.chunks[w.entry].SamplesPerChunk
	}
	w.offset = w.chunkOffset(w.chunk)
	w.chunk++
}

// seconds returns ticks in seconds, moved by the edit list.
func (w *walker) seconds(ticks *big.Int) *big.Rat {
	r := new(big.Rat).SetFrac(ticks, big.NewInt(int64(w.t.timescale)))
	return r.Add(r, w.t.delay)
}

// outside reports whether the sample does not lie inside the file.
func (w *walker) outside(s sample) error {
	if s.offset > w.fileSize || uint64(s.size) > w.fileSize-s.offset {
		return parseError("A sample of track " + strconv.Itoa(w.t.info.Number) + " at byte " +
			strconv.FormatUint(s.offset, 10) + " lies outside the file.")
	}
	return nil
}

// decimals returns r with up to 3 decimals.
func decimals(r *big.Rat) string {
	s := r.FloatString(3)
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}

// millis returns r seconds as a duration of whole milliseconds.
func millis(r *big.Rat) time.Duration {
	return time.Duration(roundTo(r, 1000)) * time.Millisecond
}

// nanos returns r seconds as a duration of whole nanoseconds.
func nanos(r *big.Rat) time.Duration {
	return time.Duration(roundTo(r, int64(time.Second)))
}

// roundTo returns r times scale, rounded half away from zero.
func roundTo(r *big.Rat, scale int64) int64 {
	n := new(big.Int).Mul(r.Num(), big.NewInt(scale))
	q, rem := new(big.Int).QuoRem(n, r.Denom(), new(big.Int))
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	if twice.Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(int64(n.Sign())))
	}
	return q.Int64()
}
