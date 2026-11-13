package transcript_test

import (
	"fmt"
	"strings"
	"testing"
)

// awsBenchItems is the number of items in the benchmark inputs. Each input holds about 1 MB.
const awsBenchItems = 10000

func awsBenchItem(i int) string {
	return fmt.Sprintf(`{"id": %d, "start_time": "%d.%02d", "end_time": "%d.%02d", "alternatives": [{"confidence": "0.91", "content": "word%d"}], "type": "pronunciation"}`,
		i, i/100, i%100, (i+1)/100, (i+1)%100, i)
}

// awsBenchInput returns a job of awsBenchItems words. With segments, one audio segment holds every item id.
func awsBenchInput(segments bool) string {
	var items, ids, text strings.Builder
	for i := range awsBenchItems {
		if i > 0 {
			items.WriteString(",")
			ids.WriteString(",")
			text.WriteString(" ")
		}
		items.WriteString(awsBenchItem(i))
		fmt.Fprintf(&ids, "%d", i)
		fmt.Fprintf(&text, "word%d", i)
	}
	if !segments {
		return `{"results": {"items": [` + items.String() + `]}}`
	}
	return fmt.Sprintf(`{"results": {"items": [%s], "audio_segments": [{"start_time": "0", "end_time": "%d", "transcript": "%s", "items": [%s]}]}}`,
		items.String(), awsBenchItems/100+1, text.String(), ids.String())
}

// awsBenchPunctuation is one word followed by awsBenchItems * 2 punctuation items, which the reader joins to it.
func awsBenchPunctuation() string {
	var items strings.Builder
	items.WriteString(awsBenchItem(0))
	for range awsBenchItems * 2 {
		items.WriteString(`,{"type": "punctuation", "alternatives": [{"confidence": "0.0", "content": "."}]}`)
	}
	return `{"results": {"items": [` + items.String() + `]}}`
}

// awsBenchRepeated is one word with awsBenchItems/2 punctuation items and awsBenchItems/2 audio segments that each
// list the id of that word. Each segment names the same word.
func awsBenchRepeated() string {
	n := awsBenchItems / 2
	var items, segments strings.Builder
	items.WriteString(awsBenchItem(0))
	for range n {
		items.WriteString(`,{"type": "punctuation", "alternatives": [{"confidence": "0.0", "content": "."}]}`)
	}
	for i := range n {
		if i > 0 {
			segments.WriteString(",")
		}
		segments.WriteString(`{"start_time": "0", "end_time": "1", "transcript": "word", "items": [0]}`)
	}
	return `{"results": {"items": [` + items.String() + `], "audio_segments": [` + segments.String() + `]}}`
}

func BenchmarkAWSTranscribe(b *testing.B) {
	cloudBench(b, awsFormat, fmt.Sprintf("items=%d", awsBenchItems), awsBenchInput(false))
	cloudBench(b, awsFormat, fmt.Sprintf("one segment of items=%d", awsBenchItems), awsBenchInput(true))
	cloudBench(b, awsFormat, fmt.Sprintf("one word of punctuation=%d", awsBenchItems*2), awsBenchPunctuation())
	cloudBench(b, awsFormat, fmt.Sprintf("segments=%d that name one word", awsBenchItems/2), awsBenchRepeated())
}
