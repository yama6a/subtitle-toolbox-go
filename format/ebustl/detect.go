package ebustl

import "bytes"

// Detect reports whether content starts with 3 ASCII digits and STL25.01 or STL30.01. It reads only the first
// 11 bytes, so the BOM and line ending steps of stbox.Detect do not change the result.
func Detect(content []byte) bool {
	if len(content) < 11 {
		return false
	}
	for _, c := range content[:3] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return bytes.HasPrefix(content[3:], []byte("STL25.01")) || bytes.HasPrefix(content[3:], []byte("STL30.01"))
}
