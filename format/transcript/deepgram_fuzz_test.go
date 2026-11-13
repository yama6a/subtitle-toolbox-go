package transcript_test

import "testing"

func FuzzReadDeepgram(f *testing.F) {
	cloudFuzz(f, deepgramFormat,
		`{"results": {"channels": [{"alternatives": [{"words": [{"word": "b", "start": 1, "end": 2}, {"word": "a", "start": 0, "end": 0.5, "speaker": 2}]}]}]}}`,
		`{"results": {"channels": [{"alternatives": [{"words": [], "paragraphs": {"paragraphs": [{"speaker": 1, "sentences": [{"text": "Hi.", "start": 0, "end": 1}, 5]}]}}]}, 3]}}`,
		`{"results": {"channels": [], "utterances": [{"start": 2, "end": 1, "transcript": "x", "speaker": "A", "words": [{"word": "x", "start": 1e12, "end": 1}]}]}}`,
		`{"results": {"channels": [{"alternatives": [{"words": [{"word": "a", "punctuated_word": null, "start": "0", "end": 1, "confidence": 2}]}]}]}}`,
	)
}
