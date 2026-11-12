package pgs

// Detect reports whether content starts with a PGS segment header of a known segment type: the bytes PG, and at byte 10
// one of 14h, 15h, 16h, 17h and 80h. The content needs at least 11 bytes.
//
// The check reads the PTS and the DTS at bytes 2 to 9, which can hold 0Dh or 0Ah. So the caller passes the raw
// bytes, without the BOM removal and line end change that stbox.Detect applies to text.
func Detect(content []byte) bool {
	if len(content) < 11 || content[0] != 'P' || content[1] != 'G' {
		return false
	}
	switch content[10] {
	case 0x14, 0x15, 0x16, 0x17, 0x80:
		return true
	}
	return false
}
