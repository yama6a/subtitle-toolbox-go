// Package mpegts reads the subtitle tracks of MPEG transport streams and M2TS files.
//
// Packets, PAT, PMT and PES: ISO/IEC 13818-1. Descriptors: ETSI EN 300 468. Teletext in PES packets: ETSI EN 300 472.
package mpegts

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/asticode/go-astits"

	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	codecTeletext = "dvb_teletext"
	// tableScanBytes is the file offset at which the table scan ends. A PAT or a PMT further in is not part of a
	// recording that starts at a packet.
	tableScanBytes = 32 << 20
	headSize       = 2*192 + 5

	subtitlePage        = 0x02
	hearingImpairedPage = 0x05
	maxPage             = 99
	readBufferSize      = 256 << 10
)

// stream is the place of a track in the file.
type stream struct {
	pid         uint16
	page        int // magazine (0 for 8) << 8 | page byte
	programPIDs []uint16
}

// File is an open MPEG-TS or M2TS file.
type File struct {
	r          io.ReaderAt
	size       int64
	packetSize int
	tracks     []model.Track
	streams    []stream
}

// Open reads the PAT and the PMTs within the first 32 MiB of r and lists the tracks.
func Open(r io.ReaderAt, size int64) (*File, error) {
	head := make([]byte, max(0, min(size, headSize)))
	if n, err := r.ReadAt(head, 0); err != nil && n < len(head) {
		return nil, fmt.Errorf("read mpegts: %w", err)
	}
	f := &File{r: r, size: size, packetSize: packetSize(head)}
	if f.packetSize == 0 {
		return nil, notTS()
	}
	if err := f.scanTables(); err != nil {
		return nil, err
	}
	return f, nil
}

func notTS() error {
	return &model.ParseError{Msg: "The file is not an MPEG-TS file."}
}

// packetSize returns 188 when the sync byte is at the offsets 0, 188 and 376 of head, 192 when it is at 4, 196
// and 388, else 0. The head must be longer than 2 packets and the time stamp.
func packetSize(head []byte) int {
	for _, size := range []int{tsPacketSize, 192} {
		sync := size - tsPacketSize
		if len(head) > 2*size+sync && head[sync] == syncByte && head[size+sync] == syncByte && head[2*size+sync] == syncByte {
			return size
		}
	}
	return 0
}

// Tracks returns the subtitle tracks in the order of the PMTs.
func (f *File) Tracks() []model.Track {
	return slices.Clone(f.tracks)
}

// pass runs one read of the file. data gets each piece of data that go-astits returns and reports whether the
// pass goes on. Damage that go-astits reports is skipped, because each error consumes at least one packet.
func (f *File) pass(a *aligner, data func(*astits.DemuxerData) bool) error {
	a.r = bufio.NewReaderSize(io.NewSectionReader(f.r, 0, f.size), readBufferSize)
	dmx := astits.NewDemuxer(context.Background(), a, astits.DemuxerOptPacketSize(tsPacketSize))
	for {
		d, err := dmx.NextData()
		if a.err != nil {
			return fmt.Errorf("read mpegts: %w", a.err)
		}
		if errors.Is(err, astits.ErrNoMorePackets) {
			return nil
		}
		if err == nil && d != nil && !data(d) {
			return nil
		}
	}
}

// tableScan collects the first PAT and the first PMT of each program.
type tableScan struct {
	havePAT  bool
	programs []uint16
	pmts     map[uint16][]*astits.PMTElementaryStream
}

// add reads one piece of data and reports whether the scan needs more.
func (t *tableScan) add(d *astits.DemuxerData, wanted map[uint16]bool) bool {
	switch {
	case d.PAT != nil && d.PID == 0 && !t.havePAT:
		t.addPAT(d.PAT, wanted)
	case d.PMT != nil && slices.Contains(t.programs, d.PID):
		if _, ok := t.pmts[d.PID]; !ok {
			t.pmts[d.PID] = d.PMT.ElementaryStreams
		}
	}
	return !t.havePAT || len(t.pmts) < len(t.programs)
}

func (t *tableScan) addPAT(pat *astits.PATData, wanted map[uint16]bool) {
	t.havePAT = true
	for _, p := range pat.Programs {
		if p.ProgramNumber != 0 && !slices.Contains(t.programs, p.ProgramMapID) {
			t.programs = append(t.programs, p.ProgramMapID)
			wanted[p.ProgramMapID] = true
		}
	}
}

// scanTables reads the PAT and the PMTs within the first 32 MiB and lists the teletext pages of each program.
func (f *File) scanTables() error {
	a := newAligner(f.packetSize, map[uint16]bool{0: true}, tableScanBytes)
	scan := &tableScan{pmts: map[uint16][]*astits.PMTElementaryStream{}}
	err := f.pass(a, func(d *astits.DemuxerData) bool { return scan.add(d, a.wanted) })
	if err != nil {
		return err
	}
	for _, pid := range scan.programs {
		f.addTeletext(scan.pmts[pid])
	}
	return nil
}

// addTeletext lists the teletext subtitle pages of the elementary streams of one program.
func (f *File) addTeletext(streams []*astits.PMTElementaryStream) {
	pids := make([]uint16, len(streams))
	for i, es := range streams {
		pids[i] = es.ElementaryPID
	}
	for _, es := range streams {
		if es.StreamType != astits.StreamTypePrivateData {
			continue
		}
		for _, desc := range es.ElementaryStreamDescriptors {
			switch {
			case desc.Teletext != nil:
				f.addPages(es.ElementaryPID, pids, desc.Length, desc.Teletext)
			case desc.VBITeletext != nil:
				f.addPages(es.ElementaryPID, pids, desc.Length, desc.VBITeletext)
			}
		}
	}
}

// addPages adds the pages of a teletext descriptor. go-astits reads a short last entry from the bytes after it,
// so only the first length/5 items count. It turns the page byte into a decimal number, so a page byte with a low
// nibble above 9 reads as another page.
func (f *File) addPages(pid uint16, programPIDs []uint16, length uint8, d *astits.DescriptorTeletext) {
	for _, item := range d.Items[:min(len(d.Items), int(length)/5)] {
		if (item.Type != subtitlePage && item.Type != hearingImpairedPage) || item.Page > maxPage {
			continue
		}
		magazine := int(item.Magazine)
		number := int(item.Page) + 100*cmp.Or(magazine, 8)
		if f.hasNumber(number) {
			number += int(pid) * 1000
			if f.hasNumber(number) {
				continue
			}
		}
		track := model.Track{
			Container: model.MPEGTS,
			Number:    number,
			CodecID:   codecTeletext,
			Format:    model.SubRip,
			Language:  language(item.Language),
		}
		if item.Type == hearingImpairedPage {
			track.Name = "hearing impaired"
		}
		f.tracks = append(f.tracks, track)
		f.streams = append(f.streams, stream{
			pid:         pid,
			page:        magazine<<8 | int(item.Page/10)<<4 | int(item.Page%10),
			programPIDs: programPIDs,
		})
	}
}

func (f *File) hasNumber(n int) bool {
	return slices.ContainsFunc(f.tracks, func(t model.Track) bool { return t.Number == n })
}

// language returns the 3 letters of a descriptor lower-cased, or "und".
func language(b []byte) string {
	s := strings.ToLower(string(b))
	if len(s) != 3 || strings.Trim(s, "abcdefghijklmnopqrstuvwxyz") != "" {
		return "und"
	}
	return s
}
