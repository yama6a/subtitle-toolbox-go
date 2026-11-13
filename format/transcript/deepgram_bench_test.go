package transcript_test

import (
	"fmt"
	"strings"
	"testing"
)

// deepgramBenchWords is the number of words in the benchmark inputs. Each input holds about 1 MB.
const deepgramBenchWords = 9000

func deepgramBenchTime(i int) string { return fmt.Sprintf("%d.%02d", i/100, i%100) }

func deepgramBenchList() (words, text string) {
	var w, t strings.Builder
	for i := range deepgramBenchWords {
		if i > 0 {
			w.WriteString(",")
			t.WriteString(" ")
		}
		fmt.Fprintf(&w, `{"word": "word%d", "start": %s, "end": %s, "confidence": 0.91, "speaker": 0, "punctuated_word": "Word%d"}`,
			i, deepgramBenchTime(i), deepgramBenchTime(i+1), i)
		fmt.Fprintf(&t, "Word%d", i)
	}
	return w.String(), t.String()
}

// deepgramBenchInput returns a response of one channel with deepgramBenchWords words. Words only gives cues grouped
// from the words. Utterance puts every word in one utterance. Paragraph puts one sentence of 1 word each in one paragraph.
func deepgramBenchInput(kind string) string {
	words, text := deepgramBenchList()
	switch kind {
	case "utterance":
		return fmt.Sprintf(`{"results": {"channels": [{"alternatives": [{"words": []}]}], "utterances": [{"start": 0, "end": %d, "transcript": "%s", "words": [%s]}]}}`,
			deepgramBenchWords/100+1, text, words)
	case "paragraph":
		var sentences strings.Builder
		for i := range deepgramBenchWords {
			if i > 0 {
				sentences.WriteString(",")
			}
			fmt.Fprintf(&sentences, `{"text": "Word%d", "start": %s, "end": %s}`, i, deepgramBenchTime(i), deepgramBenchTime(i+1))
		}
		return fmt.Sprintf(`{"results": {"channels": [{"alternatives": [{"words": [%s], "paragraphs": {"paragraphs": [{"speaker": 0, "sentences": [%s]}]}}]}]}}`,
			words, sentences.String())
	}
	return fmt.Sprintf(`{"results": {"channels": [{"alternatives": [{"words": [%s]}]}]}}`, words)
}

func BenchmarkDeepgram(b *testing.B) {
	for _, kind := range []string{"words only", "utterance", "paragraph"} {
		cloudBench(b, deepgramFormat, fmt.Sprintf("%s words=%d", kind, deepgramBenchWords), deepgramBenchInput(strings.Fields(kind)[0]))
	}
}
