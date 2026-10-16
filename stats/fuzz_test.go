package stats_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
	"github.com/yama6a/subtitle-toolbox-go/stats"
)

func FuzzOf(f *testing.F) {
	for _, dir := range []string{"statistics", "srt/real", "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".srt" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"1\n00:00:01,000 --> 00:00:01,000\nRain\n\n2\n00:00:02,000 --> 00:00:01,000\nSun today",
		"1\n00:00:01,000 --> 00:00:03,000\nCaf\xe9 au lait \xff\xfe &amp; R&B",
		"1\n00:00:01,000 --> 00:00:03,000\n<i> </i>\n\n2\n00:00:00,000 --> 00:00:09,000\n\u00a0\u3000 x",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: true}, srt.ReadOptions{})
		if err != nil {
			t.Skip()
		}

		got := stats.Of(s)
		out, err := got.MarshalJSON()

		require.NoError(t, err)
		require.True(t, json.Valid(out), "%s", out)
		require.Equal(t, s.Len(), got.CueCount)
	})
}
