// Package tmplayer reads and writes TMPlayer and TMPlayer+, the one-line-per-entry format of Polish subtitle downloads.
//
// A line is H:MM:SS:text with a start time in whole seconds. A cue ends at the next entry in time. The extension is .txt.
package tmplayer

// ReadOptions holds the settings of the TMPlayer reader. TMPlayer has none yet.
type ReadOptions struct{}

// WriteOptions holds the settings of the TMPlayer writer. TMPlayer has none yet.
type WriteOptions struct{}

// Detect reports whether content starts with a TMPlayer time and a separator, and not with a frame timecode.
//
// The time is digits, :, a minutes pair 00 to 59, :, and a seconds pair 00 to 59. Then comes an optional , and
// digits for a line number. Then comes = or :. A : does not count when it is followed by 1 or 2 digits, spaces or
// tabs, and a comma, or by digits:dd:dd. Those are the hh:mm:ss:ff frame timecodes of Spruce STL and CSV, such as
// 00:00:01:10,00:00:03:10. The text 00:00:01:5 trains leave matches. The content must have no leading BOM or white
// space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	i := digitRun(content, 0)
	if i == 0 || !timePair(content, i) || !timePair(content, i+3) {
		return false
	}
	i += 6
	if i < len(content) && content[i] == ',' {
		j := digitRun(content, i+1)
		if j == i+1 {
			return false
		}
		i = j
	}
	if i >= len(content) {
		return false
	}
	switch content[i] {
	case '=':
		return true
	case ':':
		return !frameTimecode(content[i+1:])
	}
	return false
}

// digitRun returns the index after the ASCII digits that start at content[i].
func digitRun(content []byte, i int) int {
	for i < len(content) && isDigit(content[i]) {
		i++
	}
	return i
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

// timePair reports whether content[i:] is :, a digit 0 to 5 and a digit.
func timePair(content []byte, i int) bool {
	return i+2 < len(content) && content[i] == ':' && '0' <= content[i+1] && content[i+1] <= '5' && isDigit(content[i+2])
}

// frameTimecode reports whether rest starts with 1 or 2 digits, spaces or tabs and a comma, or with digits:dd:dd.
func frameTimecode(rest []byte) bool {
	n := digitRun(rest, 0)
	return n > 0 && (n <= 2 && commaAfterBlanks(rest[n:]) || clockTail(rest[n:]))
}

func commaAfterBlanks(b []byte) bool {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
		i++
	}
	return i < len(b) && b[i] == ','
}

// clockTail reports whether b starts with :dd:dd.
func clockTail(b []byte) bool {
	return len(b) >= 6 && b[0] == ':' && isDigit(b[1]) && isDigit(b[2]) && b[3] == ':' && isDigit(b[4]) && isDigit(b[5])
}
