package cea708

import (
	"errors"
	"strconv"
)

// The layout of a caption distribution packet (CDP) of SMPTE 334-2.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs and src/libse/Cea708/Smpte291M.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
const (
	cdpIdentifier      = 0x9669
	cdpHeaderSize      = 7
	timeCodeSection    = 0x71
	timeCodeSectionLen = 5
	ccDataSection      = 0x72
	footerSection      = 0x74
	// cdpFlags marks a CDP with cc_data and an active caption service. The low bit is reserved and 1.
	cdpFlags = 0x43
	// ccCountMarker and tripletMarker are the reserved 1 bits of the count byte and of a triplet flag byte.
	ccCountMarker = 0xE0
	tripletMarker = 0xF8
	ccValid       = 0x04
)

// ErrNoCDP means the data has no CDP identifier or is shorter than the 7 byte header.
var ErrNoCDP = errors.New("The data holds no caption distribution packet.") //nolint:revive,staticcheck // the sentence is the message of the MCC parse error

// ReadCDP returns the frame rate code and the valid triplets of field 1 and of DTVCC data of a CDP.
// It reads the first cc_data section after the optional time code section. A CDP without cc_data has no triplets.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs and src/libse/Cea708/Smpte291M.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
func ReadCDP(cdp []byte) (rateCode int, triplets []Triplet, err error) {
	if len(cdp) < cdpHeaderSize || int(cdp[0])<<8|int(cdp[1]) != cdpIdentifier {
		return 0, nil, ErrNoCDP
	}
	rateCode = int(cdp[3] >> 4)
	offset := cdpHeaderSize
	if offset < len(cdp) && cdp[offset] == timeCodeSection {
		offset += timeCodeSectionLen
	}
	if offset+1 >= len(cdp) || cdp[offset] != ccDataSection {
		return rateCode, nil, nil
	}
	triplets, err = readTriplets(cdp[offset+1:])
	return rateCode, triplets, err
}

// readTriplets reads the count byte and the triplets after it.
func readTriplets(section []byte) ([]Triplet, error) {
	count := int(section[0] & 0x1F)
	if 1+3*count > len(section) {
		return nil, errors.New("The caption distribution packet is shorter than its " + strconv.Itoa(count) + " cc_data triplets.")
	}
	var triplets []Triplet
	for at := 1; count > 0; count, at = count-1, at+3 {
		flags := section[at]
		if flags&ccValid != 0 && flags&0x03 != Field2 {
			triplets = append(triplets, Triplet{Type: flags & 0x03, Data: [2]byte{section[at+1], section[at+2]}})
		}
	}
	return triplets, nil
}

// CDPTriplet is one triplet to write, valid or not.
type CDPTriplet struct {
	Type  byte
	Valid bool
	Data  uint16
}

// WriteCDP returns a CDP with a cc_data section, a footer and its checksum.
// The bytes of the CDP add up to 0 modulo 256.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/MacCaption10.cs and src/libse/Cea708/Smpte291M.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/SubtitleFormats/MacCaption10.cs), MIT.
func WriteCDP(rateCode int, sequence uint16, triplets []CDPTriplet) []byte {
	length := cdpHeaderSize + 2 + 3*len(triplets) + 3 + 1
	cdp := make([]byte, 0, length)
	cdp = append(cdp, cdpIdentifier>>8, cdpIdentifier&0xFF, byte(length), byte(rateCode)<<4|0x0F, cdpFlags, byte(sequence>>8), byte(sequence))
	cdp = append(cdp, ccDataSection, ccCountMarker|byte(len(triplets)))
	for _, t := range triplets {
		flags := tripletMarker | t.Type
		if t.Valid {
			flags |= ccValid
		}
		cdp = append(cdp, flags, byte(t.Data>>8), byte(t.Data))
	}
	cdp = append(cdp, footerSection, byte(sequence>>8), byte(sequence))
	var sum byte
	for _, b := range cdp {
		sum += b
	}
	return append(cdp, -sum)
}
