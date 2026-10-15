# Editing subtitles

## Retiming

A SubRip file made for a 25 fps release plays 4.3 % too fast on a 23.976 fps copy. With `s.ConvertFrameRate(25, 23.976)`, the cue from 100 s to 102.5 s becomes 104.271 s to 106.878 s.

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
- **Time limit**: a cue end must stay below 100,000 hours after rounding. The method checks every cue first and changes nothing when one fails.
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

if err := s.ShiftRange(1500*time.Millisecond, new(15*time.Second), nil); err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
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

err := s.SyncByPoints([]model.SyncPoint{
	{Old: 10 * time.Second, New: 12 * time.Second},
	{Old: 600 * time.Second, New: 610 * time.Second},
	{Old: 1200 * time.Second, New: 1205 * time.Second},
})
if err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
// Output:
// 12s 14.027s first
// 5m5.932s 5m7.959s middle
// 20m5s 20m6.983s last
```

- **Segments**: the cue at 300 s uses the segment from 10 s to 600 s with the factor 598/590. The cue at 1202 s is past the last point and uses the segment from 600 s to 1200 s with the factor 595/600.
- **Cue across a point**: start, end and each word timestamp map on their own, so such a cue gets times from 2 segments.
- **Points**: the list must not be empty, and the old and the new times must both increase strictly.

## Cue structure

A film comes as `film_part1.srt` and `film_part2.srt`. `part1.Append(part2, 3130*time.Second)` adds the cues of part 2 at 52:10. HLS WebVTT repeats a cue across a segment boundary, and `RemoveDuplicateCues(0)` joins the copies into one cue.

| Method | Does | Result |
|:--- |:--- |:--- |
| `Append(other, offset)` | adds copies of the cues of `other`, moved by `offset` | changes `s` |
| `Slice(from, to, moveToZero)` | keeps the cues that overlap `from` to `to`, cut at both bounds | a new subtitle |
| `ForcedCuesOnly()` | keeps the forced cues | a new subtitle |
| `SplitCue(index, at, splitAfterLine)` | splits one cue in 2 at a time and a line | changes `s` |
| `JoinCues(first, last)` | joins the cues `first` to `last` into one cue | changes `s` |
| `RemoveDuplicateCues(maxGap)` | joins neighbouring cues with the same text | changes `s` |
| `MergeSameTimeCues(timeTolerance)` | joins neighbouring cues with the same times | changes `s` |

- **Cue indexes**: 0-based. The CLI prints 1-based numbers.
- **Errors**: an index without a cue returns an error that matches `ErrCueNotFound`. Every other bad argument returns an error that matches `ErrInvalidArgument`. On an error the subtitle stays unchanged.
- **Comments**: a comment stays before its cue. A comment before a joined cue moves before the cue it joined into. A comment before a split cue stays before the first part.
- **Copies**: `Slice` and `ForcedCuesOnly` keep the metadata, the file format data, and each comment whose cue stays. A comment after the last cue stays only when the last cue stays.

### Append

<!-- example: ExampleSubtitle_Append -->
```go
var part1 model.Subtitle
part1.AddCue(model.Cue{Start: 4200 * time.Millisecond, End: 7450 * time.Millisecond, Lines: []string{"The ferry leaves at seven."}})
_ = part1.AddComment("End of part one", 1)

var part2 model.Subtitle
part2.AddCue(model.Cue{Start: 1250 * time.Millisecond, End: 4 * time.Second, Lines: []string{"PART TWO"}})

if err := part1.Append(&part2, 3130*time.Second); err != nil {
	fmt.Println(err)
	return
}

for _, c := range part1.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
fmt.Println(part1.Comments())
// Output:
// 4.2s 7.45s The ferry leaves at seven.
// 52m11.25s 52m14s PART TWO
// [{End of part one 1}]
```

- **Times**: the start, the end and each word timestamp of a copied cue move by `offset`, as `Shift` does. The time limit of retiming applies.
- **Part boundary**: a comment of `s` after its last cue comes before the first copied cue, so it stays between the 2 parts.
- **Conflicts**: per metadata key and per file format data key, the value of `s` wins.
- **Order**: all cues take their place in start order. At the same place, the comments of `s` come before the comments of `other`.

### Slice

Cues from 1 s to 3 s, 4 s to 6 s and 7 s to 9 s give 2 s to 3 s, 4 s to 6 s and 7 s to 8 s with `Slice(2*time.Second, 8*time.Second, false)`.

- **Touching cues**: a cue drops when `End <= from` or `Start >= to`.
- **Empty range**: `from == to` is legal. Only a cue across `from` stays, with a length of 0.
- **moveToZero**: moves the times and the word timestamps back by `from`, as `Shift(-from)` does. Without it, word timestamps stay, also when they fall outside the cut cue.

### Split and join

- **Split**: `SplitCue(2, 9500*time.Millisecond, 1)` of a cue from 6.5 s to 12.5 s with 2 lines gives one cue from 6.5 s to 9.5 s with line 1 and one from 9.5 s to 12.5 s with line 2. `splitAfterLine` counts from 1, so a cue of 1 line cannot split.
- **Second part**: copies the alignment, the forced flag and the format data, and has no ID. It takes its place in start order after the first part. Word timestamps do not move.
- **Join**: the joined cue holds the lines of all cues in order and ends at the latest end. The start, the ID, the alignment, the forced flag and the format data come from cue `first`. Join checks no speaker, alignment or forced flag.

### Duplicates

`RemoveDuplicateCues` joins `Hello` from 1 s to 2 s and `Hello` from 2 s to 3 s into `Hello` from 1 s to 3 s.

A cue continues the run of the cue before it when all of these hold, compared with the first cue of the run:

- the same `Text()`, tags and case included
- the same alignment, forced flag and set of `<v>` speaker names
- the same format data, with the same keys, values, value types and key order. An ASS event with style `Glow` and one with `Default` stay apart
- a gap of at most `maxGap` to the run, rounded to whole milliseconds

The run joins into its first cue, which keeps its lines and ends at the latest end of the run.

### Same-time cues

`MergeSameTimeCues(0)` joins `- Where are you going?` and `- Home.`, both from 1 s to 3 s, into one cue with both lines.

- **Group**: a cue joins the group when its start and its end both differ by at most `timeTolerance` from the first cue of the group, and it has the alignment and the forced flag of that cue. Format data and speakers do not count, so ASS events with different styles join.
- **Lines**: the lines of each cue in order. A cue whose `Text()` came before in the group adds no lines.
- **Result**: the start, the ID and the format data of the first cue, and the latest end of the group. No dialogue dash is added.
