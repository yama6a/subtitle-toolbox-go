package arib_test

import (
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/arib"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

// pesPayloads returns the PES data of the PID 304 of a transport stream with packets of size bytes.
func pesPayloads(tb testing.TB, name string, size int) [][]byte {
	tb.Helper()

	ts := testfiles.Read(tb, "mpegts/"+name)
	skip := size - 188
	var out [][]byte
	var cur []byte
	flush := func() {
		if len(cur) >= 9 {
			out = append(out, cur[9+int(cur[8]):])
		}
		cur = nil
	}
	for i := skip; i+188 <= len(ts); i += size {
		p := ts[i : i+188]
		if int(p[1]&0x1F)<<8|int(p[2]) != 304 {
			continue
		}
		data := p[4:]
		if p[3]&0x20 != 0 {
			data = p[5+int(p[4]):]
		}
		if p[1]&0x40 != 0 {
			flush()
		}
		cur = append(cur, data...)
	}
	flush()
	return out
}

func FuzzDecoder(f *testing.F) {
	for _, p := range pesPayloads(f, "arib_japanese.ts", 188) {
		f.Add(p)
	}
	for _, p := range pesPayloads(f, "arib_brazilian.m2ts", 192) {
		f.Add(p)
	}
	f.Add([]byte(statement(text(cs + setup + position(6, 3) + "\x98\x00" + kanji("\u5b57")))))
	f.Fuzz(func(_ *testing.T, data []byte) {
		for _, language := range []string{"jpn", "por", "eng"} {
			d := arib.NewDecoder(language)
			d.Add(data, 0)
			d.Add(data, 90000)
			d.Finish()
			d.Warnings()
			arib.Language(data)
		}
	})
}
