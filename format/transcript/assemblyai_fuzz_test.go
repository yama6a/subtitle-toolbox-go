package transcript_test

import "testing"

func FuzzReadAssemblyAI(f *testing.F) {
	cloudFuzz(f, assemblyAIFormat,
		assemblyAIWords([]any{"b", 1000, 2000}, []any{"a", 0, 500, "A"}),
		`{"words": [], "utterances": [{"start": 2000, "end": 1000, "text": "x", "speaker": 5, "words": [{"text": "x", "start": 1e15, "end": 1}]}]}`,
		`{"language_code": "en_us", "words": [{"text": "a", "start": "0", "end": 1, "confidence": 3, "speaker": true}], "utterances": null}`,
		`{"utterances": [5, {"start": 0, "end": 1, "text": "a", "words": [null]}]}`,
	)
}
