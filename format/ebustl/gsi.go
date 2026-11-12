package ebustl

import (
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/codepage"
)

const (
	gsiSize        = 1024
	ttiSize        = 128
	ttiHeaderSize  = 16
	timeCodeSize   = 4
	defaultColumns = 40
	teletextRows   = 23
)

// Offsets of the TTI block fields, EBU Tech 3264 table 2.
const (
	ttiSGN = 0
	ttiSN  = 1
	ttiEBN = 3
	ttiCS  = 4
	ttiTCI = 5
	ttiTCO = 9
	ttiVP  = 13
	ttiJC  = 14
	ttiCF  = 15
)

const (
	lastBlock   = 0xFF
	unusedSpace = 0x8F
	commentFlag = 1
)

type gsiField struct {
	name   string
	offset int
	length int
}

// gsiFields holds the fields of the GSI block in file order, EBU Tech 3264 table 1.
//
//nolint:gochecknoglobals // lookup table of EBU Tech 3264 table 1
var gsiFields = []gsiField{
	{"CPN", 0, 3},
	{"DFC", 3, 8},
	{"DSC", 11, 1},
	{"CCT", 12, 2},
	{"LC", 14, 2},
	{"OPT", 16, 32},
	{"OET", 48, 32},
	{"TPT", 80, 32},
	{"TET", 112, 32},
	{"TN", 144, 32},
	{"TCD", 176, 32},
	{"SLR", 208, 16},
	{"CD", 224, 6},
	{"RD", 230, 6},
	{"RN", 236, 2},
	{"TNB", 238, 5},
	{"TNS", 243, 5},
	{"TNG", 248, 3},
	{"MNC", 251, 2},
	{"MNR", 253, 2},
	{"TCS", 255, 1},
	{"TCP", 256, 8},
	{"TCF", 264, 8},
	{"TND", 272, 1},
	{"DSN", 273, 1},
	{"CO", 274, 3},
	{"PUB", 277, 32},
	{"EN", 309, 32},
	{"ECD", 341, 32},
	{"spare", 373, 75},
	{"UDA", 448, 576},
}

// languages maps the LC field to a BCP 47 tag, EBU Tech 3264 appendix 3.
//
//nolint:gochecknoglobals // lookup table of EBU Tech 3264 appendix 3
var languages = map[string]string{
	"01": "sq", "02": "br", "03": "ca", "04": "hr", "05": "cy", "06": "cs", "07": "da", "08": "de",
	"09": "en", "0A": "es", "0B": "eo", "0C": "et", "0D": "eu", "0E": "fo", "0F": "fr", "10": "fy",
	"11": "ga", "12": "gd", "13": "gl", "14": "is", "15": "it", "16": "se", "17": "la", "18": "lv",
	"19": "lb", "1A": "lt", "1B": "hu", "1C": "mt", "1D": "nl", "1E": "no", "1F": "oc", "20": "pl",
	"21": "pt", "22": "ro", "23": "rm", "24": "sr", "25": "sk", "26": "sl", "27": "fi", "28": "sv",
	"29": "tr", "2A": "nl-BE", "2B": "wa",
	"45": "zu", "46": "vi", "47": "uz", "48": "ur", "49": "uk", "4A": "th", "4B": "te", "4C": "tt",
	"4D": "ta", "4E": "tg", "4F": "sw", "50": "srn", "51": "so", "52": "si", "53": "sn", "54": "sh",
	"55": "rue", "56": "ru", "57": "qu", "58": "ps", "59": "pa", "5A": "fa", "5B": "pap", "5C": "or",
	"5D": "ne", "5E": "nd", "5F": "mr", "60": "mo", "61": "ms", "62": "mg", "63": "mk", "64": "lo",
	"65": "ko", "66": "km", "67": "kk", "68": "kn", "69": "ja", "6A": "id", "6B": "hi", "6C": "he",
	"6D": "ha", "6E": "gn", "6F": "gu", "70": "el", "71": "ka", "72": "ff", "73": "prs", "74": "cv",
	"75": "zh", "76": "my", "77": "bg", "78": "bn", "79": "be", "7A": "bm", "7B": "az", "7C": "as",
	"7D": "hy", "7E": "ar", "7F": "am",
}

// gsiTable returns the code page of the CPN field. An unknown code reads as 850.
func gsiTable(cpn string) *codepage.Table {
	switch cpn {
	case "437":
		return codepage.CP437
	case "860":
		return codepage.CP860
	case "863":
		return codepage.CP863
	case "865":
		return codepage.CP865
	default:
		return codepage.CP850
	}
}

// decodeText decodes bytes with the character code table cct, which is "00" to "04".
func decodeText(cct string, b []byte) string {
	switch cct {
	case "01":
		return codepage.ISO8859_5.Decode(b)
	case "02":
		return codepage.ISO8859_6.Decode(b)
	case "03":
		return codepage.ISO8859_7_1987.Decode(b)
	case "04":
		return codepage.ISO8859_8_1988.Decode(b)
	default:
		return codepage.DecodeISO6937(b)
	}
}

// gsiValues holds the value of each GSI field in the order of gsiFields.
type gsiValues []string

func (g gsiValues) get(name string) string {
	i := slices.IndexFunc(gsiFields, func(f gsiField) bool { return f.name == name })
	return g[i]
}

// readGSI returns the value of each field of the GSI block, decoded with the code page of CPN and without
// trailing spaces.
func readGSI(block []byte) gsiValues {
	table := gsiTable(string(block[:3]))
	values := make(gsiValues, len(gsiFields))
	for i, f := range gsiFields {
		values[i] = strings.TrimRight(table.Decode(block[f.offset:f.offset+f.length]), " ")
	}
	return values
}

// fieldInt reads a GSI number the way the PHP (int) cast does. It skips leading white space, reads an optional
// sign and the leading digits, and gives 0 when there are none. So "2x" is 2.
func fieldInt(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\v\f")
	sign := 1
	if s != "" && (s[0] == '+' || s[0] == '-') {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	n := 0
	for i := 0; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
	}
	return sign * n
}
