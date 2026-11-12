package mp4_test

import (
	"encoding/binary"
	"slices"
)

// This file ports the box writer of Mp4FixtureWriter.php in subtitle-toolbox-php to Go.

type sampleSpec struct {
	duration uint32
	data     []byte
}

type chunkSpec struct {
	offset  uint64
	samples [][2]uint32 // duration and size
}

// trackSpec describes a trak box. The boxes field replaces the stsd, stts, stsc, stsz or stco box.
type trackSpec struct {
	id        uint32
	handler   string
	entry     []byte
	timescale uint32
	zeroTime  bool // writes a timescale of 0
	language  string
	elng      string
	name      string
	disabled  bool
	version1  bool
	co64      bool
	elstV1    bool
	stz2      bool
	boxes     map[string][]byte
	edits     [][2]int64 // segment duration and media time, a media time of -1 is an empty edit
	perChunk  int
	samples   []sampleSpec
	chunks    []chunkSpec
}

func u16(v uint16) []byte        { return binary.BigEndian.AppendUint16(nil, v) }
func u32(v uint32) []byte        { return binary.BigEndian.AppendUint32(nil, v) }
func u64(v uint64) []byte        { return binary.BigEndian.AppendUint64(nil, v) }
func cat(parts ...[]byte) []byte { return slices.Concat(parts...) }
func zeros(n int) []byte         { return make([]byte, n) }

func box(typ string, data ...[]byte) []byte {
	payload := cat(data...)
	return cat(u32(uint32(8+len(payload))), []byte(typ), payload)
}

func fullBox(typ string, version byte, flags uint32, data ...[]byte) []byte {
	return box(typ, u32(uint32(version)<<24|flags), cat(data...))
}

func ftyp() []byte {
	return box("ftyp", []byte("isom"), u32(512), []byte("isomiso2mp41"))
}

// textSample is a tx3g sample: the 16-bit text length, the text and the modifier boxes.
func textSample(text string, modifiers ...[]byte) []byte {
	return cat(u16(uint16(len(text))), []byte(text), cat(modifiers...))
}

// tx3gEntry is a tx3g sample entry. A sinf box in extra makes it the entry of an encrypted track.
func tx3gEntry(extra ...[]byte) []byte {
	style := cat(u16(0), u16(0), u16(1), []byte{0, 18}, []byte{0xFF, 0xFF, 0xFF, 0xFF})
	ftab := box("ftab", u16(1), u16(1), []byte{10}, []byte("Sans-Serif"))
	return box("tx3g", zeros(6), u16(1), u32(0), []byte{0x01, 0xFF}, zeros(4), zeros(8), style, ftab, cat(extra...))
}

func videoTrack(id uint32, perChunk, samples, size int) trackSpec {
	data := make([]sampleSpec, samples)
	for i := range data {
		data[i] = sampleSpec{500, slices.Repeat([]byte{0x80}, size)}
	}
	return trackSpec{id: id, handler: "vide", entry: box("mp4v", zeros(78)), perChunk: perChunk, samples: data}
}

// file lays out the samples of all tracks in one mdat box and writes the moov box before or after it.
func file(tracks []trackSpec, moovFirst bool) []byte {
	var mdat []byte
	laid := make([]trackSpec, len(tracks))
	for i, t := range tracks {
		for from := 0; from < len(t.samples); from += t.perChunk {
			c := chunkSpec{offset: uint64(len(mdat))}
			for _, s := range t.samples[from:min(from+t.perChunk, len(t.samples))] {
				c.samples = append(c.samples, [2]uint32{s.duration, uint32(len(s.data))})
				mdat = append(mdat, s.data...)
			}
			t.chunks = append(t.chunks, c)
		}
		laid[i] = t
	}
	moov := func(base uint64) []byte {
		shifted := make([]trackSpec, len(laid))
		for i, t := range laid {
			t.chunks = slices.Clone(t.chunks)
			for j := range t.chunks {
				t.chunks[j].offset += base
			}
			shifted[i] = t
		}
		return moovBox(shifted)
	}
	head := ftyp()
	if moovFirst {
		return cat(head, moov(uint64(len(head)+len(moov(0))+8)), box("mdat", mdat))
	}
	return cat(head, box("mdat", mdat), moov(uint64(len(head)+8)))
}

func moovBox(tracks []trackSpec) []byte {
	mvhd := fullBox("mvhd", 0, 0, u32(0), u32(0), u32(1000), u32(0), zeros(80))
	parts := make([][]byte, 0, 1+len(tracks))
	parts = append(parts, mvhd)
	for _, t := range tracks {
		parts = append(parts, trak(t))
	}
	return box("moov", parts...)
}

func trak(t trackSpec) []byte {
	flags := uint32(0x000003)
	if t.disabled {
		flags = 0x000002
	}
	var tkhd []byte
	if t.version1 {
		tkhd = fullBox("tkhd", 1, flags, u64(0), u64(0), u32(t.id), u32(0), u64(0), zeros(60))
	} else {
		tkhd = fullBox("tkhd", 0, flags, u32(0), u32(0), u32(t.id), u32(0), u32(0), zeros(60))
	}
	timescale := t.timescale
	if timescale == 0 && !t.zeroTime {
		timescale = 1000
	}
	language := cmpOr(t.language, "und")
	packed := uint16(language[0]-0x60)<<10 | uint16(language[1]-0x60)<<5 | uint16(language[2]-0x60)
	var mdhd []byte
	if t.version1 {
		mdhd = fullBox("mdhd", 1, 0, u64(0), u64(0), u32(timescale), u64(0), u16(packed), u16(0))
	} else {
		mdhd = fullBox("mdhd", 0, 0, u32(0), u32(0), u32(timescale), u32(0), u16(packed), u16(0))
	}
	hdlr := fullBox("hdlr", 0, 0, u32(0), []byte(t.handler), zeros(12), []byte("Handler\x00"))
	var elng []byte
	if t.elng != "" {
		elng = fullBox("elng", 0, 0, []byte(t.elng+"\x00"))
	}
	mdia := box("mdia", mdhd, hdlr, elng, box("minf", box("stbl", sampleTables(t))))
	var udta, edts []byte
	if t.name != "" {
		udta = box("udta", box("name", []byte(t.name)))
	}
	if t.edits != nil {
		var entries []byte
		version := byte(0)
		for _, e := range t.edits {
			if t.elstV1 {
				version = 1
				entries = append(entries, cat(u64(uint64(e[0])), u64(uint64(e[1])), u16(1), u16(0))...)
			} else {
				entries = append(entries, cat(u32(uint32(e[0])), u32(uint32(e[1])), u16(1), u16(0))...)
			}
		}
		edts = box("edts", fullBox("elst", version, 0, u32(uint32(len(t.edits))), entries))
	}
	return box("trak", tkhd, edts, mdia, udta)
}

func cmpOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func sampleTables(t trackSpec) []byte {
	var samples [][2]uint32
	for _, c := range t.chunks {
		samples = append(samples, c.samples...)
	}
	stsd := fullBox("stsd", 0, 0, u32(1), t.entry)

	var stts [][2]uint32
	for _, s := range samples {
		if n := len(stts); n > 0 && stts[n-1][1] == s[0] {
			stts[n-1][0]++
		} else {
			stts = append(stts, [2]uint32{1, s[0]})
		}
	}
	sttsData := u32(uint32(len(stts)))
	for _, e := range stts {
		sttsData = append(sttsData, cat(u32(e[0]), u32(e[1]))...)
	}

	var stsc [][2]uint32
	for i, c := range t.chunks {
		if n := len(stsc); n == 0 || stsc[n-1][1] != uint32(len(c.samples)) {
			stsc = append(stsc, [2]uint32{uint32(i + 1), uint32(len(c.samples))})
		}
	}
	stscData := u32(uint32(len(stsc)))
	for _, e := range stsc {
		stscData = append(stscData, cat(u32(e[0]), u32(e[1]), u32(1))...)
	}

	var stsz []byte
	if t.stz2 {
		sizes := u32(uint32(len(samples)))
		for _, s := range samples {
			sizes = append(sizes, u16(uint16(s[1]))...)
		}
		stsz = fullBox("stz2", 0, 0, []byte{0, 0, 0, 0x10}, sizes)
	} else {
		sizes := cat(u32(0), u32(uint32(len(samples))))
		for _, s := range samples {
			sizes = append(sizes, u32(s[1])...)
		}
		stsz = fullBox("stsz", 0, 0, sizes)
	}

	stco := u32(uint32(len(t.chunks)))
	typ := "stco"
	if t.co64 {
		typ = "co64"
	}
	for _, c := range t.chunks {
		if t.co64 {
			stco = append(stco, u64(c.offset)...)
		} else {
			stco = append(stco, u32(uint32(c.offset))...)
		}
	}
	tables := map[string][]byte{
		"stsd": stsd, "stts": fullBox("stts", 0, 0, sttsData), "stsc": fullBox("stsc", 0, 0, stscData), "stsz": stsz,
		"stco": fullBox(typ, 0, 0, stco),
	}
	out := make([]byte, 0, 1024)
	for _, name := range []string{"stsd", "stts", "stsc", "stsz", "stco"} {
		if replacement, ok := t.boxes[name]; ok {
			out = append(out, replacement...)
		} else {
			out = append(out, tables[name]...)
		}
	}
	return out
}
