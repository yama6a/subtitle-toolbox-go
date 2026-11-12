package mpegts_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mpegts"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func TestTracks(t *testing.T) {
	t.Parallel()

	track := func(number int, language, name string) model.Track {
		return model.Track{
			Container: model.MPEGTS, Number: number, CodecID: "dvb_teletext", Format: model.SubRip,
			Language: language, Name: name,
		}
	}
	tests := []struct {
		name string
		file string
		want []model.Track
	}{
		{"teletext.ts", "teletext.ts", []model.Track{
			track(888, "deu", ""), track(777, "deu", "hearing impaired"), track(801, "eng", ""),
			track(802, "pol", ""), track(301, "eng", ""),
		}},
		{"one_page.ts", "one_page.ts", []model.Track{track(888, "eng", "")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, open(t, fixture(t, tt.file)).Tracks())
		})
	}
}

func TestTeletextPages(t *testing.T) {
	t.Parallel()

	charsets := map[int]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(fixture(t, "teletext_charsets.txt"))), "\n") {
		var number int
		_, text, _ := strings.Cut(line, " ")
		_, err := fmt.Sscan(line, &number)
		require.NoError(t, err)
		charsets[number] = text
	}
	yellow := `<font color="#ffff00">- Wir müssen uns beeilen!</font>`
	tests := []struct {
		number int
		want   []cueRow
	}{
		{888, []cueRow{
			cue(0.5, 3, "Der Zug fährt um acht Uhr.", yellow),
			cue(3, 5, `<font color="#00ffff">- Ja?</font> - Nein.`, `<font color="#00ff00">Grüße aus der Straße.</font>`),
			cue(6, 10, "Es ist 20 Uhr."),
			cue(11, 12, "Ende."),
		}},
		{777, []cueRow{cue(1, 2, "(Leise Musik)"), cue(2, 4, "(Leise Musik)", "Hallo?")}},
		{801, []cueRow{cue(1.5, 4.5, charsets[801])}},
		{802, []cueRow{cue(2.5, 5.5, charsets[802])}},
		{301, []cueRow{cue(6, 9, charsets[301])}},
	}
	ts := fixture(t, "teletext.ts")
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.number), func(t *testing.T) {
			t.Parallel()

			s, _, err := open(t, ts).Extract(tt.number, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cuesOf(s))
		})
	}
}

func TestTeletextBuiltPages(t *testing.T) {
	t.Parallel()

	r := int64(1 << 33)
	lastCue := 2500 * time.Millisecond
	tests := []struct {
		name string
		file func() []byte
		opts model.CommonReadOptions
		want []cueRow
	}{
		{"a last page lasts LastCueDuration", func() []byte {
			w := single(eng888())
			w.teletextPES(teletextPID, 90000, pageHeader(888), row(888, 22, startBox+"Bye."))
			return w.bytes()
		}, model.CommonReadOptions{LastCueDuration: &lastCue}, []cueRow{cue(0, 2.5, "Bye.")}},
		{"a last page ends at the last PTS", func() []byte {
			w := single(eng888())
			w.teletextPES(teletextPID, 90000, pageHeader(888), row(888, 22, startBox+"Bye."))
			w.teletextPES(teletextPID, 270000, fillingHeader(1))
			return w.bytes()
		}, model.CommonReadOptions{}, []cueRow{cue(0, 2, "Bye.")}},
		{"times count across the PTS wrap", func() []byte {
			w := single(eng888())
			w.teletextPES(teletextPID, r-90000, pageHeader(888), row(888, 22, startBox+"Before."), fillingHeader(8))
			w.teletextPES(teletextPID, 90000, pageHeader(888), fillingHeader(8))
			return w.bytes()
		}, model.CommonReadOptions{}, []cueRow{cue(0, 2, "Before.")}},
		{"a header with 2-bit errors ends the page of its magazine", func() []byte {
			damaged := header(850, true, 0, true, false)
			damaged[3] ^= 0x03
			w := single(eng888())
			w.teletextPES(teletextPID, 90000, pageHeader(888), row(888, 21, startBox+"Kept."), damaged, row(850, 22, startBox+"Other page."))
			w.teletextPES(teletextPID, 180000, fillingHeader(8))
			return w.bytes()
		}, model.CommonReadOptions{}, []cueRow{cue(0, 1, "Kept.")}},
		{"a markup character in the text is escaped", func() []byte {
			w := single(eng888())
			w.teletextPES(teletextPID, 90000, pageHeader(888), row(888, 22, startBox+"a<b & c>d"), fillingHeader(8))
			w.teletextPES(teletextPID, 180000, pageHeader(888), fillingHeader(8))
			return w.bytes()
		}, model.CommonReadOptions{}, []cueRow{cue(0, 1, "a&lt;b &amp; c&gt;d")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extract(t, tt.file(), 888, tt.opts))
		})
	}
}

func TestNumberClash(t *testing.T) {
	t.Parallel()

	w := newWriter()
	w.tables(1, pmtPID, teletextPID,
		teletextStream(teletextPID, ttPage{"deu", subtitlePageType, 150}),
		teletextStream(0x0201, ttPage{"fra", subtitlePageType, 150}, ttPage{"Fra", subtitlePageType, 150}, ttPage{"f1a", hearingImpairedPageType, 151}))
	w.teletextPES(0x0201, 90000, pageHeader(150), row(150, 22, startBox+"Bonjour."))
	f := open(t, w.bytes())

	numbers := make([]int, 0, len(f.Tracks()))
	for _, tr := range f.Tracks() {
		numbers = append(numbers, tr.Number)
	}
	assert.Equal(t, []int{150, 513150, 151}, numbers)
	assert.Equal(t, "und", f.Tracks()[2].Language)
	assert.Equal(t, []cueRow{cue(0, 5, "Bonjour.")}, extract(t, w.bytes(), 513150, model.CommonReadOptions{}))
}

// TestLeniency pins how damage in the packets changes the cues. Each row names the cues of the clean file that it keeps.
func TestLeniency(t *testing.T) {
	t.Parallel()

	clean := fixture(t, "teletext.ts")
	packets := split(clean)
	var continuation int
	for i, p := range packets {
		if pid(p) == teletextPID && p[1]&0x40 == 0 {
			continuation = i
			break
		}
	}
	require.NotZero(t, continuation)

	insert := func(at int, extra []byte) []byte {
		return append(append(bytes.Clone(clean[:at*188]), extra...), clean[at*188:]...)
	}
	jump := func() []byte {
		out := split(clean)
		for _, p := range out[continuation:] {
			if pid(p) == teletextPID {
				p[3] = p[3]&0xF0 | (p[3]+3)&0x0F
			}
		}
		return join(out)
	}
	strayVideo := func() []byte {
		out := split(clean)
		var video int
		for i, p := range out {
			if pid(p) == videoPID {
				video = i
				break
			}
		}
		stray := bytes.Clone(out[video])
		stray[1] &^= 0x40
		return join(slices.Insert(out, video, stray))
	}
	want := extract(t, clean, 888, model.CommonReadOptions{})
	require.Len(t, want, 4)

	tests := []struct {
		name string
		file []byte
	}{
		{"7 junk bytes between packets", insert(3, []byte("\x00\x01\x02\x03\x04\x05\x06"))},
		{"188 junk bytes between packets", insert(5, make([]byte, 188))},
		{"M2TS packets", m2ts(clean)},
		{"a continuity counter jump in a PES that lost no data", jump()},
		{"a video packet that does not start a PES before the video PES", strayVideo()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, extract(t, tt.file, 888, model.CommonReadOptions{}))
		})
	}
}

// pes3 returns a writer with a teletext PES packet of 3 packets between two one-packet pages.
func pes3() (w *tsWriter, first, last int) {
	w = single(eng888())
	w.teletextPES(teletextPID, 90000*11, pageHeader(888), row(888, 22, startBox+"One"), fillingHeader(8))
	first = w.buf.Len() / 188
	rows := [][]byte{pageHeader(888)}
	for r := 1; r <= 10; r++ {
		rows = append(rows, row(888, r, startBox+"Row"+strconv.Itoa(r)))
	}
	w.teletextPES(teletextPID, 90000*12, rows...)
	last = w.buf.Len()/188 - 1
	w.teletextPES(teletextPID, 90000*13, pageHeader(888), fillingHeader(8))
	return w, first, last
}

func TestRepeatedPacket(t *testing.T) {
	t.Parallel()

	w, first, last := pes3()
	require.Equal(t, 2, last-first)
	clean := extract(t, w.bytes(), 888, model.CommonReadOptions{})
	require.Len(t, clean, 2)
	assert.Len(t, clean[1].Lines, 10)

	tests := []struct {
		name  string
		index int
	}{
		{"the first packet of a PES of 3 packets", first},
		{"the middle packet", first + 1},
		{"the last packet", last},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			packets := split(w.bytes())
			file := join(slices.Insert(packets, tt.index, bytes.Clone(packets[tt.index])))
			assert.Equal(t, clean, extract(t, file, 888, model.CommonReadOptions{}))
		})
	}
}

func TestDamagedSyncByte(t *testing.T) {
	t.Parallel()

	clean := fixture(t, "teletext.ts")
	want := extract(t, clean, 888, model.CommonReadOptions{})
	packets := split(clean)
	var first int
	for i, p := range packets {
		if pid(p) == teletextPID {
			first = i
			break
		}
	}
	packets[first][0] = 0x00

	// The first PES of the teletext stream loses its first packet, so the page of 0.5 s is lost and the rest stays.
	assert.Equal(t, want[1:], extract(t, join(packets), 888, model.CommonReadOptions{}))
}

func TestTransportErrorFlag(t *testing.T) {
	t.Parallel()

	build := func(middle func(w *tsWriter), flagged int) []byte {
		w := single(eng888())
		w.teletextPES(teletextPID, 90000*11, pageHeader(888), row(888, 22, startBox+"One"), fillingHeader(8))
		start := w.buf.Len()
		middle(w)
		w.teletextPES(teletextPID, 90000*13, pageHeader(888), fillingHeader(8))
		out := w.bytes()
		if flagged >= 0 {
			out[start+flagged*188+1] |= 0x80
		}
		return out
	}
	onePacket := func(w *tsWriter) {
		w.teletextPES(teletextPID, 90000*12, pageHeader(888), row(888, 22, startBox+"Two"), fillingHeader(8))
	}
	twoPackets := func(w *tsWriter) {
		w.teletextPES(teletextPID, 90000*12, pageHeader(888), row(888, 21, startBox+"More"), row(888, 22, startBox+"Two"), fillingHeader(8))
	}
	tests := []struct {
		name string
		file []byte
		want []cueRow
	}{
		{"no flag, 1 packet", build(onePacket, -1), []cueRow{cue(0, 1, "One"), cue(1, 2, "Two")}},
		{"the flag on a PES of 1 packet drops it", build(onePacket, 0), []cueRow{cue(0, 2, "One")}},
		{"no flag, 2 packets", build(twoPackets, -1), []cueRow{cue(0, 1, "One"), cue(1, 2, "More", "Two")}},
		{"the flag on the second of 2 packets drops the PES", build(twoPackets, 1), []cueRow{cue(0, 2, "One")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, extract(t, tt.file, 888, model.CommonReadOptions{}))
		})
	}
}

func TestWrongPATChecksum(t *testing.T) {
	t.Parallel()

	good := fixture(t, "teletext.ts")
	damaged := bytes.Clone(good)
	damaged[187] ^= 0xFF

	assert.Empty(t, open(t, damaged).Tracks())
	bad := bytes.Clone(damaged[:188])
	bad[3] |= 0x0F // another counter than the valid PAT, which the reader would drop as a repeat
	assert.Len(t, open(t, append(bad, good...)).Tracks(), 5)
}

// nullReaderAt serves a PAT, null packets and the other packets of a file without holding the null packets.
type nullReaderAt struct {
	head, tail []byte
	nulls      int
}

func (n nullReaderAt) size() int64 { return int64(len(n.head)+len(n.tail)) + int64(n.nulls)*188 }

func (n nullReaderAt) ReadAt(p []byte, off int64) (int, error) {
	nullEnd := int64(len(n.head)) + int64(n.nulls)*188
	for i := range p {
		pos := off + int64(i)
		switch {
		case pos >= n.size():
			return i, io.EOF
		case pos < int64(len(n.head)):
			p[i] = n.head[pos]
		case pos < nullEnd:
			p[i] = 0xFF
			if k := (pos - int64(len(n.head))) % 188; k < 4 {
				p[i] = []byte{0x47, 0x1F, 0xFF, 0x10}[k]
			}
		default:
			p[i] = n.tail[pos-nullEnd]
		}
	}
	return len(p), nil
}

func TestTableScanLimit(t *testing.T) {
	t.Parallel()

	packets := split(fixture(t, "one_page.ts"))
	tests := []struct {
		name   string
		nulls  int
		tracks int
	}{
		{"the PMT starts at byte 33,554,428, before the limit", 178480, 1},
		{"the PMT starts at byte 33,554,616, after the limit", 178481, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := nullReaderAt{head: packets[0], tail: join(packets[1:]), nulls: tt.nulls}
			f, err := mpegts.Open(r, r.size())
			require.NoError(t, err)
			assert.Len(t, f.Tracks(), tt.tracks)
		})
	}
}

func timeLimitFile() []byte {
	w := single(eng888())
	pts := int64(0)
	w.teletextPES(teletextPID, pts, pageHeader(888), row(888, 22, startBox+"Start."))
	for range 7700 {
		pts = (pts + 1<<32 - 1) % (1 << 33)
		w.teletextPES(teletextPID, pts, pageHeader(888), fillingHeader(8))
	}
	pts = (pts + 1<<32 - 1) % (1 << 33)
	w.teletextPES(teletextPID, pts, pageHeader(888), row(888, 22, startBox+"End."))
	return w.bytes()
}

type failingReaderAt struct {
	data  []byte
	calls atomic.Int32
	from  int32
}

var errDisk = errors.New("disk gone")

func (f *failingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if f.calls.Add(1) >= f.from {
		return 0, errDisk
	}
	return bytes.NewReader(f.data).ReadAt(p, off)
}

func TestErrors(t *testing.T) {
	t.Parallel()

	arabic := single(ttPage{"ara", subtitlePageType, 888})
	arabic.teletextPES(teletextPID, 90000, header(888, true, 7, true, false), designation(888, 28, 10, 7), row(888, 22, startBox+"ABC"))
	ts := fixture(t, "teletext.ts")
	tests := []struct {
		name string
		call func() error
		want string
		as   func(error) bool
	}{
		{"not TS", func() error {
			_, err := mpegts.Open(strings.NewReader(strings.Repeat("x", 1000)), 1000)
			return err
		}, "The file is not an MPEG-TS file.", isParseError},
		{"a short head", func() error {
			_, err := mpegts.Open(bytes.NewReader(ts[:376]), 376)
			return err
		}, "The file is not an MPEG-TS file.", isParseError},
		{"no track 999", func() error {
			_, _, err := open(t, ts).Extract(999, model.CommonReadOptions{})
			return err
		}, "The file has no subtitle track with the number 999.", func(err error) bool {
			var ia *model.InvalidArgumentError
			return errors.As(err, &ia) && ia.Option == "ReadOptions.Track"
		}},
		{"an Arabic page", func() error {
			_, _, err := open(t, arabic.bytes()).Extract(888, model.CommonReadOptions{})
			return err
		}, "Teletext page 888 uses the Arabic or Hebrew character set. The reader reads Latin, Cyrillic and Greek pages.", isParseError},
		{"the time limit", func() error {
			_, _, err := open(t, timeLimitFile()).Extract(888, model.CommonReadOptions{})
			return err
		}, "The cue times of track 888 reach 367506034.876 s. Times must be below 100000 hours.", isParseError},
		{"a negative size", func() error {
			_, err := mpegts.Open(bytes.NewReader(ts), -1)
			return err
		}, "The file is not an MPEG-TS file.", isParseError},
		{"a ReadAt error in Open", func() error {
			_, err := mpegts.Open(&failingReaderAt{data: ts, from: 1}, int64(len(ts)))
			return err
		}, "read mpegts: disk gone", func(err error) bool { return errors.Is(err, errDisk) }},
		{"a ReadAt error in Extract", func() error {
			f, err := mpegts.Open(&failingReaderAt{data: ts, from: 3}, int64(len(ts)))
			require.NoError(t, err)
			_, _, err = f.Extract(888, model.CommonReadOptions{})
			return err
		}, "read mpegts: disk gone", func(err error) bool { return errors.Is(err, errDisk) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.True(t, tt.as(err), err)
		})
	}
}

func isParseError(err error) bool {
	var pe *model.ParseError
	return errors.As(err, &pe) && pe.Format == "" && pe.Line == 0
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"teletext.ts", "one_page.ts"} {
		ts := fixture(f, name)
		f.Add(ts)
		f.Add(m2ts(ts))
	}
	f.Fuzz(func(_ *testing.T, data []byte) {
		file, err := mpegts.Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		for _, tr := range file.Tracks() {
			_, _, _ = file.Extract(tr.Number, model.CommonReadOptions{})
		}
	})
}

func ExampleOpen() {
	ts, err := os.ReadFile("../../testdata/mpegts/one_page.ts")
	if err != nil {
		fmt.Println(err)
		return
	}
	f, err := mpegts.Open(bytes.NewReader(ts), int64(len(ts)))
	if err != nil {
		fmt.Println(err)
		return
	}
	track := f.Tracks()[0]
	s, _, err := f.Extract(track.Number, model.CommonReadOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(track.Number, track.CodecID, track.Language)
	fmt.Println(s.Cues()[0].Start, s.Cues()[0].Text())
	// Output:
	// 888 dvb_teletext eng
	// 1s The ferry leaves at noon.
}
