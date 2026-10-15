package parsing

import (
	"bytes"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	utf8BOM     = "\ufeff"
	ctrlZ       = 0x1a
	whiteSpace  = " \t\r\n"
	replacement = "\ufffd"
)

// Prepare runs the text pre-pass on UTF-8 content and returns the content for the reader.
// False means that the content is blank, so the reader returns an empty subtitle.
// Prepare never changes content. The flag says whether the format reads invalid UTF-8 as U+FFFD.
// The UTF-8 check that uses it arrives with https://github.com/yama6a/subtitle-toolbox-go/issues/16.
func (s *State) Prepare(content []byte, _ bool) ([]byte, bool, error) {
	for bytes.HasPrefix(content, []byte(utf8BOM)) {
		content = content[len(utf8BOM):]
	}
	content = trimCtrlZ(content)
	if s.ReplaceNUL {
		content = bytes.ReplaceAll(content, []byte{0}, []byte(replacement))
	}
	if i := bytes.IndexByte(content, 0); i >= 0 && !s.Lenient() {
		line := lineAt(content, i)
		return nil, false, &model.ParseError{
			Format: s.format,
			Line:   line,
			Msg:    "Line " + strconv.Itoa(line) + " has a NUL character. Check the encoding of the file.",
		}
	}
	if i := slices.IndexFunc(content, isControl); i >= 0 {
		if s.Lenient() {
			line := lineAt(content, i)
			s.warn("The file has control characters, the first on line "+strconv.Itoa(line)+". The parser removed them.", line, model.Repaired)
		}
		content = slices.DeleteFunc(slices.Clone(content), isControl)
	}
	if len(bytes.Trim(content, whiteSpace)) == 0 {
		return content, false, nil
	}
	return content, true, nil
}

// trimCtrlZ removes the last run of Ctrl-Z when only white space follows it. DOS editors end files with it.
func trimCtrlZ(content []byte) []byte {
	end := len(bytes.TrimRight(content, whiteSpace))
	start := end
	for start > 0 && content[start-1] == ctrlZ {
		start--
	}
	if start == end {
		return content
	}
	return slices.Concat(content[:start], content[end:])
}

func isControl(b byte) bool {
	return b < 0x20 && b != '\t' && b != '\n' && b != '\r' || b == 0x7f
}

// lineAt returns the 1-based line of offset i, which holds no CR or LF. LF, a run of CRs and LF, and a lone CR
// each end a line, as the PHP pattern /\r+\n|\r/ reads them.
func lineAt(content []byte, i int) int {
	line := 1
	for j := 0; j < i; j++ {
		switch content[j] {
		case '\n':
			line++
		case '\r':
			k := j
			for content[k+1] == '\r' {
				k++
			}
			if content[k+1] == '\n' {
				line++
				k++
			} else {
				line += k - j + 1
			}
			j = k
		}
	}
	return line
}
