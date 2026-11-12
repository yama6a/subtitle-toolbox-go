package mpegts_test

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"strconv"
	"strings"
)

const (
	videoPID    = 0x0100
	teletextPID = 0x0200
	pmtPID      = 0x1000

	subtitlePageType        = 0x02
	hearingImpairedPageType = 0x05

	startBox = "\x0B\x0B"
	endBox   = "\x0A\x0A"
)

// tsWriter writes MPEG transport stream packets with teletext PES packets. Packets and tables follow
// ISO/IEC 13818-1, PES packets ETSI EN 300 472 and teletext packets ETS 300 706.
type tsWriter struct {
	counters map[int]int
	buf      bytes.Buffer
}

func newWriter() *tsWriter { return &tsWriter{counters: map[int]int{}} }

func (w *tsWriter) bytes() []byte { return w.buf.Bytes() }

type pmtStream struct {
	pid         int
	streamType  byte
	descriptors []byte
}

// tables adds the PAT of one program and its PMT.
func (w *tsWriter) tables(programNumber, pmt, pcrPID int, streams ...pmtStream) {
	w.section(0, 0x00, 1, binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, uint16(programNumber)), uint16(0xE000|pmt)))
	body := binary.BigEndian.AppendUint16(binary.BigEndian.AppendUint16(nil, uint16(0xE000|pcrPID)), 0xF000)
	for _, s := range streams {
		body = append(body, s.streamType)
		body = binary.BigEndian.AppendUint16(body, uint16(0xE000|s.pid))
		body = binary.BigEndian.AppendUint16(body, uint16(0xF000|len(s.descriptors)))
		body = append(body, s.descriptors...)
	}
	w.section(pmt, 0x02, programNumber, body)
}

type ttPage struct {
	language string
	kind     int
	number   int
}

// teletextDescriptor returns a teletext descriptor with one entry for each page.
func teletextDescriptor(pages ...ttPage) []byte {
	var entries []byte
	for _, p := range pages {
		entries = append(entries, p.language...)
		entries = append(entries, byte(p.kind<<3|p.number/100%8), byte(p.number%100/10<<4|p.number%10))
	}
	return append([]byte{0x56, byte(len(entries))}, entries...)
}

func teletextStream(pid int, pages ...ttPage) pmtStream {
	return pmtStream{pid: pid, streamType: 0x06, descriptors: teletextDescriptor(pages...)}
}

// teletextPES adds a teletext PES packet with the teletext packets and stuffing data units, so that it fills
// whole packets. Each teletext packet has 42 bytes in the bit order of ETS 300 706.
func (w *tsWriter) teletextPES(pid int, pts int64, packets ...[]byte) {
	data := teletextData(packets)
	for (45+len(data))%184 != 0 {
		data = append(data, 0xFF, 0x2C)
		data = append(data, bytes.Repeat([]byte{0xFF}, 44)...)
	}
	w.packets(pid, pes(0xBD, pts, data, 36))
}

// shortTeletextPES adds a teletext PES packet without stuffing data units. Its last packet has an adaptation field.
func (w *tsWriter) shortTeletextPES(pid int, pts int64, packets ...[]byte) {
	w.packets(pid, pes(0xBD, pts, teletextData(packets), 36))
}

func teletextData(packets [][]byte) []byte {
	data := []byte{0x10}
	for i, p := range packets {
		data = append(data, 0x03, 0x2C, byte(0xE0|(7+i)), 0xE4)
		for _, b := range p {
			data = append(data, bits.Reverse8(b))
		}
	}
	return data
}

// m2ts returns the packets with a 4-byte time stamp in front of each.
func m2ts(ts []byte) []byte {
	var out []byte
	for i := 0; i*188 < len(ts); i++ {
		out = binary.BigEndian.AppendUint32(out, uint32(i*2700)&0x3FFFFFFF)
		out = append(out, ts[i*188:(i+1)*188]...)
	}
	return out
}

func hamming84(value int) byte {
	d1, d2, d3, d4 := value&1, value>>1&1, value>>2&1, value>>3&1
	p1, p2, p3 := 1^d1^d3^d4, 1^d1^d2^d4, 1^d1^d2^d3
	p4 := 1 ^ p1 ^ d1 ^ p2 ^ d2 ^ p3 ^ d3 ^ d4
	return byte(p1 | d1<<1 | p2<<2 | d2<<3 | p3<<4 | d3<<5 | p4<<6 | d4<<7)
}

func hamming2418(value int) []byte {
	b := value&1<<2 | value>>1&0x07<<4 | value>>4&0x7F<<8 | value>>11&0x7F<<16
	for _, check := range []int{1, 2, 4, 8, 16} {
		ones := 0
		for pos := 1; pos <= 23; pos++ {
			if pos&check != 0 {
				ones += b >> (pos - 1) & 1
			}
		}
		if ones%2 == 0 {
			b |= 1 << (check - 1)
		}
	}
	if bits.OnesCount(uint(b))%2 == 0 {
		b |= 1 << 23
	}
	return []byte{byte(b), byte(b >> 8), byte(b >> 16)}
}

func address(magazine, row int) []byte {
	return []byte{hamming84(magazine | row&1<<3), hamming84(row >> 1)}
}

// parity returns the text with the odd parity bit of each byte.
func parity(text string) []byte {
	out := []byte(text)
	for i, b := range out {
		if bits.OnesCount8(b)%2 == 0 {
			out[i] |= 0x80
		}
	}
	return out
}

func pad(text string, n int) string { return text + strings.Repeat(" ", max(0, n-len(text))) }

// header returns the page header packet X/0 of a page such as 888. national holds C12 to C14 with C12 as bit 0.
func header(page int, erase bool, national int, subtitle, serial bool) []byte {
	b := func(v bool, n int) int {
		if v {
			return n
		}
		return 0
	}
	values := []int{page % 10, page % 100 / 10, 0, b(erase, 8), 0, b(subtitle, 8), b(subtitle, 1), b(serial, 1) | national<<1}
	out := address(page/100%8, 0)
	for _, v := range values {
		out = append(out, hamming84(v))
	}
	return append(out, parity(pad(strconv.Itoa(page)+" TEXT", 32))...)
}

// pageHeader returns the header of a subtitle page with the erase flag and the English national option.
func pageHeader(page int) []byte { return header(page, true, 0, true, false) }

// fillingHeader returns a time filling header of a magazine. It ends the page transmission of the magazine.
func fillingHeader(magazine int) []byte {
	out := address(magazine%8, 0)
	for _, v := range []int{0xF, 0xF, 0, 0, 0, 0, 0, 0} {
		out = append(out, hamming84(v))
	}
	return append(out, parity(strings.Repeat(" ", 32))...)
}

// row returns a display row packet X/1 to X/23 with attribute bytes and G0 codes.
func row(page, row int, codes string) []byte {
	return append(address(page/100%8, row), parity(pad(codes, 40))...)
}

// designation returns packet X/28/0 format 1 or M/29/0 with the G0 set of group and national.
// The triplet bits 10, 9 and 8 hold C12, C13 and C14, the reverse order of national.
func designation(page, rowNumber, group, national int) []byte {
	options := national&1<<2 | national&2 | national>>2&1
	out := append(address(page/100%8, rowNumber), hamming84(0))
	out = append(out, hamming2418((group<<3|options)<<7)...)
	for range 12 {
		out = append(out, hamming2418(0)...)
	}
	return out
}

func (w *tsWriter) section(pid, tableID, extension int, body []byte) {
	section := []byte{byte(tableID)}
	section = binary.BigEndian.AppendUint16(section, uint16(0xB000|(len(body)+9)))
	section = binary.BigEndian.AppendUint16(section, uint16(extension))
	section = append(section, 0xC1, 0x00, 0x00)
	section = append(section, body...)
	section = binary.BigEndian.AppendUint32(section, crc32MPEG2(section))
	w.packets(pid, append([]byte{0x00}, section...))
}

func pes(streamID byte, pts int64, data []byte, headerLength int) []byte {
	h := []byte{0x84, 0x80, byte(headerLength), byte(0x21 | pts>>29&0x0E)}
	h = binary.BigEndian.AppendUint16(h, uint16(pts>>14&0xFFFE|1))
	h = binary.BigEndian.AppendUint16(h, uint16(pts<<1&0xFFFE|1))
	h = append(h, bytes.Repeat([]byte{0xFF}, headerLength-5)...)
	out := []byte{0x00, 0x00, 0x01, streamID}
	out = binary.BigEndian.AppendUint16(out, uint16(len(h)+len(data)))
	return append(append(out, h...), data...)
}

// packets splits the payload into packets. The adaptation field of the last packet stuffs it to 188 bytes.
func (w *tsWriter) packets(pid int, payload []byte) {
	for first := true; len(payload) > 0 || first; first = false {
		var adaptation []byte
		has := false
		room := 184
		if len(payload) < room {
			has = true
			if missing := room - len(payload); missing > 1 {
				adaptation = append([]byte{0x00}, bytes.Repeat([]byte{0xFF}, missing-2)...)
			}
			room = len(payload)
		}
		counter := w.counters[pid]
		w.counters[pid] = (counter + 1) % 16
		flags := 0
		if first {
			flags = 0x4000
		}
		control := 0x10
		if has {
			control = 0x30
		}
		w.buf.WriteByte(0x47)
		w.buf.Write(binary.BigEndian.AppendUint16(nil, uint16(flags|pid)))
		w.buf.WriteByte(byte(control | counter))
		if has {
			w.buf.WriteByte(byte(len(adaptation)))
			w.buf.Write(adaptation)
		}
		w.buf.Write(payload[:room])
		payload = payload[room:]
	}
}

func crc32MPEG2(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
