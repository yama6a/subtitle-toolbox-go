package ebustl_test

import (
	"bytes"
	"cmp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/ebustl"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// gsiOffsets holds the offset and the length of the GSI fields that the tests set.
var gsiOffsets = map[string][2]int{
	"CPN": {0, 3}, "DFC": {3, 8}, "DSC": {11, 1}, "CCT": {12, 2}, "LC": {14, 2}, "OPT": {16, 32},
	"MNC": {251, 2}, "MNR": {253, 2}, "TCP": {256, 8},
}

// gsiBlock returns a GSI block of spaces with the code page 850, 25 fps, teletext, ISO 6937 and English.
// Each entry of fields replaces the bytes of one field.
func gsiBlock(fields map[string]string) []byte {
	block := bytes.Repeat([]byte{' '}, 1024)
	values := map[string]string{"CPN": "850", "DFC": "STL25.01", "DSC": "1", "CCT": "00", "LC": "09"}
	for k, v := range fields {
		values[k] = v
	}
	for k, v := range values {
		o := gsiOffsets[k]
		copy(block[o[0]:o[0]+o[1]], strings.Repeat(" ", o[1]))
		copy(block[o[0]:o[0]+o[1]], v)
	}
	return block
}

// tti describes one TTI block. The zero value of ebn is 0, so New sets 0xFF.
type tti struct {
	sgn, cs  byte
	sn       uint16
	ebn      byte
	tci, tco [4]byte
	vp, jc   byte
	// exactPos keeps vp and jc as set, also 0. Else 0 takes the default.
	exactPos bool
	cf       byte
	text     string
}

// ttiBlock returns a block of 128 bytes. Unset fields take the defaults: subtitle number 1, extension block FFh,
// 1 s and 5 frames to 2 s and 10 frames, row 22 and centered.
func ttiBlock(b tti) []byte {
	if b.sn == 0 {
		b.sn = 1
	}
	if b.ebn == 0 {
		b.ebn = 0xFF
	}
	if b.tci == ([4]byte{}) {
		b.tci, b.tco = [4]byte{0, 0, 1, 5}, [4]byte{0, 0, 2, 10}
	}
	if !b.exactPos {
		b.vp, b.jc = cmp.Or(b.vp, 22), cmp.Or(b.jc, 2)
	}
	out := []byte{b.sgn, byte(b.sn), byte(b.sn >> 8), b.ebn, b.cs}
	out = append(out, b.tci[:]...)
	out = append(out, b.tco[:]...)
	out = append(out, b.vp, b.jc, b.cf)
	return append(out, []byte(b.text+strings.Repeat("\x8F", 112-len(b.text)))...)
}

func readBytes(t *testing.T, content []byte, opts ebustl.ReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := ebustl.Read(bytes.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s, warnings
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
