package transcript_test

import "testing"

func FuzzReadGoogleSpeech(f *testing.F) {
	cloudFuzz(f, googleFormat,
		`{"results": [{"alternatives": [{"transcript": "a", "words": [{"startTime": "0s", "endTime": "1s", "word": "a", "speakerTag": 2}]}]}]}`,
		`{"name": "x", "done": true, "response": {"results": [{"resultEndTime": {"seconds": "9", "nanos": 5}}, {"alternatives": [5]}]}}`,
		`{"results": [{"alternatives": [{"words": [{"startOffset": "1e3s", "endOffset": "-1s", "word": 5, "speakerLabel": "A"}]}], "resultEndOffset": "99999999999999999999s"}]}`,
		`{"results": [{"alternatives": [{"words": [{"startTime": 1, "endTime": "2.5", "word": "x", "speakerTag": "0"}]}]}]}`,
	)
}
