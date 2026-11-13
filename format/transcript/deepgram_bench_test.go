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

// deepgramBenchCopies is 1 paragraph of 600 members with 1,000 sentences, and 8,000 words. The file has about 0.7 MB.
// The cues hold 11 MB of paragraph copies, near the limit of 16 times the file plus 1 MiB.
func deepgramBenchCopies() string {
	var paragraph, sentences, words strings.Builder
	for i := range 600 {
		fmt.Fprintf(&paragraph, `"k%04d": "0123456", `, i)
	}
	for i := range 1000 {
		if i > 0 {
			sentences.WriteString(",")
		}
		fmt.Fprintf(&sentences, `{"text": "w", "start": %s, "end": %s}`, deepgramBenchTime(i), deepgramBenchTime(i+1))
	}
	for i := range 8000 {
		if i > 0 {
			words.WriteString(",")
		}
		fmt.Fprintf(&words, `{"word": "w%d", "start": %s, "end": %s, "confidence": 0.9, "punctuated_word": "W%d"}`, i, deepgramBenchTime(i/8), deepgramBenchTime(i/8+1), i)
	}
	return fmt.Sprintf(`{"results": {"channels": [{"alternatives": [{"words": [%s], "paragraphs": {"paragraphs": [{%s"sentences": [%s]}]}}]}]}}`,
		words.String(), paragraph.String(), sentences.String())
}

func BenchmarkDeepgramParagraphCopies(b *testing.B) {
	cloudBench(b, deepgramFormat, "1,000 sentences of a paragraph of 600 members", deepgramBenchCopies())
}
