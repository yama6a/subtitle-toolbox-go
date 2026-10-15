package parsing

import (
	"bytes"
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	ctrlZ       = 0x1a
	whiteSpace  = " \t\r\n"
	replacement = "\ufffd"
)

// Prepare runs the text pre-pass on UTF-8 content and returns the content for the reader.
// False means that the content is blank, so the reader returns an empty subtitle.
// Prepare never changes content. replacesInvalidUTF8 says whether the format reads invalid UTF-8 as U+FFFD.
func (s *State) Prepare(content []byte, replacesInvalidUTF8 bool) ([]byte, bool, error) {
	content, err := s.checkUTF8(content, replacesInvalidUTF8)
	if err != nil {
		return nil, false, err
	}
	content = trimCtrlZ(textenc.TrimUTF8BOMs(content))
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

// checkUTF8 warns in lenient mode about content that is not valid UTF-8. A format that replaces invalid UTF-8
// fails in strict mode and reads each bad byte as U+FFFD in lenient mode. The offset counts from before the BOM.
func (s *State) checkUTF8(content []byte, replaces bool) ([]byte, error) {
	i := textenc.InvalidUTF8Offset(content)
	if i < 0 {
		return content, nil
	}
	msg := "The content is not valid UTF-8. The first bad byte is at offset " + strconv.Itoa(i) + ". Set the source encoding of the file."
	switch {
	case !replaces:
		if s.Lenient() {
			s.warn(msg, model.NoLine, model.Repaired)
		}
		return content, nil
	case !s.Lenient():
		return nil, &model.ParseError{Format: s.format, Msg: msg}
	default:
		s.warn(msg+" The parser read the bad bytes as U+FFFD.", model.NoLine, model.Repaired)
		return textenc.ReplaceInvalidUTF8(content), nil
	}
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
