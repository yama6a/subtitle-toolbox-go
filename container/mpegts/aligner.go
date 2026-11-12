package mpegts

import (
	"bufio"
	"bytes"
	"io"
)

const (
	tsPacketSize = 188
	syncByte     = 0x47
)

type ptsEvent struct {
	pid uint16
	pts int64
}

// aligner is an io.Reader that hands go-astits clean 188-byte packets. go-astits cannot resync after damage and
// cannot read the time stamp of an M2TS packet, so the aligner does both and filters the packets that the pass
// does not need.
// counter holds the last continuity counter of a PID as the file has it and as go-astits gets it.
type counter struct{ raw, fixed uint8 }

type aligner struct {
	r          *bufio.Reader
	packetSize int
	wanted     map[uint16]bool // PIDs that go-astits reads
	others     map[uint16]bool // PIDs of which only the first PES header gives a PTS event
	limit      int64           // packets that start at or after this file offset end the pass, 0 for none
	pos        int64           // file offset of the next byte of r
	err        error           // the first read error
	events     []ptsEvent
	counters   map[uint16]counter
	seen       map[uint16]bool
	packet     [tsPacketSize]byte
	rest       []byte
}

func newAligner(packetSize int, wanted map[uint16]bool, limit int64) *aligner {
	return &aligner{
		packetSize: packetSize, wanted: wanted, limit: limit,
		counters: map[uint16]counter{}, seen: map[uint16]bool{},
	}
}

func (a *aligner) Read(p []byte) (int, error) {
	if len(a.rest) == 0 {
		if !a.next() {
			return 0, io.EOF
		}
		a.rest = a.packet[:]
	}
	n := copy(p, a.rest)
	a.rest = a.rest[n:]
	return n, nil
}

// next reads the next packet that the pass needs into a.packet. It returns false at the end of the pass.
func (a *aligner) next() bool {
	sync := a.packetSize - tsPacketSize
	for {
		buf, err := a.r.Peek(a.packetSize)
		if err != nil {
			if err != io.EOF {
				a.err = err
			}
			return false
		}
		if buf[sync] != syncByte {
			a.skip(a.syncOffset(buf, sync))
			continue
		}
		if a.limit > 0 && a.pos >= a.limit {
			return false
		}
		copy(a.packet[:], buf[sync:])
		a.skip(a.packetSize)
		if a.accept(&a.packet) {
			return true
		}
	}
}

// syncOffset returns how many bytes to discard so that the next sync byte after the one that is missing leads the packet.
func (a *aligner) syncOffset(buf []byte, sync int) int {
	if i := bytes.IndexByte(buf[sync+1:], syncByte); i >= 0 {
		return i + 1
	}
	return tsPacketSize
}

func (a *aligner) skip(n int) {
	d, _ := a.r.Discard(n)
	a.pos += int64(d)
}

// accept reports whether go-astits gets the packet. It records the PTS events of the other streams and fixes the
// continuity counter.
func (a *aligner) accept(p *[tsPacketSize]byte) bool {
	if p[1]&0x80 != 0 {
		return false
	}
	pid := uint16(p[1]&0x1F)<<8 | uint16(p[2])
	if a.others[pid] {
		if p[1]&0x40 != 0 && !a.seen[pid] {
			a.seen[pid] = true
			if pts, ok := firstPTS(payload(p)); ok {
				a.events = append(a.events, ptsEvent{pid, pts})
			}
		}
		return false
	}
	if !a.wanted[pid] {
		return false
	}
	return p[3]&0x10 == 0 || a.fixCounter(pid, p)
}

// fixCounter reports whether go-astits gets a packet with a payload.
func (a *aligner) fixCounter(pid uint16, p *[tsPacketSize]byte) bool {
	c, seen := a.counters[pid]
	raw := p[3] & 0x0F
	// go-astits checks the counter gap before the repeat, so a repeated packet inside a PES would empty it.
	// The aligner drops the repeat. A jump becomes the expected counter, so go-astits keeps the data across it.
	if seen && raw == c.raw {
		return false
	}
	c.raw = raw
	if seen {
		c.fixed = (c.fixed + 1) & 0x0F
	} else {
		c.fixed = raw
	}
	a.counters[pid] = c
	p[3] = p[3]&0xF0 | c.fixed
	return true
}

// payload returns the payload of a packet, nil when it has none.
func payload(p *[tsPacketSize]byte) []byte {
	control := p[3] >> 4 & 0x03
	at := 4
	if control&0x02 != 0 {
		at += 1 + int(p[4])
	}
	if control&0x01 == 0 || at >= tsPacketSize {
		return nil
	}
	return p[at:]
}

// firstPTS returns the PTS of the PES header at the start of a packet payload.
func firstPTS(b []byte) (int64, bool) {
	if len(b) < 9 || b[0] != 0 || b[1] != 0 || b[2] != 1 || b[7]&0x80 == 0 || len(b) < 14 {
		return 0, false
	}
	return int64(b[9]>>1&0x07)<<30 | int64(b[10])<<22 | int64(b[11]>>1)<<15 | int64(b[12])<<7 | int64(b[13]>>1), true
}
