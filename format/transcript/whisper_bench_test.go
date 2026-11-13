package transcript_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// The inputs hold about 1 MB. The benchmarks have no time limit, so a rise from linear to quadratic cost shows in
// the numbers of the bench run.
const benchWords = 12000

func benchTime(i int) string { return fmt.Sprintf("%d.%03d", i/100, i%100*10) }

// oneSegment is one segment with benchWords words that the text holds, each with its own start.
func oneSegment() string {
	var text, words strings.Builder
	for i := range benchWords {
		if i > 0 {
			text.WriteString(" ")
			words.WriteString(",")
		}
		fmt.Fprintf(&text, "word%d", i)
		fmt.Fprintf(&words, `{"word": " word%d", "start": %s, "end": %s, "probability": 0.9}`, i, benchTime(i), benchTime(i+1))
	}
	return fmt.Sprintf(`{"segments": [{"start": 0, "end": %s, "text": " %s", "words": [%s]}]}`, benchTime(benchWords), text.String(), words.String())
}

// topLevelWords is the OpenAI API shape: benchWords words at the top level and 2000 segments that take them.
func topLevelWords() string {
	var segments, words strings.Builder
	for i := range 2000 {
		if i > 0 {
			segments.WriteString(",")
		}
		fmt.Fprintf(&segments, `{"id": %d, "start": %s, "end": %s, "text": " text"}`, i, benchTime(i*10), benchTime(i*10+10))
	}
	for i := range benchWords {
		if i > 0 {
			words.WriteString(",")
		}
		fmt.Fprintf(&words, `{"word": "word%d", "start": %s, "end": %s}`, i, benchTime(i), benchTime(i+1))
	}
	return fmt.Sprintf(`{"task": "transcribe", "segments": [%s], "words": [%s]}`, segments.String(), words.String())
}

// oneWordOfTokens is one whisper.cpp entry with 2 * benchWords tokens that join to one word.
func oneWordOfTokens() string {
	var tokens strings.Builder
	for i := range benchWords * 2 {
		if i > 0 {
			tokens.WriteString(",")
		}
		fmt.Fprintf(&tokens, `{"text": "%s", "offsets": {"from": %d, "to": %d}}`, "abcdefghij"[:1+i%9], i, i+1)
	}
	return fmt.Sprintf(`{"transcription": [{"offsets": {"from": 0, "to": %d}, "text": " word", "tokens": [%s]}]}`, benchWords*3, tokens.String())
}

func BenchmarkReadWhisper(b *testing.B) {
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("segment words=%d", benchWords), oneSegment()},
		{fmt.Sprintf("top-level words=%d", benchWords), topLevelWords()},
		{fmt.Sprintf("one word of tokens=%d", benchWords*2), oneWordOfTokens()},
	} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if _, _, err := transcript.ReadWhisper(strings.NewReader(bm.content), model.CommonReadOptions{}, withWords); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWriteWhisper(b *testing.B) {
	for _, bm := range []struct{ name, content string }{
		{fmt.Sprintf("segment words=%d", benchWords), oneSegment()},
		{fmt.Sprintf("top-level words=%d", benchWords), topLevelWords()},
	} {
		s, _, err := transcript.ReadWhisper(strings.NewReader(bm.content), model.CommonReadOptions{}, withWords)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if err := transcript.WriteWhisper(io.Discard, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
