package hls_test

import (
	"errors"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v3/hls"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func FuzzParseTimestampMap(f *testing.F) {
	for _, s := range []string{
		"X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000",
		"X-TIMESTAMP-MAP=LOCAL:01:00:00.000,MPEGTS:324000000",
		" X-TIMESTAMP-MAP=MPEGTS:181083, LOCAL:01:02.345 ",
		"X-TIMESTAMP-MAP=LOCAL:100:00:00.500,MPEGTS:0",
		"X-TIMESTAMP-MAP=mpegts:99999999999999999999,local:00:00.000",
		"X-TIMESTAMP-MAP=LOCAL:00:00.000",
		"X-TIMESTAMP-MAP=,,:",
		"WEBVTT",
		"",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, line string) {
		m, err := hls.ParseTimestampMap(line)
		if err != nil {
			var perr *model.ParseError
			if !errors.Is(err, model.ErrParse) || !errors.As(err, &perr) || perr.Format != model.WebVTT {
				t.Fatalf("ParseTimestampMap(%q) returned %v", line, err)
			}
			return
		}
		if m.MPEGTS < 0 || m.Local < 0 || m.Local >= timecode.MaxTime || m.Local%1_000_000 != 0 {
			t.Fatalf("ParseTimestampMap(%q) returned %+v", line, m)
		}
		if m.Validate() != nil {
			return
		}
		back, err := hls.ParseTimestampMap(m.Header())
		if err != nil || back != m {
			t.Fatalf("ParseTimestampMap(%q) gave %+v, and its header %q gave %+v with %v", line, m, m.Header(), back, err)
		}
	})
}
