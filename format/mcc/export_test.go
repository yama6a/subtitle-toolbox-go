package mcc

// The unexported helpers that the tests of package mcc_test call.

var (
	DecodeHex = decodeHex
	EncodeHex = encodeHex
	ANCPacket = ancPacket
)

// HeaderText is the header text of the writer.
const HeaderText = headerText
