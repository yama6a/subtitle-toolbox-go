package bitmap_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
)

// pngSpec describes a PNG file that the tests write chunk by chunk, so every colour type and bit depth is covered.
type pngSpec struct {
	width, height    int
	depth, colorType byte
	palette          []byte // PLTE, 3 bytes per entry
	trns             []byte
	rows             [][]byte // scanlines without the filter byte
	idatChunks       int      // 0 or 1 writes one IDAT chunk
}

func chunk(kind string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, kind...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(append([]byte(kind), data...)))
}

func (s pngSpec) bytes() []byte {
	ihdr := binary.BigEndian.AppendUint32(nil, uint32(s.width))
	ihdr = binary.BigEndian.AppendUint32(ihdr, uint32(s.height))
	ihdr = append(ihdr, s.depth, s.colorType, 0, 0, 0)

	var raw bytes.Buffer
	for _, row := range s.rows {
		raw.WriteByte(0)
		raw.Write(row)
	}
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(raw.Bytes())
	_ = w.Close()

	out := []byte("\x89PNG\r\n\x1a\n")
	out = append(out, chunk("IHDR", ihdr)...)
	if s.palette != nil {
		out = append(out, chunk("PLTE", s.palette)...)
	}
	if s.trns != nil {
		out = append(out, chunk("tRNS", s.trns)...)
	}
	data, n := z.Bytes(), max(s.idatChunks, 1)
	size := (len(data) + n - 1) / n
	for i := range n {
		out = append(out, chunk("IDAT", data[min(i*size, len(data)):min((i+1)*size, len(data))])...)
	}
	return append(out, chunk("IEND", nil)...)
}
