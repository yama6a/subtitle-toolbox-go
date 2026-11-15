package mcc

// The unexported helpers that the tests of package mcc_test call.

var (
	DecodeHex = decodeHex
	ANCPacket = ancPacket
)

// HeaderText is the header text of the writer.
const HeaderText = headerText

// EncodeHex writes bytes as upper-case hex digits and alias letters. U stays unused, as files disagree on its length.
func EncodeHex(data []byte) string { return string(appendHex(nil, data)) }
