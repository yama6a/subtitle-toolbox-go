package transcript_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const podcastBenchSegments = 16000

// oneWordSegments is podcastBenchSegments one-word segments of one speaker that join into one cue. The input is
// about 1 MB.
func oneWordSegments() string {
	var b strings.Builder
	b.WriteString(`{"version": "1.0.0", "segments": [`)
	for i := range podcastBenchSegments {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"speaker": "Anna", "startTime": %s, "endTime": %s, "body": "word%d", "confidence": 0.9}`, benchTime(i*2), benchTime(i*2+1), i)
	}
	b.WriteString("]}")
	return b.String()
}

// fallingStarts is podcastBenchSegments segments without an end whose starts fall, so no segment has a later start
// next to it. It is the worst input of the backward pass.
func fallingStarts() string {
	var b strings.Builder
	b.WriteString(`{"segments": [`)
	for i := range podcastBenchSegments {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"startTime": %s, "body": "two words %d"}`, benchTime(podcastBenchSegments*2-i), i)
	}
	b.WriteString("]}")
	return b.String()
}

func BenchmarkReadPodcast(b *testing.B) {
	for _, bm := range []struct {
		name, content string
		opts          transcript.ReadOptions
	}{
		{fmt.Sprintf("joined segments=%d", podcastBenchSegments), oneWordSegments(), withWords},
		{fmt.Sprintf("kept segments=%d", podcastBenchSegments), oneWordSegments(), transcript.ReadOptions{KeepSegments: true}},
		{fmt.Sprintf("falling starts segments=%d", podcastBenchSegments), fallingStarts(), transcript.ReadOptions{}},
	} {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bm.content)))
			for b.Loop() {
				if _, _, err := transcript.ReadPodcast(strings.NewReader(bm.content), model.CommonReadOptions{}, bm.opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWritePodcast(b *testing.B) {
	s, _, err := transcript.ReadPodcast(strings.NewReader(oneWordSegments()), model.CommonReadOptions{}, withWords)
	if err != nil {
		b.Fatal(err)
	}
	for _, opts := range []transcript.PodcastWriteOptions{{}, {WordSegments: true}} {
		b.Run(fmt.Sprintf("words=%d word segments=%t", podcastBenchSegments, opts.WordSegments), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := transcript.WritePodcast(io.Discard, s, model.CommonWriteOptions{}, transcript.WriteOptions{Podcast: opts}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
