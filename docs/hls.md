# HLS subtitles

HTTP Live Streaming (HLS) delivers a subtitle track as WebVTT segment files that an `.m3u8` media playlist lists. Package `hls` cuts a subtitle into such segments and joins downloaded segments into one subtitle. Example: `stbox hls film.srt --output-dir subs --segment 6` writes `subs/sub0.vtt`, `subs/sub1.vtt` and more, and `subs/subs.m3u8` with one `#EXTINF:6.000,` line per segment.

## Timestamp map

Each segment starts with a header `X-TIMESTAMP-MAP=LOCAL:00:00:00.000,MPEGTS:900000`. It says that the WebVTT cue time `LOCAL` plays at the timestamp `MPEGTS` of the video. `MPEGTS` counts ticks of the 90 kHz MPEG-2 clock, so 900000 is 10 s. The value has 33 bits, from 0 to 8589934591. See [RFC 8216 section 3.5](https://datatracker.ietf.org/doc/html/rfc8216#section-3.5).

| Call | Result |
|:--- |:--- |
| `hls.ParseTimestampMap(line)` | the map of a header line. `LOCAL` and `MPEGTS` may come in either order. It returns a `*model.ParseError` for another line, and it does not check the range of `MPEGTS` |
| `hls.TimestampMapOf(s)` | the map of the first `X-TIMESTAMP-MAP` line in the WebVTT header lines of a parsed subtitle, or `false` when it has none |
| `m.Validate()` | an error that matches `model.ErrInvalidArgument` when `MPEGTS` is outside 33 bits or `Local` is negative |
| `m.Header()` | the header line, with `LOCAL` first as the RFC writes it |
| `m.Offset(streamStart)` | the time to add to a cue time to count it from the timestamp `streamStart` |

`Offset` folds a difference of 2^32 ticks or more into the range of a wrapped 33-bit timestamp. So a segment with `MPEGTS` 90000 and a stream start of 8589844592 gets an offset of 2 s.

## Segment a subtitle

`hls.Segment(s, opts)` returns a `*hls.Rendition`. It writes no segment yet. `EachSegment` writes one segment at a time and calls a function with its file name and content, so the memory does not grow with the media duration. Each call writes the segments again.

<!-- example: hls/ExampleSegment -->
```go
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
```

| Field of `SegmentOptions` | Default | Meaning |
|:--- |:--- |:--- |
| `SegmentDuration` | 6 s | duration of each segment, rounded to whole milliseconds. At least 1 ms |
| `Map` | MPEGTS 900000, LOCAL 0 | the map of each header. It is a pointer, because MPEGTS 0 is legal |
| `FileNamePattern` | `sub%d.vtt` | file name of segment n, with n from 0. It holds exactly one verb such as `%d` or `%03d` |
| `MediaDuration` | the end of the last cue | duration of the video, so the playlist covers all of it |

- **Cues**: segment i covers the milliseconds from `i x d` to `(i+1) x d`, and the last segment stops at the total duration. A segment holds every cue that overlaps it, with its full times, so a cue that crosses a boundary is in both segments. A cue of zero length goes into the segment where it starts.
- **Identifiers**: a cue without an identifier gets its 1-based index in the subtitle, so it has the same identifier in every segment.
- **File data**: each segment keeps the header text, the `STYLE` blocks and the `REGION` blocks of the subtitle, and it replaces an old `X-TIMESTAMP-MAP` line with the new map. It does not keep comments.
- **Local time**: the cue times in the segments are shifted by `Map.Local`, so a `LOCAL` of 10 s with a `MPEGTS` of 126000 writes the cue that starts at 1.8 s as `00:00:11.800`.
- **Total duration**: without `MediaDuration`, the total is the largest cue end. A subtitle without a cue and without `MediaDuration` fails with an error that matches `model.ErrInvalidArgument`.
- **Playlist**: `Playlist()` returns a VOD media playlist with `#EXT-X-VERSION:3`. `#EXT-X-TARGETDURATION` is the largest segment duration in seconds, rounded to the nearest integer and at least 1. RFC 8216 section 4.3.3.1 asks for this, so each `#EXTINF` value rounded to the nearest integer does not exceed it. `Durations()` returns the file name and duration of each segment, and `Count()` the number of segments.

## Join segments

`hls.Join(segments, hls.JoinOptions{})` reads the segments in playlist order and returns one subtitle. The caller passes the content of each segment. The package fetches nothing and reads no playlist.

<!-- example: hls/ExampleJoin -->
```go
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
```

- **Stream start**: cue times count from the `MPEGTS` of the first segment. `JoinOptions.StreamStart` sets another start. It must be a 33-bit value.
- **No map**: a segment without a header counts as `MPEGTS` 0 and `LOCAL` 0, as RFC 8216 section 3.5 says.
- **Negative times**: a cue that lands before the stream start starts at 0.
- **Repeats**: a cue with the same start, end and text as an earlier cue is dropped. Then `RemoveDuplicateCues(0)` joins cues with the same text that overlap or touch, so a cue that `Segment` split at a boundary is whole again.
- **File data**: the result has the file data of the first segment without its `X-TIMESTAMP-MAP` lines.
- **Errors**: a segment that the strict WebVTT reader rejects, a broken map and an `MPEGTS` of 2^33 or more fail the call. The message names the segment, counted from 0.

## Command line

```
stbox hls film.srt --output-dir subs --segment 6
```

The command reads one input and writes the segments and the playlist into `--output-dir`. See [cli.md](cli.md#hls).
