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

## Timing fixes

Speech-to-text tools and old editors write cues that overlap, flash for 0.3 s, or start and end exactly on the speech. A SubRip cue ends at 5.2 s and the next starts at 5.1 s. `s.FixOverlaps(time.Second * 2 / 24)` ends the first cue at 5.017 s, 2 frames at 24 fps before the next cue.

| Method | Moves | Does |
|:--- |:--- |:--- |
| `FixOverlaps(minGap)` | ends | ends each cue at least `minGap` before the next later start |
| `ExtendShortCues(minDuration, minGap)` | ends | makes each cue last at least `minDuration` where the next cue allows it |
| `AddLeadInOut(leadIn, leadOut, minGap)` | starts and ends | shows each cue earlier and longer where the neighbours allow it |
| `LimitLongCues(maxDuration)` | ends | ends each cue at most `maxDuration` after its start |
| `MergeShortCues(opts)` | cues | joins each short cue with the next or the previous cue when the result fits the limits |

- **Word timestamps**: no fix moves them, so the speech keeps its times.
- **Rounding**: the 4 fixes other than `MergeShortCues` take the options as given and round only the new time to whole milliseconds. 5.1 s minus 2/24 s is 5.016666667 s, which rounds to 5.017 s.
- **Limits of each fix**: no end moves before its start. `ExtendShortCues` never shortens a cue and never adds an overlap. `LimitLongCues` never lengthens a cue.
- **Same start**: cues with the same start, such as a sign and a dialogue line, all end before the next later start. They can still overlap each other.
- **Time limit**: `ExtendShortCues` and `AddLeadInOut` add a duration to a cue time, so they check the time limit of retiming. A cue end must stay below 100,000 hours.
- **Errors**: a negative gap or lead returns an error that matches `ErrInvalidArgument`. So does a duration of 0 or less for `ExtendShortCues` and `LimitLongCues`. On an error the subtitle stays unchanged.

### Lead-in and lead-out

<!-- example: ExampleSubtitle_AddLeadInOut -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Is the ferry running today?"}},
	model.Cue{Start: 2100 * time.Millisecond, End: 3 * time.Second, Lines: []string{"Only the morning one."}},
)

if err := s.AddLeadInOut(200*time.Millisecond, 300*time.Millisecond, 40*time.Millisecond); err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Start, c.End, c.Text())
}
// Output:
// 800ms 2.06s Is the ferry running today?
// 2.1s 3.3s Only the morning one.
```

- **Order**: the lead-out comes first, so it takes the space in a short gap. The first cue ends at 2.06 s, 40 ms before the next cue, so the next cue keeps its start.
- **Inside another cue**: a start or an end inside another cue does not move.
- **Result**: no start goes below 0, no start moves later, no end moves earlier, and 2 cues that did not overlap before do not overlap after.

### Short cues

Speech-to-text output often splits a sentence into cues of less than 1 s. Take `Wait.` from 62.1 s to 62.6 s, `Where are you` from 62.64 s and `going?` up to 64.1 s. `MergeShortCues(model.MergeShortCuesOptions{})` joins them into `Wait. Where are you going?` from 62.1 s to 64.1 s.

- **Short cue**: a cue that lasts less than `Limits.MinDuration`, or has fewer visible characters than `MinCharacters`.
- **Partner**: the next cue first, then the previous cue. A joined cue that is still short joins again.
- **Join check**: both cues have the same alignment, forced flag and `<v>` speakers. The gap between them is at most `MaxGap`, and the joined cue lasts at most `Limits.MaxDuration`. The lines must wrap into `Limits.MaxCharactersPerLine` and `Limits.MaxLinesPerCue`, with each dialogue turn on lines of its own. A `Limits.MaxCharactersPerSecond` above 0 limits the reading speed too.
- **Defaults**: the limits default to the Netflix limits for English: 42 characters per line, 2 lines and 7 s. `MinDuration` defaults to 1 s and `MaxGap` to 250 ms.
- **Why pointers**: 0 is a legal `MinDuration` and `MaxGap` and differs from the default. `new(500 * time.Millisecond)` sets one in one expression.
- **One speaker**: when both cues start with a `<v>` tag of one speaker, the result gets one tag. `<v.loud Anna>Hi.</v>` and `<v Anna>Hello.</v>` become `<v.loud Anna>Hi. Hello.</v>`.
- **Options**: `KeepSentenceEnds` stops a join after a cue that ends with `.`, `?` or `!`. `MergeSameSpeakerAnyDuration` makes every cue a candidate, joins only cues of one `<v>` speaker, and has no duration limit.
- **Result**: the joined cue keeps the start, the ID, the alignment, the forced flag and the format data of the first cue. A comment before a joined cue moves before the result.
- **Rounding**: the checks round `MinDuration`, `MaxDuration` and `MaxGap` to whole milliseconds, as PHP does.

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
- **moveToZero**: subtracts `from` from the times and the word timestamps, then rounds once to whole milliseconds, as PHP does. A time below 0 becomes 0. Without it, word timestamps stay, also when they fall outside the cut cue.
- **moveToZero and Shift**: for a `from` of whole milliseconds, `Slice(from, to, true)` equals `Slice(from, to, false)` followed by `Shift(-from)`. Otherwise the 2 differ, because the second way rounds twice. Example: a cue from 1 s to 5 s with `from` 2.0005 s starts at 0 with `moveToZero` and at 0.001 s after the shift.

### Split and join

- **Split**: `SplitCue(2, 9500*time.Millisecond, 1)` of a cue from 6.5 s to 12.5 s with 2 lines gives one cue from 6.5 s to 9.5 s with line 1 and one from 9.5 s to 12.5 s with line 2. `splitAfterLine` counts from 1, so a cue of 1 line cannot split.
- **Second part**: copies the alignment, the forced flag and the format data, and has no ID. It takes its place in start order after the first part. Word timestamps do not move.
- **Join**: the joined cue holds the lines of all cues in order and ends at the latest end. The start, the ID, the alignment, the forced flag and the format data come from cue `first`. Join checks no speaker, alignment or forced flag.
- **Join example**: cues `zero` 0 s to 1 s, `one` 1 s to 2 s, `two` 2 s to 4 s, `three` 5 s to 6 s and `four` 7 s to 8 s, with a comment before each cue from `one` on. `JoinCues(1, 3)` gives `zero`, then `one\ntwo\nthree` from 1 s to 6 s, then `four`. The comments of `one`, `two` and `three` sit before the joined cue.

### Duplicates

`RemoveDuplicateCues` joins `Hello` from 1 s to 2 s and `Hello` from 2 s to 3 s into `Hello` from 1 s to 3 s.

A cue continues the run of the cue before it when all of these hold, compared with the first cue of the run:

- the same `Text()`, tags and case included
- the same alignment, forced flag and set of `<v>` speaker names
- the same format data: the same format keys, and per format the same keys, values, value types and key order. The order of the format keys does not count. An ASS event with style `Glow` and one with `Default` stay apart
- a gap of at most `maxGap` to the run, rounded to whole milliseconds

The run joins into its first cue, which keeps its lines and ends at the latest end of the run.

### Same-time cues

`MergeSameTimeCues(0)` joins `- Where are you going?` and `- Home.`, both from 1 s to 3 s, into one cue with both lines.

- **Group**: a cue joins the group when its start and its end both differ by at most `timeTolerance` from the first cue of the group, and it has the alignment and the forced flag of that cue. Format data and speakers do not count, so ASS events with different styles join.
- **Lines**: the lines of each cue in order. A cue whose `Text()` came before in the group adds no lines.
- **Result**: the start, the ID and the format data of the first cue, and the latest end of the group. No dialogue dash is added.

## Line wrapping

Subtitle guidelines limit a line to a number of visible characters, often 42, and a cue to 2 lines. `s.WrapLines(42, 2)` turns `This is a very long subtitle line that does not fit on the screen` into `This is a very long subtitle line` and `that does not fit on the screen`.

<!-- example: ExampleSubtitle_WrapLines -->
```go
var s model.Subtitle
s.AddCues(
	model.Cue{Start: time.Second, End: 4 * time.Second, Lines: []string{"<i>This is a very long subtitle line that does not fit on the screen</i>"}},
	model.Cue{Start: 5 * time.Second, End: 7 * time.Second, Lines: []string{"- Are you coming with us to the coast tonight?", "- Yes."}},
)

if err := s.WrapLines(42, 2); err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.All() {
	fmt.Println(c.Text())
}
// Output:
// <i>This is a very long subtitle line</i>
// <i>that does not fit on the screen</i>
// - Are you coming with
// us to the coast tonight?
// - Yes.
```

| Method | Does |
|:--- |:--- |
| `WrapLines(maxCharactersPerLine, maxLinesPerCue)` | breaks the lines of each cue that does not fit. A cue that fits stays as it is |
| `UnwrapLines()` | joins the lines of every cue. Each dialogue turn that starts with a dash line keeps a line of its own |

- **Visible characters**: tags count 0, an entity such as `&amp;` counts 1, other text counts grapheme clusters. Invalid UTF-8 counts bytes. A line is measured as its words joined by 1 space.
- **Breaks**: the wrapper uses the fewest lines up to the limit that fit, then the fewest breaks inside a CJK phrase, then the most equal line lengths. A word longer than the line never breaks. When no line count fits, the cue gets `maxLinesPerCue` lines with the least overflow.
- **Tags**: each break closes the open `b`, `i`, `u`, `s`, `font` and `v` tags and opens them again on the next line.
- **Dialogue turn**: the part of a cue that one person speaks. A turn starts at the first line, at each line that starts with a dialogue dash, and inside a line at a dash after `.`, `?`, `!`, the ellipsis U+2026 or a CJK sentence end. Each turn wraps on its own lines, so a cue with 2 turns can end with more than `maxLinesPerCue` lines.
- **Minus sign**: a dash before a digit, as in `-20 degrees`, starts no turn. A dash in mid sentence, as in `the coast - all the way`, starts none either.
- **Chinese and Japanese**: a line also breaks between 2 CJK characters where JIS X 4051 allows it. No line starts with a closing bracket, a stop, a small kana or the long vowel mark U+30FC. No line ends with an opening bracket. A break after CJK punctuation or the ideographic space U+3000 wins over equal line lengths. There is no dictionary, so a break can fall inside a Japanese word.
- **Latin words in CJK text**: `WhatsApp` inside Chinese text stays whole. Korean breaks only at spaces.
- **Joining**: 2 lines join with a space, or without one when a CJK character ends the first and starts the second.
- **Right-to-left lines**: a cue with the embedding of `FixRightToLeft` gets a new embedding on each new line, because a line break ends every embedding in the Unicode bidi algorithm.
- **Errors**: a limit below 1 returns an error that matches `ErrInvalidArgument`, and the subtitle stays unchanged.
