package teletext

import (
	"errors"
	"strconv"
)

// nationalPositions are the 13 positions of a national option subset in the Latin G0 set.
var nationalPositions = [13]int{0x23, 0x24, 0x40, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F, 0x60, 0x7B, 0x7C, 0x7D, 0x7E} //nolint:gochecknoglobals // lookup table

// nationalSubsets holds the characters of each national option subset of ETS 300 706 table 36.
//
//nolint:gochecknoglobals // lookup table
var nationalSubsets = map[string]string{
	"english":    "\u00A3$@\u2190\u00BD\u2192\u2191#\u2014\u00BC\u2016\u00BE\u00F7",
	"french":     "\u00E9\u00EF\u00E0\u00EB\u00EA\u00F9\u00EE#\u00E8\u00E2\u00F4\u00FB\u00E7",
	"swedish":    "#\u00A4\u00C9\u00C4\u00D6\u00C5\u00DC_\u00E9\u00E4\u00F6\u00E5\u00FC",
	"czech":      "#\u016F\u010D\u0165\u017E\u00FD\u00ED\u0159\u00E9\u00E1\u011B\u00FA\u0161",
	"german":     "#$\u00A7\u00C4\u00D6\u00DC^_\u00B0\u00E4\u00F6\u00FC\u00DF",
	"spanish":    "\u00E7$\u00A1\u00E1\u00E9\u00ED\u00F3\u00FA\u00BF\u00FC\u00F1\u00E8\u00E0",
	"italian":    "\u00A3$\u00E9\u00B0\u00E7\u2192\u2191#\u00F9\u00E0\u00F2\u00E8\u00EC",
	"polish":     "#\u0144\u0105\u01B5\u015A\u0141\u0107\u00F3\u0119\u017C\u015B\u0142\u017A",
	"turkish":    "\u20BA\u011F\u0130\u015E\u00D6\u00C7\u00DC\u011E\u0131\u015F\u00F6\u00E7\u00FC",
	"serbian":    "#\u00CB\u010C\u0106\u017D\u0110\u0160\u00EB\u010D\u0107\u017E\u0111\u0161",
	"romanian":   "#\u00A4\u0162\u00C2\u015E\u0102\u00CE\u0131\u0163\u00E2\u015F\u0103\u00EE",
	"estonian":   "#\u00F5\u0160\u00C4\u00D6\u017D\u00DC\u00D5\u0161\u00E4\u00F6\u017E\u00FC",
	"lithuanian": "#$\u0160\u0117\u0119\u017D\u010D\u016B\u0161\u0105\u0173\u017E\u012F",
}

// nonLatinSets holds the G0 sets from 0x20 to 0x7F of ETS 300 706 tables 33 and 35 that differ from ASCII.
//
//nolint:gochecknoglobals // lookup table
var nonLatinSets = map[string]string{
	"cyrillic1": " !\"#$%&'()*+,-./0123456789:;<=>?\u0427\u0410\u0411\u0426\u0414\u0415\u0424\u0413\u0425\u0418\u0408\u041A\u041B\u041C\u041D\u041E\u041F\u040C\u0420\u0421\u0422\u0423\u0412\u0403\u0409\u040A\u0417\u040B\u0416\u0402\u0428\u040F\u0447\u0430\u0431\u0446\u0434\u0435\u0444\u0433\u0445\u0438\u0458\u043A\u043B\u043C\u043D\u043E\u043F\u045C\u0440\u0441\u0442\u0443\u0432\u0453\u0459\u045A\u0437\u045B\u0436\u0452\u0448\u25A0",
	"cyrillic2": " !\"#$%\u044B'()*+,-./0123456789:;<=>?\u042E\u0410\u0411\u0426\u0414\u0415\u0424\u0413\u0425\u0418\u0419\u041A\u041B\u041C\u041D\u041E\u041F\u042F\u0420\u0421\u0422\u0423\u0416\u0412\u042C\u042A\u0417\u0428\u042D\u0429\u0427\u042B\u044E\u0430\u0431\u0446\u0434\u0435\u0444\u0433\u0445\u0438\u0439\u043A\u043B\u043C\u043D\u043E\u043F\u044F\u0440\u0441\u0442\u0443\u0436\u0432\u044C\u044A\u0437\u0448\u044D\u0449\u0447\u25A0",
	"cyrillic3": " !\"#$%\u0457'()*+,-./0123456789:;<=>?\u042E\u0410\u0411\u0426\u0414\u0415\u0424\u0413\u0425\u0418\u0419\u041A\u041B\u041C\u041D\u041E\u041F\u042F\u0420\u0421\u0422\u0423\u0416\u0412\u042C\u0406\u0417\u0428\u0404\u0429\u0427\u0407\u044E\u0430\u0431\u0446\u0434\u0435\u0444\u0433\u0445\u0438\u0439\u043A\u043B\u043C\u043D\u043E\u043F\u044F\u0440\u0441\u0442\u0443\u0436\u0432\u044C\u0456\u0437\u0448\u0454\u0449\u0447\u25A0",
	"greek":     " !\"#$%&'()*+,-./0123456789:;\u00AB=\u00BB?\u0390\u0391\u0392\u0393\u0394\u0395\u0396\u0397\u0398\u0399\u039A\u039B\u039C\u039D\u039E\u039F\u03A0\u03A1\u0374\u03A3\u03A4\u03A5\u03A6\u03A7\u03A8\u03A9\u03AA\u03AB\u03AC\u03AD\u03AE\u03AF\u03B0\u03B1\u03B2\u03B3\u03B4\u03B5\u03B6\u03B7\u03B8\u03B9\u03BA\u03BB\u03BC\u03BD\u03BE\u03BF\u03C0\u03C1\u03C2\u03C3\u03C4\u03C5\u03C6\u03C7\u03C8\u03C9\u03CA\u03CB\u03CC\u03CD\u03CE\u25A0",
}

// groups holds the G0 set of each national option 0 to 7 in each group of ETS 300 706 table 33. The empty
// name is the Arabic or the Hebrew set.
//
//nolint:gochecknoglobals // lookup table
var groups = map[int][8]string{
	0:  {"english", "french", "swedish", "czech", "german", "spanish", "italian", "english"},
	1:  {"polish", "french", "swedish", "czech", "german", "english", "italian", "english"},
	2:  {"english", "french", "swedish", "turkish", "german", "spanish", "italian", "english"},
	3:  {"english", "french", "swedish", "czech", "german", "serbian", "italian", "romanian"},
	4:  {"cyrillic1", "cyrillic2", "estonian", "czech", "german", "cyrillic3", "lithuanian", "english"},
	6:  {"english", "french", "swedish", "turkish", "german", "spanish", "italian", "greek"},
	8:  {"english", "french", "swedish", "czech", "german", "spanish", "italian", ""},
	10: {"english", "french", "swedish", "czech", "german", "", "italian", ""},
}

// languageGroups is the group of a receiver in the country of a language, for streams without X/28 and M/29.
//
//nolint:gochecknoglobals // lookup table
var languageGroups = map[string]int{
	"pol": 1, "tur": 2, "hrv": 3, "slv": 3, "bos": 3, "ron": 3, "rum": 3,
	"rus": 4, "ukr": 4, "bul": 4, "srp": 4, "mkd": 4, "mac": 4, "bel": 4, "est": 4, "lav": 4, "lit": 4,
	"gre": 6, "ell": 6,
}

// LanguageGroup returns the character set group of an ISO 639-2 language code, 0 for an unknown one.
func LanguageGroup(language string) int {
	return languageGroups[language]
}

func errRightToLeft(page int) error {
	return errors.New("Teletext page " + strconv.Itoa(page) + " uses the Arabic or Hebrew character set. The reader reads Latin, Cyrillic and Greek pages.")
}

// g0 returns the characters from 0x20 to 0x7F of the G0 set of a group and a national option.
func g0(group, nationalOption int) ([]rune, bool) {
	row, ok := groups[group]
	if !ok {
		row = groups[0]
	}
	name := row[nationalOption]
	if name == "" {
		return nil, false
	}
	if set, ok := nonLatinSets[name]; ok {
		return []rune(set), true
	}
	chars := make([]rune, 96)
	for i := range 95 {
		chars[i] = rune(0x20 + i)
	}
	chars[95] = 0x25A0
	for i, r := range []rune(nationalSubsets[name]) {
		chars[nationalPositions[i]-0x20] = r
	}
	return chars, true
}
