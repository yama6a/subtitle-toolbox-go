# Comparing two subtitles

A translator delivers version 2 of a subtitle. It adds `Welcome.` at the start, changes `Milbrook` to `Millbrook` and drops `Thank you.`. `diff.Compare(&v1, &v2, diff.Options{})` lists these 3 differences. It pairs cues by time and text, not by cue number. So the added cue does not shift the later pairs.

<!-- example: ExampleText -->
```go
var v1, v2 model.Subtitle
v1.AddCues(
	model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
	model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Milbrook."}},
	model.Cue{Start: 8 * time.Second, End: 9 * time.Second, Lines: []string{"Thank you."}},
)
v2.AddCues(
	model.Cue{Start: 0, End: 500 * time.Millisecond, Lines: []string{"Welcome."}},
	model.Cue{Start: 1 * time.Second, End: 3 * time.Second, Lines: []string{"The train is late."}},
	model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{"The next stop is Millbrook."}},
)

differences, err := diff.Compare(&v1, &v2, diff.Options{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Print(diff.Text(differences))
// Output:
// added: new cue 1
// + 00:00:00.000 --> 00:00:00.500
//   Welcome.
//
// text changed: old cue 2, new cue 3
// - 00:00:04.000 --> 00:00:06.000
//   The next stop is Milbrook.
// + 00:00:04.000 --> 00:00:06.000
//   The next stop is Millbrook.
//
// removed: old cue 3
// - 00:00:08.000 --> 00:00:09.000
//   Thank you.
```

- **Fields**: each `Difference` holds its `Kind`, and `OldIndex` and `NewIndex` as 0-based cue indexes. The side without a cue has -1.
- **Equal**: `diff.Equal(&v1, &v2, diff.Options{})` is true when `Compare` finds no difference.

`stbox diff v1.srt v2.srt` prints the same report and exits 1 when the files differ. [cli.md](cli.md) holds the options.

`diff.Text` writes one block per difference. Cue numbers start at 1.

```
text changed: old cue 12, new cue 12
- 00:00:39.000 --> 00:00:40.500
  I'll be their.
+ 00:00:39.000 --> 00:00:40.500
  I'll be there.
```

## Options

| Field | Effect |
|:--- |:--- |
| `TimeTolerance` | The largest start or end difference that counts as the same time. Nil means 1 ms. `0` compares the times exactly |
| `IgnoreFormatting` | Compares the text without tags and with entities decoded |
| `IgnoreWhitespace` | Compares the text without spaces, tabs and line breaks |
| `TextOnly` | Reports no timing changes. The times still help to pair the cues |

A negative `TimeTolerance` returns an error that matches `ErrInvalidArgument`.

## Kinds

| Kind | Meaning |
|:--- |:--- |
| `added` | A new cue without an old partner |
| `removed` | An old cue without a new partner |
| `text changed` | The compared texts differ, or the forced flag differs |
| `timing changed` | The start or the end differs by more than the time tolerance |
| `text and timing changed` | Both |

- **Pairs without a change** are not in the list.
- **Copies**: `Old` and `New` are copies. Neither input changes, and a caller cannot change a subtitle through a difference.
- **Split cue**: one half pairs with the old cue as a text change. The other half is an added cue.
- **Moved cue**: a cue that moves past other cues is removed in one place and added in the other.

## How cues pair

Two cues pair when their text is the same or nearly the same, or when they overlap in time. The pairs keep their order in both subtitles. Among all such pair sets, the comparison takes the one with the largest total weight.

Weight of a pair: `2 * text score + time score`.

| Score | 1 | Between 0 and 1 | 0 |
|:--- |:--- |:--- |:--- |
| Text | The compared texts are equal | `1 - edit distance / length of the longer text`, counted in bytes | The shorter text is under 70 % of the longer one, or the result is under 0.7 |
| Time | Start and end both differ by at most the tolerance | The overlap divided by the span of both cues, when the overlap is at least half of the shorter cue | The overlap is under half of the shorter cue, or there is none |

A pair with weight 0 does not exist. Two cues at the same time with unrelated text pair as a text change. Two cues with the same text pair as a timing change, also when they do not overlap.

Example: `The bakery opens at six.` from 10 s to 12 s and from 100 s to 102 s is one timing change. `The train leaves now.` from 1 s to 3 s and `A different line.` from 2.9 s to 5 s are a removed and an added cue, because the overlap is 0.1 s of a shorter cue of 2 s.

## Limits

- **Large stretches**: cues with the same text on both sides split the lists into stretches. A stretch of up to 40,000 table cells (old cues times new cues) gets the full alignment. In a larger stretch, only the pairs of the heaviest chain stay. A cue with nearly the same text that does not overlap in time is then removed and added. The limit keeps the memory bounded.
- **Repeated text**: a text that occurs more than 50 times in the new subtitle, such as `[MUSIC]`, pairs by text only with the 50 cues of that text nearest in cue number. Time overlap still pairs the others.
- **Long cues**: new cues up to 60 s and longer cues sit in two lists. One cue of 2 hours then does not make every old cue scan every new cue.
- **Bytes**: the edit distance counts bytes. For non-ASCII text, this gives other scores than a count in characters.
- **Order**: removed cues come before added cues when both fill the same gap.

## Credit

The idea of comparing two subtitles by cue likeness comes from the Compare tool of [Subtitle Edit](https://github.com/SubtitleEdit/subtitleedit/blob/main/docs/features/compare.md). The algorithm is this repository's own.
