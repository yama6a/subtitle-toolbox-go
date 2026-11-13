package mcc_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/mcc"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const benchCues = 20000

func benchSubtitle() *model.Subtitle {
	s := &model.Subtitle{}
	for i := range benchCues {
		start := time.Duration(i) * 2 * time.Second
		s.AddCue(model.Cue{Start: start, End: start + 1500*time.Millisecond, Lines: []string{fmt.Sprintf("Cue number %d", i), "<i>on two lines</i>"}})
	}
	return s
}

func BenchmarkRead(b *testing.B) {
	var file bytes.Buffer
	if _, err := mcc.Write(&file, benchSubtitle(), model.CommonWriteOptions{}, mcc.WriteOptions{}); err != nil {
		b.Fatal(err)
	}
	content := file.Bytes()
	b.Run(fmt.Sprintf("cues=%d", benchCues), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, _, err := mcc.Read(bytes.NewReader(content), model.CommonReadOptions{}, mcc.ReadOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkWrite(b *testing.B) {
	s := benchSubtitle()
	b.Run(fmt.Sprintf("cues=%d", benchCues), func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := mcc.Write(io.Discard, s, model.CommonWriteOptions{}, mcc.WriteOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}
