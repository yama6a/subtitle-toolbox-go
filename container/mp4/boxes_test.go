package mp4_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/container/mp4"
)

// moovHeader rewrites the header of the moov box, which must be the last box of the file.
func moovHeader(t *testing.T, content []byte, kind string) []byte {
	t.Helper()
	at := bytes.LastIndex(content, []byte("moov")) - 4
	require.Equal(t, len(content), at+int(uint32(content[at])<<24|uint32(content[at+1])<<16|uint32(content[at+2])<<8|uint32(content[at+3])))
	payload := content[at+8:]
	switch kind {
	case "size 0":
		return cat(content[:at], u32(0), []byte("moov"), payload)
	default:
		return cat(content[:at], u32(1), []byte("moov"), u64(uint64(len(payload)+16)), payload)
	}
}

func TestBoxVersions(t *testing.T) {
	t.Parallel()

	plain := file([]trackSpec{{
		id: 1, handler: "sbtl", entry: tx3gEntry(), perChunk: 1,
		version1: true, co64: true, stz2: true, timescale: 90000, elstV1: true, edits: [][2]int64{{1000, -1}, {500, 0}},
		samples: []sampleSpec{{90000, textSample("one")}, {90000, textSample("two")}},
	}}, false)
	want := []cueRow{{ms(1000), ms(2000), []string{"one"}, false}, {ms(2000), ms(3000), []string{"two"}, false}}

	tests := []struct {
		name    string
		content []byte
	}{
		{"version 1 boxes, co64, stz2 and an edit list", plain},
		{"moov box with size 0", moovHeader(t, plain, "size 0")},
		{"moov box with a 64-bit size", moovHeader(t, plain, "large")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, rows(extract(t, openBytes(t, tt.content), 1, mp4.ExtractOptions{})))
		})
	}
}

func TestEditList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		edits [][2]int64
		want  []cueRow
	}{
		{"empty edits move the samples", [][2]int64{{1500, -1}, {500, 250}, {1000, 0}}, []cueRow{{ms(1250), ms(2250), []string{"Hi"}, false}}},
		{"a media time drops the earlier samples", [][2]int64{{1000, 2000}}, nil},
		{"a media time cuts the start of a sample", [][2]int64{{1000, 400}}, []cueRow{{0, ms(600), []string{"Hi"}, false}}},
		{"an edit list with empty edits only", [][2]int64{{1000, -1}}, []cueRow{{0, ms(1000), []string{"Hi"}, false}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := extract(t, openBytes(t, single(trackSpec{edits: tt.edits}, textSample("Hi"))), 1, mp4.ExtractOptions{})
			if tt.want == nil {
				assert.Empty(t, s.Cues())
				return
			}
			assert.Equal(t, tt.want, rows(s))
		})
	}
}
