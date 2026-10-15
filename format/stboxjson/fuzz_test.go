package stboxjson_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const infOrNaN = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"../../testdata/json", realDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				content, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					f.Fatal(err)
				}
				f.Add(content)
			}
		}
	}
	for _, seed := range []string{
		`{"version": 1, "cues": [{"start": 5, "end": 3, "lines": ["a"]}, {"start": 1e20, "end": 2, "lines": [7]}]}`,
		`{"version": 1, "metadata": ["a"], "formatData": {"x": [{"base64": "AAE="}], "y": {"a": 1e400}}, "cues": []}`,
		`{"version": 1, "comments": [{"text": "a", "beforeCueIndex": 3}], "cues": [{"start": 2, "end": 3, "lines": []}, {"start": 1, "end": 2, "lines": ["<00:00:01.500>b"]}]}`,
		`[]`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			common := model.CommonReadOptions{Lenient: lenient}
			s, warnings, err := stboxjson.Read(bytes.NewReader(content), common, stboxjson.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			checkCues(t, s)
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}

			var out bytes.Buffer
			err = stboxjson.Write(&out, s, model.CommonWriteOptions{}, stboxjson.WriteOptions{})
			if err != nil {
				var uerr *model.UnwritableError
				if !errors.As(err, &uerr) || uerr.Reason != infOrNaN {
					t.Fatalf("Write returned %v", err)
				}
				continue
			}
			again, _, err := stboxjson.Read(bytes.NewReader(out.Bytes()), common, stboxjson.ReadOptions{})
			if err != nil {
				t.Fatalf("the output %s does not read: %v", out.Bytes(), err)
			}
			if !reflect.DeepEqual(s, again) {
				t.Fatalf("the output %s reads to another subtitle", out.Bytes())
			}
		}
	})
}

func checkCues(t *testing.T, s *model.Subtitle) {
	t.Helper()

	cues := s.Cues()
	for i, c := range cues {
		if i > 0 && c.Start < cues[i-1].Start {
			t.Fatalf("cue %d starts before cue %d", i, i-1)
		}
		for _, d := range []int64{int64(c.Start), int64(c.End)} {
			if d%1e6 != 0 || d >= int64(timecode.MaxTime) || d <= -int64(timecode.MaxTime) {
				t.Fatalf("cue %d has the time %d ns", i, d)
			}
		}
	}
}
