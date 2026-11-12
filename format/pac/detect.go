package pac

import "bytes"

// Detect reports whether content starts with the PAC header and a record start with a record mark: 01h and 20 bytes
// 00h, 2 bytes of the subtitle number and a mark from 60h to 67h. The content must have no leading BOM or white
// space and LF line endings, as stbox.Detect passes it. A first subtitle number with the bytes 0Dh and 0Ah
// therefore does not detect.
func Detect(content []byte) bool {
	const length = headerLength + 1 + 2 + 1
	return len(content) >= length && bytes.HasPrefix(content, header) && content[headerLength] == recordStart &&
		content[length-1] >= recordMark && content[length-1] <= lastRecordMark
}
