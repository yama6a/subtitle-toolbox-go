package hls_test

import (
	"fmt"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/hls"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func ExampleSegment() {
	var s model.Subtitle
	s.AddCue(model.Cue{Start: time.Second, End: 3 * time.Second, Lines: []string{"The ferry leaves at noon."}})
	s.AddCue(model.Cue{Start: 5 * time.Second, End: 7500 * time.Millisecond, Lines: []string{"Please keep your tickets ready."}})

	r, err := hls.Segment(&s, hls.SegmentOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = r.EachSegment(func(name string, content []byte) error {
		fmt.Printf("%s\n%s\n", name, content)
		return nil
	})
	fmt.Print(string(r.Playlist()))
	// Output:
	// sub0.vtt
	// WEBVTT
	// X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000
	//
	// 1
	// 00:00:01.000 --> 00:00:03.000
	// The ferry leaves at noon.
	//
	// 2
	// 00:00:05.000 --> 00:00:07.500
	// Please keep your tickets ready.
	//
	// sub1.vtt
	// WEBVTT
	// X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000
	//
	// 2
	// 00:00:05.000 --> 00:00:07.500
	// Please keep your tickets ready.
	//
	// #EXTM3U
	// #EXT-X-VERSION:3
	// #EXT-X-TARGETDURATION:6
	// #EXT-X-MEDIA-SEQUENCE:0
	// #EXT-X-PLAYLIST-TYPE:VOD
	// #EXTINF:6.000,
	// sub0.vtt
	// #EXTINF:1.500,
	// sub1.vtt
	// #EXT-X-ENDLIST
}

func ExampleJoin() {
	segments := [][]byte{
		[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000\n\n00:00:04.000 --> 00:00:06.000\nAll aboard.\n"),
		[]byte("WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000\n\n00:00:04.000 --> 00:00:06.000\nAll aboard.\n"),
	}
	s, err := hls.Join(segments, hls.JoinOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, c := range s.All() {
		fmt.Println(c.Start, c.End, c.Text())
	}
	// Output:
	// 4s 6s All aboard.
}

func ExampleParseTimestampMap() {
	m, err := hls.ParseTimestampMap("X-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:00:00:10.000")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(m.Header())
	fmt.Println(m.Offset(0))
	// Output:
	// X-TIMESTAMP-MAP=LOCAL:00:00:10.000,MPEGTS:900000
	// 0s
}
