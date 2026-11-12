package cea708_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
)

func TestReadCDP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cdp          string
		wantCode     int
		wantTriplets []cea708.Triplet
		wantErr      string
	}{
		{name: "no identifier", cdp: "0A 96 70 0000000000", wantErr: "The data holds no caption distribution packet."},
		{name: "6 bytes", cdp: "96 69 06 4F 43 00", wantErr: "The data holds no caption distribution packet."},
		{name: "no sections", cdp: "96 69 07 4F 43 00 00", wantCode: 4},
		{
			name:     "cc_data after a time code section",
			cdp:      "96 69 18 7F 43 00 01 71 00 00 00 00 72 E4 FC 94 2C F4 80 80 FF 41 42 FA 00 00",
			wantCode: 7,
			wantTriplets: []cea708.Triplet{
				{Type: cea708.Field1, Data: [2]byte{0x94, 0x2C}},
				{Type: cea708.Field1, Data: [2]byte{0x80, 0x80}},
				{Type: cea708.DTVCCStart, Data: [2]byte{0x41, 0x42}},
			},
		},
		{name: "short cc_data", cdp: "96 69 0A 4F 43 00 00 72 F4 FC 94", wantErr: "The caption distribution packet is shorter than its 20 cc_data triplets."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, triplets, err := cea708.ReadCDP(hexBytes(t, tt.cdp))
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantTriplets, triplets)
		})
	}
}

func TestWriteCDPReadsBackTheValidTriplets(t *testing.T) {
	t.Parallel()

	in := []cea708.CDPTriplet{
		{Type: cea708.Field1, Valid: true, Data: 0x942C},
		{Type: cea708.Field2, Valid: true, Data: 0x8080},
		{Type: cea708.DTVCCStart, Valid: true, Data: 0x4142},
		{Type: cea708.DTVCCData, Valid: false},
	}
	cdp := cea708.WriteCDP(5, 0x0102, in)

	var sum byte
	for _, b := range cdp {
		sum += b
	}
	assert.Zero(t, sum)
	assert.Equal(t, len(cdp), int(cdp[2]))
	code, triplets, err := cea708.ReadCDP(cdp)
	require.NoError(t, err)
	assert.Equal(t, 5, code)
	assert.Equal(t, []cea708.Triplet{
		{Type: cea708.Field1, Data: [2]byte{0x94, 0x2C}},
		{Type: cea708.DTVCCStart, Data: [2]byte{0x41, 0x42}},
	}, triplets)
}

func FuzzCDP(f *testing.F) {
	f.Add([]byte{0x96, 0x69, 0x07, 0x4F, 0x43, 0, 0})
	f.Add(cea708.WriteCDP(4, 7, []cea708.CDPTriplet{{Type: cea708.Field1, Valid: true, Data: 0x942C}, {Type: cea708.DTVCCData}}))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = cea708.ReadCDP(data)

		triplets := make([]cea708.CDPTriplet, 0, len(data)/3)
		var want []cea708.Triplet
		for i := 0; i+2 < len(data) && len(triplets) < 31; i += 3 {
			tr := cea708.CDPTriplet{Type: data[i] & 3, Valid: data[i]&4 != 0, Data: uint16(data[i+1])<<8 | uint16(data[i+2])}
			triplets = append(triplets, tr)
			if tr.Valid && tr.Type != cea708.Field2 {
				want = append(want, cea708.Triplet{Type: tr.Type, Data: [2]byte{data[i+1], data[i+2]}})
			}
		}
		_, got, err := cea708.ReadCDP(cea708.WriteCDP(4, 0, triplets))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("read %d triplets, wrote %d valid", len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("triplet %d is %v, want %v", i, got[i], want[i])
			}
		}
	})
}

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()

	_, data := blockBytes(t, s)
	return data
}
