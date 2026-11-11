package cea708_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
)

const (
	window0 = "98 38 DA 32 71 1F 09" // window 0, visible, bottom center, 2 rows
	window1 = "99 18 8A 0A 00 1F 09" // window 1, hidden, top left
)

// blockBytes reads a block as hex tokens and "t:text" tokens. An underscore in the text is a space.
// A leading "@N" token sets the service, which is 1 without it.
func blockBytes(t *testing.T, block string) (service int, data []byte) {
	t.Helper()
	service = 1
	for _, tok := range strings.Fields(block) {
		if n, ok := strings.CutPrefix(tok, "@"); ok {
			service, _ = strconv.Atoi(n)
		} else if text, ok := strings.CutPrefix(tok, "t:"); ok {
			data = append(data, strings.ReplaceAll(text, "_", " ")...)
		} else {
			b, err := strconv.ParseUint(tok, 16, 8)
			require.NoError(t, err, tok)
			data = append(data, byte(b))
		}
	}
	return service, data
}

// packet returns the triplets of one DTVCC packet with the blocks. Blocks of service 7 and above use the extended header.
func packet(t *testing.T, blocks ...string) []cea708.Triplet {
	t.Helper()
	body := []byte{0}
	for _, block := range blocks {
		service, data := blockBytes(t, block)
		if service < 7 {
			body = append(body, byte(service<<5|len(data)))
		} else {
			body = append(body, byte(7<<5|len(data)), byte(service))
		}
		body = append(body, data...)
	}
	if len(body)%2 == 1 {
		body = append(body, 0)
	}
	body[0] = byte(len(body) / 2)
	var triplets []cea708.Triplet
	for i := 0; i < len(body); i += 2 {
		typ := byte(cea708.DTVCCData)
		if i == 0 {
			typ = cea708.DTVCCStart
		}
		triplets = append(triplets, cea708.Triplet{Type: typ, Data: [2]byte{body[i], body[i+1]}})
	}
	return triplets
}

func describe(states []cea708.State) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = fmt.Sprintf("%g|%d|%s", s.Time.Seconds(), s.AnchorPoint, strings.Join(s.Lines, "/"))
	}
	return out
}

func TestDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		service int
		packets [][]string // one packet per second, each with its blocks
		want    []string   // time in seconds, anchor point, lines joined with "/"
	}{
		{
			name:    "characters of G0, G1 and G2",
			packets: [][]string{{window0 + " 43 61 66 E9 20 10 25 20 7F"}},
			want:    []string{"0|7|Caf\u00e9 \u2026 \u266a"},
		},
		{
			name:    "show a hidden window",
			packets: [][]string{{window1 + " t:Hidden"}, {"89 02"}},
			want:    []string{"1|0|Hidden"},
		},
		{
			name:    "toggle and hide",
			packets: [][]string{{window1 + " t:Two"}, {"8B 02"}, {"8A 02"}},
			want:    []string{"1|0|Two", "2|-1|"},
		},
		{
			name:    "clear and delete",
			packets: [][]string{{window0 + " t:One"}, {"88 01"}, {window0 + " t:Two"}, {"8C 01"}},
			want:    []string{"0|7|One", "1|-1|", "2|7|Two", "3|-1|"},
		},
		{
			name:    "carriage return scrolls on the last row",
			packets: [][]string{{window0 + " t:A 0D t:B"}, {"0D t:C"}},
			want:    []string{"0|7|A/B", "1|7|B/C"},
		},
		{
			name:    "backspace and horizontal carriage return",
			packets: [][]string{{window0 + " t:ABC 08 08 t:D 0D t:EF 0E t:G"}},
			want:    []string{"0|7|AD/G"},
		},
		{
			name:    "form feed clears the window",
			packets: [][]string{{window0 + " t:Old 0C t:New"}},
			want:    []string{"0|7|New"},
		},
		{
			name:    "pen location",
			packets: [][]string{{window0 + " 92 01 00 t:Low 92 00 02 t:Up"}},
			want:    []string{"0|7|Up/Low"},
		},
		{
			name:    "pen attributes and color",
			packets: [][]string{{window0 + " 90 01 80 t:it 90 01 40 t:un 90 01 00 91 30 00 00 t:red"}},
			want:    []string{"0|7|<i>it</i><u>un</u><font color=\"#ff0000\">red</font>"},
		},
		{
			name: "skipped codes",
			packets: [][]string{
				{window0},
				{"t:a 11 41 t:b 18 AC 00 t:c 10 08 41 t:d"},
				{"10 80 01 02 03 04 t:e 10 90 02 AA BB t:f 10 A0 t:g 8D 05 t:h"},
			},
			want: []string{"1|7|abcd", "2|7|abcdefgh"},
		},
		{
			name:    "a character goes to the current window and reset deletes all",
			packets: [][]string{{window0, window1 + " t:x 80 t:y"}, {"8F"}},
			want:    []string{"0|7|y", "1|-1|"},
		},
		{
			name:    "absolute anchor",
			packets: [][]string{{"98 38 46 C8 80 1F 09 t:Hi"}},
			want:    []string{"0|8|Hi"},
		},
		{
			name:    "service 2 with the extended header ignores the blocks of service 9",
			service: 2,
			packets: [][]string{{"@9 " + window0 + " t:Nine", "@2 " + window0 + " t:Two"}},
			want:    []string{"0|7|Two"},
		},
		{
			name:    "service 9 with the extended header",
			service: 9,
			packets: [][]string{{"@9 " + window0 + " t:Nine", "@2 " + window0 + " t:Two"}},
			want:    []string{"0|7|Nine"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := max(tt.service, 1)
			var frames []cea708.Frame
			for i, blocks := range tt.packets {
				frames = append(frames, cea708.Frame{Time: time.Duration(i) * time.Second, Triplets: packet(t, blocks...)})
			}
			assert.Equal(t, tt.want, describe(cea708.NewDecoder(service).Decode(frames)))
		})
	}
}

func TestDecodePacketOverTwoFrames(t *testing.T) {
	t.Parallel()
	triplets := packet(t, window0+" t:Hello")
	require.Greater(t, len(triplets), 1)
	frames := []cea708.Frame{
		{Time: 0, Triplets: triplets[:1]},
		{Time: 500 * time.Millisecond, Triplets: triplets[1:]},
	}
	assert.Equal(t, []string{"0.5|7|Hello"}, describe(cea708.NewDecoder(1).Decode(frames)))
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{3, 5, 0x21, 2, 0x98, 0x38, 2, 0xDA, 0x32, 2, 0x71, 0x1F, 2, 0x09, 0x41})
	f.Add([]byte{3, 0, 0x21, 2, 0x10, 0x25, 2, 0x8F, 0x7F, 2, 0x0D, 0x0C})
	f.Fuzz(func(t *testing.T, data []byte) {
		var triplets []cea708.Triplet
		for ; len(data) >= 3; data = data[3:] {
			triplets = append(triplets, cea708.Triplet{Type: 2 + data[0]&1, Data: [2]byte{data[1], data[2]}})
		}
		for _, s := range cea708.NewDecoder(1).Decode([]cea708.Frame{{Triplets: triplets}}) {
			for _, l := range s.Lines {
				if strings.ContainsFunc(l, func(r rune) bool { return r < 0x20 || r >= 0x7F && r < 0xA0 }) {
					t.Fatalf("control character in %q", l)
				}
			}
		}
	})
}
