package mcc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
)

func TestDecodeHex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, text string
		want       []byte
		wantMsg    string
	}{
		{"aliases and lower-case hex", "HPQRSTUZ9fA0", []byte{
			0xFA, 0, 0, 0xFA, 0, 0, 0xFB, 0x80, 0x80, 0xFC, 0x80, 0x80, 0xFD, 0x80, 0x80, 0x96, 0x69, 0x61, 0x01, 0xE1, 0, 0, 0, 0, 0x9F, 0xA0,
		}, ""},
		{"invalid character", "T49SV", nil, `The MCC data has the invalid character "V" at position 5.`},
		{"alias after half a byte", "T4S", nil, `The MCC data has the invalid character "S" at position 3.`},
		{"half a byte", "T4", nil, "The MCC data ends with half a byte."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, msg := mcc.DecodeHex(tt.text)
			assert.Equal(t, tt.wantMsg, msg)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeHex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"runs of empty triplets take the longest letter", append(make([]byte, 0), repeat([]byte{0xFA, 0, 0}, 10)...), "OG"},
		{"letters for the fixed byte runs", []byte{0xFB, 0x80, 0x80, 0x96, 0x69, 0x61, 0x01, 0x00, 0xFA, 0x00}, "PSTZFAZ"},
		{"U is never written", []byte{0xE1, 0, 0, 0}, "E1ZZZ"},
		{"other bytes are upper-case hex", []byte{0x9F, 0xA0, 0x0B}, "9FA00B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text := mcc.EncodeHex(tt.data)
			assert.Equal(t, tt.want, text)
			back, msg := mcc.DecodeHex(text)
			require.Empty(t, msg)
			assert.Equal(t, tt.data, back)
		})
	}
}

func repeat(b []byte, n int) []byte {
	out := make([]byte, 0, len(b)*n)
	for range n {
		out = append(out, b...)
	}
	return out
}
