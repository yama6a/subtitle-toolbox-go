package cea708

// The cc_type values of a cc_data triplet.
const (
	Field1     = 0 // CEA-608 byte pair of field 1
	Field2     = 1 // CEA-608 byte pair of field 2, never passed in
	DTVCCData  = 2
	DTVCCStart = 3
)

// The C0 codes and the extension prefix.
const (
	codeBS   = 0x08
	codeFF   = 0x0C
	codeCR   = 0x0D
	codeHCR  = 0x0E
	codeEXT1 = 0x10
	codeP16  = 0x18
)

// The C1 commands. Window commands take a bitmap of the 8 windows.
const (
	cmdCW0 = 0x80
	cmdCLW = 0x88
	cmdDSW = 0x89
	cmdHDW = 0x8A
	cmdTGW = 0x8B
	cmdDLW = 0x8C
	cmdRST = 0x8F
	cmdSPA = 0x90
	cmdSPC = 0x91
	cmdSPL = 0x92
	cmdSWA = 0x97
	cmdDF0 = 0x98
)

const (
	musicNote = 0x7F
	white     = 0x3F
)

// c1Parameters holds the parameter bytes of each C1 command from 0x80 to 0x9F.
var c1Parameters = [32]int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 0, 0, 2, 3, 2, 0, 0, 0, 0, 4, 6, 6, 6, 6, 6, 6, 6, 6} //nolint:gochecknoglobals // constant table

// g2 holds the characters after EXT1. Codes 0x20 and 0x21 are the transparent spaces.
//
// Adapted from Subtitle Edit, src/libse/Cea708/Cea708.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/58fad713d9c2b322784bc2fa52148e078dbc8759/src/libse/Cea708/Cea708.cs), MIT.
var g2 = map[byte]rune{ //nolint:gochecknoglobals // constant table
	0x20: ' ', 0x21: ' ', 0x25: '\u2026', 0x2A: '\u0160', 0x2C: '\u0152', 0x30: '\u2588', 0x31: '\u2018',
	0x32: '\u2019', 0x33: '\u201c', 0x34: '\u201d', 0x35: '\u2022', 0x39: '\u2122', 0x3A: '\u0161',
	0x3C: '\u0153', 0x3D: '\u2120', 0x3F: '\u0178', 0x76: '\u215b', 0x77: '\u215c', 0x78: '\u215d',
	0x79: '\u215e', 0x7A: '\u2502', 0x7B: '\u2510', 0x7C: '\u2514', 0x7D: '\u2500',
	0x7E: '\u2518', 0x7F: '\u250c',
}

// channelHex holds the hex value of a 2-bit color channel.
var channelHex = [4]string{"00", "55", "aa", "ff"} //nolint:gochecknoglobals // constant table

// g2Codes maps a G2 character to its code, without the transparent spaces.
var g2Codes = func() map[rune]byte { //nolint:gochecknoglobals // lookup built once from g2
	m := make(map[rune]byte, len(g2))
	for code, r := range g2 {
		if code > 0x21 {
			m[r] = code
		}
	}
	return m
}()
