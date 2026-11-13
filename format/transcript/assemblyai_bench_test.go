package transcript_test

import (
	"fmt"
	"strings"
	"testing"
)

// assemblyAIBenchWords is the number of words in the benchmark inputs. Each input holds about 1 MB.
const assemblyAIBenchWords = 12000

// assemblyAIBenchInput returns a transcript of assemblyAIBenchWords words. With utterances, one utterance holds them all.
func assemblyAIBenchInput(utterances bool) string {
	var words, text strings.Builder
	for i := range assemblyAIBenchWords {
		if i > 0 {
			words.WriteString(",")
			text.WriteString(" ")
		}
		fmt.Fprintf(&words, `{"text": "word%d", "start": %d, "end": %d, "confidence": 0.91, "speaker": "A"}`, i, i*10, i*10+10)
		fmt.Fprintf(&text, "word%d", i)
	}
	if !utterances {
		return `{"words": [` + words.String() + `]}`
	}
	return fmt.Sprintf(`{"words": [], "utterances": [{"start": 0, "end": %d, "text": "%s", "speaker": "A", "words": [%s]}]}`,
		assemblyAIBenchWords*10, text.String(), words.String())
}

func BenchmarkAssemblyAI(b *testing.B) {
	cloudBench(b, assemblyAIFormat, fmt.Sprintf("words=%d", assemblyAIBenchWords), assemblyAIBenchInput(false))
	cloudBench(b, assemblyAIFormat, fmt.Sprintf("one utterance of words=%d", assemblyAIBenchWords), assemblyAIBenchInput(true))
}
