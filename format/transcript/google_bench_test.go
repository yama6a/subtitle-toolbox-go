package transcript_test

import (
	"fmt"
	"strings"
	"testing"
)

// googleBenchWords is the number of words in the benchmark inputs. Each input holds about 1 MB.
const googleBenchWords = 12000

func googleBenchTime(ms int) string { return fmt.Sprintf("%d.%03ds", ms/1000, ms%1000) }

// googleBenchInput returns a response with googleBenchWords words. Diarized puts them in one result with speaker tags.
// Else every 10 words form a result.
func googleBenchInput(diarized bool) string {
	var words, text strings.Builder
	var results []string
	flush := func() {
		results = append(results, fmt.Sprintf(`{"alternatives": [{"transcript": "%s", "words": [%s]}]}`, text.String(), words.String()))
		words.Reset()
		text.Reset()
	}
	for i := range googleBenchWords {
		if words.Len() > 0 {
			words.WriteString(",")
			text.WriteString(" ")
		}
		speaker := ""
		if diarized {
			speaker = `, "speakerTag": 1`
		}
		fmt.Fprintf(&words, `{"startTime": "%s", "endTime": "%s", "word": "word%d"%s}`, googleBenchTime(i*10), googleBenchTime(i*10+10), i, speaker)
		fmt.Fprintf(&text, "word%d", i)
		if !diarized && i%10 == 9 {
			flush()
		}
	}
	if diarized {
		flush()
	}
	return `{"results": [` + strings.Join(results, ",") + `]}`
}

func BenchmarkGoogleSpeech(b *testing.B) {
	cloudBench(b, googleFormat, fmt.Sprintf("results of 10 words, words=%d", googleBenchWords), googleBenchInput(false))
	cloudBench(b, googleFormat, fmt.Sprintf("one diarized result, words=%d", googleBenchWords), googleBenchInput(true))
}
