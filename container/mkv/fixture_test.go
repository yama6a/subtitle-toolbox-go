package mkv_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mkv"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
)

const (
	idSegment  = 0x18538067
	idTracks   = 0x1654AE6B
	idInfo     = 0x1549A966
	idCluster  = 0x1F43B675
	idCues     = 0x1C53BB6B
	idTags     = 0x1254C367
	trackVideo = 1
	trackSub   = 0x11
	lacingXiph = 0x02
)

func idBytes(id uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, id)
	return bytes.TrimLeft(b, "\x00")
}

func sizeBytes(size uint64) []byte {
	length := 1
	for size >= 1<<(7*length)-1 {
		length++
	}
	b := binary.BigEndian.AppendUint64(nil, size|1<<(7*length))
	return b[8-length:]
}

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func element(id uint32, data []byte) []byte {
	return cat(idBytes(id), sizeBytes(uint64(len(data))), data)
}

func unknownSizeElement(id uint32, data []byte) []byte {
	return cat(idBytes(id), []byte{1, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, data)
}

func uintElement(id uint32, v uint64) []byte {
	b := bytes.TrimLeft(binary.BigEndian.AppendUint64(nil, v), "\x00")
	if len(b) == 0 {
		b = []byte{0}
	}
	return element(id, b)
}

func ebmlHeader(docType string) []byte {
	return element(0x1A45DFA3, cat(uintElement(0x4286, 1), uintElement(0x42F7, 1), uintElement(0x42F2, 4),
		uintElement(0x42F3, 8), element(0x4282, []byte(docType)), uintElement(0x4287, 4), uintElement(0x4285, 2)))
}

func info(writingApp string) []byte {
	return element(idInfo, cat(uintElement(0x2AD7B1, 1000000), element(0x4D80, []byte("fixture")), element(0x5741, []byte(writingApp))))
}

type trackFields struct {
	number       uint64
	kind         uint64
	codecID      string
	codecPrivate []byte
	encodings    []byte
}

func trackEntry(t trackFields) []byte {
	data := cat(uintElement(0xD7, t.number), uintElement(0x73C5, t.number*1000), uintElement(0x83, t.kind), element(0x86, []byte(t.codecID)))
	if t.codecPrivate != nil {
		data = cat(data, element(0x63A2, t.codecPrivate))
	}
	if t.encodings != nil {
		data = cat(data, element(0x6D80, t.encodings))
	}
	return element(0xAE, data)
}

func subTrack(number uint64, codecID string) []byte {
	return trackEntry(trackFields{number: number, kind: trackSub, codecID: codecID})
}

func tracks(entries ...[]byte) []byte { return element(idTracks, cat(entries...)) }

// compression writes a ContentEncoding. algo is 0 zlib, 1 bzlib, 2 lzo and 3 header stripping.
func compression(order, algo uint64, settings []byte, scope uint64) []byte {
	comp := uintElement(0x4254, algo)
	if settings != nil {
		comp = cat(comp, element(0x4255, settings))
	}
	return element(0x6240, cat(uintElement(0x5031, order), uintElement(0x5032, scope), uintElement(0x5033, 0), element(0x5034, comp)))
}

func blockHead(track uint64, rel int16, flags byte) []byte {
	return cat(sizeBytes(track), binary.BigEndian.AppendUint16(nil, uint16(rel)), []byte{flags})
}

func simpleBlock(track uint64, rel int16, data string, lacing byte) []byte {
	return element(0xA3, cat(blockHead(track, rel, 0x80|lacing), []byte(data)))
}

// blockGroup writes a BlockGroup. A negative duration leaves the BlockDuration out.
func blockGroup(track uint64, rel int16, data string, duration int64) []byte {
	group := element(0xA1, cat(blockHead(track, rel, 0), []byte(data)))
	if duration >= 0 {
		group = cat(group, uintElement(0x9B, uint64(duration)))
	}
	return element(0xA0, group)
}

func cluster(timestamp uint64, blocks ...[]byte) []byte {
	return element(idCluster, cat(uintElement(0xE7, timestamp), cat(blocks...)))
}

// cues writes a Cues element with one point per block: time, track, cluster position and block position.
func cues(points [][4]uint64) []byte {
	var data []byte
	for _, p := range points {
		data = cat(data, element(0xBB, cat(uintElement(0xB3, p[0]),
			element(0xB7, cat(uintElement(0xF7, p[1]), uintElement(0xF1, p[2]), uintElement(0xF0, p[3]))))))
	}
	return element(idCues, data)
}

func frameCountTags(uid uint64, frames string) []byte {
	return element(idTags, element(0x7373, cat(element(0x63C0, uintElement(0x63C5, uid)),
		element(0x67C8, cat(element(0x45A3, []byte("NUMBER_OF_FRAMES")), element(0x4487, []byte(frames)))))))
}

// seekHead writes each position in 8 bytes, so the size of the SeekHead does not depend on the positions.
func seekHead(positions [][2]uint64) []byte {
	var data []byte
	for _, p := range positions {
		data = cat(data, element(0x4DBB, cat(element(0x53AB, idBytes(uint32(p[0]))), element(0x53AC, binary.BigEndian.AppendUint64(nil, p[1])))))
	}
	return element(0x114D9B74, data)
}

func segmentFile(docType string, segment ...[]byte) []byte {
	return cat(ebmlHeader(docType), element(idSegment, cat(segment...)))
}

func openFile(tb testing.TB, b []byte) *mkv.File {
	tb.Helper()
	f, err := mkv.Open(bytes.NewReader(b), int64(len(b)))
	require.NoError(tb, err)
	return f
}

func openTestdata(tb testing.TB, name string) *mkv.File {
	tb.Helper()
	return openFile(tb, testfiles.Read(tb, "mkv/"+name))
}

// repeatReader is an io.ReaderAt that reads n bytes of value v.
type repeatReader struct {
	n int64
	v byte
}

func (r repeatReader) ReadAt(p []byte, off int64) (int, error) {
	if off >= r.n {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), r.n-off))
	for i := range n {
		p[i] = r.v
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// zlibZeros compresses n zero bytes in steps of 1 MB, so the test never holds the inflated data.
func zlibZeros(tb testing.TB, n int) []byte {
	tb.Helper()
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	zero := make([]byte, 1<<20)
	for left := n; left > 0; left -= len(zero) {
		_, err := w.Write(zero[:min(left, len(zero))])
		require.NoError(tb, err)
	}
	require.NoError(tb, w.Close())
	return buf.Bytes()
}
