package pac_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// footer is the default end block of a new file.
const footer = "\xFF\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x11\x00dummy end of file"

// file returns a PAC file with one record per text, from 1 s to 2 s at 25 fps, numbered from 0. Each text follows
// the line start FEh 02h 03h.
func file(texts ...string) []byte {
	out := append([]byte{1}, make([]byte, 19)...)
	for i, text := range texts {
		text = "\xFE\x02\x03" + text
		out = append(out, 0)
		out = binary.LittleEndian.AppendUint16(out, uint16(i))
		out = append(out, 0x60)
		for _, n := range [...]uint16{0, 100, 0, 200} {
			out = binary.LittleEndian.AppendUint16(out, n)
		}
		out = binary.LittleEndian.AppendUint16(out, uint16(len(text)+1))
		out = append(out, 0x0B)
		out = append(out, text...)
	}
	return append(out, footer...)
}

func read(t *testing.T, content []byte, lenient bool, opts pac.ReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := pac.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, opts)
	require.NoError(t, err)
	return s, warnings
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) []byte {
	t.Helper()

	var out bytes.Buffer
	_, err := pac.Write(&out, s, common, pac.WriteOptions{})
	require.NoError(t, err)
	return out.Bytes()
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }
