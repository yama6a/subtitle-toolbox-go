# Editing subtitles

## Retiming

A SubRip file made for a 25 fps release plays 4.3 % too fast on a 23.976 fps copy. `s.ConvertFrameRate(25, 23.976)` moves a cue from 100 s to 102.5 s to 104.271 s to 106.878 s.

| Method | Moves |
|:--- |:--- |
| `Shift(d)` | every cue by `d`, which may be negative |
| `ShiftRange(d, from, to)` | each cue with `from <= Start < to` by `d`. A nil bound sets no limit on that side |
| `Scale(factor)` | every time `t` to `t * factor`. The factor is finite and above 0 |
| `ConvertFrameRate(from, to)` | every time by `Scale(from / to)`, with frame rates in frames per second |
| `SyncByPoints(points)` | every time onto the line through the sync points |

- **Word timestamps**: every `<hh:mm:ss.mmm>` tag in the cue lines moves with the same map as the cue times.
- **Clamp**: a time that becomes negative becomes 0. The cue stays, also when it becomes 0 s long.
- **Rounding**: each new time is rounded to whole milliseconds, half away from zero. Scale and sync compute in `float64` milliseconds.
- **Order**: a cue that moves before earlier cues takes its place in start order. Its comments move with it.
- **Time limit**: a cue end must stay below 100,000 hours. The method checks every cue first and changes nothing when one fails.
- **Errors**: every error matches `ErrInvalidArgument`. On an error the subtitle keeps all its times.

### Shift a range

A cue across the range end moves as a whole, because its start decides.

<!-- example: ExampleSubtitle_ShiftRange -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 10 * time.Second, End: 12 * time.Second, Lines: []string{"intro"}},
	model.Cue{Start: 20 * time.Second, End: 22 * time.Second, Lines: []string{"scene"}},
)

_ = s.ShiftRange(1500*time.Millisecond, new(15*time.Second), nil)
// Output:
// 10s 12s intro
// 21.5s 23.5s scene
```

- **Why pointers**: 0 is a legal bound and differs from "no limit". `new(15 * time.Second)` sets a bound in one expression.

### Sync points

A **sync point** is a pair of an old time and a new time. 2 neighbouring points span a segment, which maps a time `t` to `a.New + (t - a.Old) * (b.New - a.New) / (b.Old - a.Old)`. A time uses the last segment that starts at or before it. Times before the first point use the first segment, and times after the last point use the last segment. 1 point shifts every cue by `New - Old`.

<!-- example: ExampleSubtitle_SyncByPoints -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: 10 * time.Second, End: 12 * time.Second, Lines: []string{"first"}},
	model.Cue{Start: 300 * time.Second, End: 302 * time.Second, Lines: []string{"middle"}},
	model.Cue{Start: 1200 * time.Second, End: 1202 * time.Second, Lines: []string{"last"}},
)

_ = s.SyncByPoints([]model.SyncPoint{
	{Old: 10 * time.Second, New: 12 * time.Second},
	{Old: 600 * time.Second, New: 610 * time.Second},
	{Old: 1200 * time.Second, New: 1205 * time.Second},
})
// Output:
// 12s 14.027s first
// 5m5.932s 5m7.959s middle
// 20m5s 20m6.983s last
```

- **Segments**: the cue at 300 s uses the segment from 10 s to 600 s with the factor 598/590. The cue at 1202 s is past the last point and uses the segment from 600 s to 1200 s with the factor 595/600.
- **Cue across a point**: start, end and each word timestamp map on their own, so such a cue gets times from 2 segments.
- **Points**: the list must not be empty, and the old and the new times must both increase strictly.
