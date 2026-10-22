// Package ssatag reads the alignment override tags of SSA and ASS, which SubRip files carry too.
package ssatag

import "strings"

// legacyAlignments maps the SSA codes of \a to numpad values: 1 to 3 are bottom, +4 is top, +8 is middle.
//
//nolint:gochecknoglobals // lookup table of the legacy SSA alignment codes
var legacyAlignments = map[string]int{
	"1": 1, "2": 2, "3": 3, "5": 7, "6": 8, "7": 9, "9": 4, "10": 5, "11": 6,
	"01": 1, "02": 2, "03": 3, "05": 7, "06": 8, "07": 9, "09": 4,
}

// Alignment returns the numpad alignment of an \an or \a tag, for example 8 for `\an8` and for `\a6`.
// It returns false for another tag. One final LF after the tag passes, as the PHP pattern end $ allows it.
func Alignment(tag string) (numpad int, ok bool) {
	tag = strings.TrimSuffix(tag, "\n")
	if n, ok := strings.CutPrefix(tag, `\an`); ok {
		if len(n) == 1 && n[0] >= '1' && n[0] <= '9' {
			return int(n[0] - '0'), true
		}
		return 0, false
	}
	code, ok := strings.CutPrefix(tag, `\a`)
	if !ok {
		return 0, false
	}
	numpad, ok = legacyAlignments[code]
	return numpad, ok
}

// LegacyCode returns the SSA code of \a for a numpad alignment from 1 to 9, for example "6" for 8.
func LegacyCode(numpad int) string {
	codes := [10]string{1: "1", 2: "2", 3: "3", 4: "9", 5: "10", 6: "11", 7: "5", 8: "6", 9: "7"}
	if numpad < 1 || numpad > 9 {
		return ""
	}
	return codes[numpad]
}
