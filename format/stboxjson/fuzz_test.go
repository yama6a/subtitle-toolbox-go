package stboxjson_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const infOrNaN = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"json", realDir, "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".json" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, c := range readErrorCases() {
		f.Add([]byte(c.content))
	}
	for _, c := range numberCases() {
		f.Add([]byte(c.content()))
	}
	for _, c := range warningCases() {
		f.Add([]byte(c.content))
	}
	for _, seed := range []string{
		`{"version": 1, "metadata": ["a"], "formatData": {"x": [{"base64": "AAE="}], "y": {"a": 1e400}}, "cues": []}`,
		`{"version": 1, "comments": [{"text": "a", "beforeCueIndex": 3}], "cues": [{"start": 2, "end": 3, "lines": []}, {"start": 1, "end": 2, "lines": ["<00:00:01.500>b"]}]}`,
		`[]`,
		`{"version": 2, "cues": [{"start": 1, "end": 4, "lines": ["The beach was empty."], "speaker": "Anna", "words": [{"text": "The", "start": 1, "end": 1.2}, {"text": "beach", "start": 1.2, "end": 2, "confidence": 0.93}]}]}`,
		`{"version": 2, "cues": [{"start": 1, "end": 2, "lines": ["a"], "words": [{"text": "b", "start": 1, "end": 2}]}, {"start": 1, "end": 2, "lines": ["a"], "speaker": 7, "words": {}}]}`,
		`{"version": 2, "cues": [{"start": 1, "end": 2, "lines": ["a a"], "words": [{"text": "a", "start": 2, "end": 1}, {"text": "", "start": 1, "end": 1}, {"text": "a", "start": 0, "end": 1, "confidence": 2}]}]}`,
		`{"version": 2, "formatData": {"x": {"a": 1}}, "cues": [{"start": 1, "end": 2, "lines": ["a"], "formatData": {"vtt": {"voiceClass": "loud", "voiceEnd": true}}}]}`,
		`{"version": 3, "cues": []}`,
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

			checkWrite(t, s, common, 1)
			checkWrite(t, s, common, 2)
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

// checkWrite writes s as the given version and reads it back. A second write gives the same bytes, and a second read
// gives the same subtitle.
func checkWrite(t *testing.T, s *model.Subtitle, common model.CommonReadOptions, version int) {
	t.Helper()

	opts := stboxjson.WriteOptions{Version: version}
	var out bytes.Buffer
	_, err := stboxjson.Write(&out, s, model.CommonWriteOptions{}, opts)
	if err != nil {
		var uerr *model.UnwritableError
		if !errors.As(err, &uerr) || uerr.Reason != infOrNaN {
			t.Fatalf("Write of version %d returned %v", version, err)
		}
		return
	}
	again, _, err := stboxjson.Read(bytes.NewReader(out.Bytes()), common, stboxjson.ReadOptions{})
	if err != nil {
		t.Fatalf("the output %s does not read: %v", out.Bytes(), err)
	}
	var second bytes.Buffer
	if _, err := stboxjson.Write(&second, again, model.CommonWriteOptions{}, opts); err != nil || second.String() != out.String() {
		t.Fatalf("the output %s changes when it is read and written again: %v", out.Bytes(), err)
	}
	third, _, err := stboxjson.Read(bytes.NewReader(second.Bytes()), common, stboxjson.ReadOptions{})
	if err != nil || !reflect.DeepEqual(again, third) {
		t.Fatalf("the output %s reads to another subtitle: %v", out.Bytes(), err)
	}
}
