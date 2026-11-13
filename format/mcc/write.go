package mcc

import (
	"crypto/sha1" //nolint:gosec // the hash names a file, it protects nothing
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var fieldNameRegexp = regexp.MustCompile(`^[A-Za-z][A-Za-z ]*$`)

const (
	defaultRate    = "29.97"
	defaultVersion = "2.0"
	program        = "Subtitle Toolbox"
	nullPair       = 0x8080
	service1       = 0x20
)

// Write writes s as MCC with CEA-608 field 1 and CEA-708 service 1 data. A cue that CEA-608 cannot hold fails with
// an *model.UnwritableError, unless the fit changes it. The report lists the fit changes of the CEA-608 data.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	s, err := writing.SkipImageCues(s, model.MCC, common)
	if err != nil {
		return WriteReport{}, err
	}
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	file, _ := s.FormatData(formatDataKey)
	row := outputRate(file, opts)
	dropFrame := dropFrameOf(file, opts, row)
	version, _ := file.String("version")
	if version == "" || dropFrame && row.labels == 60 {
		version = defaultVersion
	}

	words, changes, err := caption.CEA608Words(model.MCC, s.Cues(), true)
	if err != nil {
		return WriteReport{}, err
	}
	lines := dataLines(s, words, row, dropFrame)
	text := append([]byte(header(version, label(row, dropFrame), file, lines)), lines...)
	if err := writing.Write(w, model.MCC, text, common, false); err != nil {
		return WriteReport{}, err
	}
	return WriteReport{Changes: changes}, nil
}

// dataLines returns one line for each frame with CEA-608 words or a CEA-708 block, in frame order.
func dataLines(s *model.Subtitle, words []caption.FrameWord, row rate, dropFrame bool) []byte {
	byFrame := map[int64][]uint16{}
	for _, fw := range words {
		n := roundDiv(fw.Frame*1001*row.fps.Num, 30000*row.fps.Den)
		byFrame[n] = append(byFrame[n], fw.Word)
	}
	// A packet holds a header byte and a block header byte. A cc_data triplet holds 2 bytes.
	capacity := 2*(row.triplets-2) - 2
	blocks := map[int64][]byte{}
	for _, b := range cea708.Blocks(caption.CEA708Captions(s.Cues()), row.fps.Frames, capacity) {
		blocks[b.Frame] = b.Data
	}
	frames := slices.Concat(slices.Collect(maps.Keys(byFrame)), slices.Collect(maps.Keys(blocks)))
	slices.Sort(frames)
	frames = slices.Compact(frames)

	fps := row.fps
	fps.DropFrame = dropFrame
	lines := make([]byte, 0, len(frames)*64)
	sequence := uint16(0)
	for _, frame := range frames {
		if fps.Duration(frame) >= timecode.MaxTime {
			break
		}
		triplets := frameTriplets(byFrame[frame], blocks[frame], sequence, row.triplets)
		tc := fps.TimecodeOf(frame)
		lines = appendPadded(lines, tc.Hours)
		for _, field := range []int{tc.Minutes, tc.Seconds, tc.Frames} {
			lines = appendPadded(append(lines, ':'), field)
		}
		lines = append(lines, '\t')
		lines = appendHex(lines, ancPacket(cea708.WriteCDP(row.code, sequence, triplets)))
		lines = append(lines, '\n')
		sequence++
	}
	return lines
}

// appendPadded appends n with at least 2 digits.
func appendPadded(dst []byte, n int) []byte {
	if n < 10 {
		dst = append(dst, '0')
	}
	return strconv.AppendInt(dst, int64(n), 10)
}

// frameTriplets returns the triplets of one line: the field 1 words, a field 2 null pair, the DTVCC packet and
// empty triplets up to the count of the rate.
func frameTriplets(words []uint16, block []byte, sequence uint16, count int) []cea708.CDPTriplet {
	triplets := make([]cea708.CDPTriplet, 0, count)
	for _, word := range orNull(words) {
		triplets = append(triplets, cea708.CDPTriplet{Type: cea708.Field1, Valid: true, Data: word})
	}
	triplets = append(triplets, cea708.CDPTriplet{Type: cea708.Field2, Valid: true, Data: nullPair})
	if block != nil {
		triplets = append(triplets, dtvccTriplets(block, sequence)...)
	}
	for len(triplets) < count {
		triplets = append(triplets, cea708.CDPTriplet{Type: cea708.DTVCCData})
	}
	return triplets
}

func orNull(words []uint16) []uint16 {
	if len(words) == 0 {
		return []uint16{nullPair}
	}
	return words
}

func roundDiv(a, b int64) int64 {
	return (2*a + b) / (2 * b)
}

func outputRate(file *model.Data, opts WriteOptions) rate {
	if !opts.FrameRate.IsZero() {
		row, _ := rateNear(opts.FrameRate)
		return row
	}
	name, _ := file.String("frameRate")
	if row, ok := rateNamed(name); ok {
		return row
	}
	row, _ := rateNamed(defaultRate)
	return row
}

// dropFrameOf applies the drop-frame mode. Only 29.97 and 59.94 have drop-frame labels.
func dropFrameOf(file *model.Data, opts WriteOptions, row rate) bool {
	if row.fps.Den == 1 || row.labels == 24 {
		return false
	}
	switch opts.DropFrame {
	case model.DropFrameOn:
		return true
	case model.DropFrameOff:
		return false
	case model.DropFrameKeep:
	}
	if opts.FrameRate.DropFrame {
		return true
	}
	stored, ok := file.Bool("dropFrame")
	return stored || !ok
}

func label(row rate, dropFrame bool) string {
	if dropFrame {
		return strconv.Itoa(row.labels) + "DF"
	}
	return strconv.Itoa(row.labels)
}

// dtvccTriplets returns the triplets of a DTVCC packet with one service 1 block. The packet header holds the
// sequence number 0 to 3 and the packet size in pairs of bytes.
func dtvccTriplets(block []byte, sequence uint16) []cea708.CDPTriplet {
	packet := append([]byte{service1 | byte(len(block))}, block...)
	if len(packet)%2 == 0 {
		packet = append(packet, 0)
	}
	packet = append([]byte{byte(sequence&3)<<6 | byte((len(packet)+1)/2)}, packet...)
	triplets := make([]cea708.CDPTriplet, 0, len(packet)/2)
	for i := 0; i < len(packet); i += 2 {
		t := cea708.CDPTriplet{Type: cea708.DTVCCData, Valid: true, Data: uint16(packet[i])<<8 | uint16(packet[i+1])}
		if i == 0 {
			t.Type = cea708.DTVCCStart
		}
		triplets = append(triplets, t)
	}
	return triplets
}

// ancPacket wraps a CDP in an ANC packet with DID 61h and SDID 01h and its checksum.
func ancPacket(cdp []byte) []byte {
	anc := append([]byte{0x61, 0x01, byte(len(cdp))}, cdp...)
	var sum byte
	for _, b := range anc {
		sum += b
	}
	return append(anc, sum)
}

// header returns the notice, the header fields and the empty line before the data lines. A new UUID comes from the
// hash of the data lines, so the same captions give the same file on the same day.
func header(version, timeCodeRateValue string, file *model.Data, lines []byte) string {
	rates := "24, 25, 30, 30DF, 50, 60"
	if version != "1.0" {
		rates += ", 60DF"
	}
	hash := sha1.Sum(lines) //nolint:gosec // see the import
	fields := model.NewData(
		model.Field{Key: "UUID", Value: uuid(hash)},
		model.Field{Key: "Creation Program", Value: program},
		model.Field{Key: "Creation Date", Value: time.Now().UTC().Format("Monday, January 2, 2006")},
	)
	stored, _ := file.Object("header")
	for _, k := range stored.Keys() {
		if v, ok := stored.String(k); ok && fieldNameRegexp.MatchString(k) && !strings.HasPrefix(k+"="+v, versionPrefix) {
			fields.Set(k, v)
		}
	}
	fields.Set(timeCodeRate, timeCodeRateValue)

	var b strings.Builder
	fmt.Fprintf(&b, headerText, version, rates)
	b.WriteString("\n\n")
	for _, k := range fields.Keys() {
		v, _ := fields.String(k)
		b.WriteString(k + "=" + oneLine(v) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// oneLine turns the line breaks of a header text into spaces, because a field is one line.
func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

// uuid returns a name-based UUID of version 5 from a SHA-1 hash, as RFC 9562 defines it.
func uuid(hash [sha1.Size]byte) string {
	b := hash[:16]
	b[6] = b[6]&0x0F | 0x50
	b[8] = b[8]&0x3F | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}
