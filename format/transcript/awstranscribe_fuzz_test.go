package transcript_test

import "testing"

func FuzzReadAWSTranscribe(f *testing.F) {
	cloudFuzz(f, awsFormat,
		awsHelloWorld,
		`{"results": {"items": [{"type": "punctuation", "alternatives": [{"content": "x"}]}, `+awsItem("1", "1", "2", "a")+`], "audio_segments": [`+
			`{"start_time": "1", "end_time": "2", "transcript": "a", "items": [1, "1", 2.5, null]}]}}`,
		`{"results": {"items": [`+awsItem("1", "5", "1", "a")+`,`+awsItem("2", "1e12", "1", "b")+`], "speaker_labels": {"segments": []}}}`,
		`{"results": {"items": [{"start_time": 1, "end_time": 2, "speaker_label": 7, "alternatives": [{"content": "a", "confidence": "2"}]}]}}`,
	)
}
