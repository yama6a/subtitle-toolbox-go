# Upgrade to 4.0

Version 4.0 moves the module path, and it moves word timing and the speaker out of the cue lines into `Cue.Words` and `Cue.Speaker`. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v4@latest
```

## The 3.x call and the 4.0 call

| 3.x | 4.0 |
|:--- |:--- |
| `import stbox "github.com/yama6a/subtitle-toolbox-go/v3"` | `import stbox "github.com/yama6a/subtitle-toolbox-go/v4"` |
| read word times from the tags in `cue.Lines` with the `markup` helpers | `for _, w := range cue.Words { fmt.Println(w.Text, w.Start, w.End) }` |
| read the speaker from the first `<v>` tag of `cue.Lines` | `cue.Speaker` |
| put `<v Anna>` in front of the first line | `cue.Speaker = "Anna"` |
| put `<00:00:01.000>` tags into a line | set `cue.Words`, or set the lines with tags and call `cue.ExtractWordTags()` |
| strip tags and keep the word timestamps | strip tags. `cue.Words` stays. Set `cue.Words = nil` to drop it |
| `markup.InsertWordTimestamps(text, words)`, `markup.TimedWord` | `cue.Words = []stbox.Word{{Text: "Hi", Start: time.Second, End: 2 * time.Second}}` |
| `markup.WordTimestamp(tag)` | `cue.Words[i].Start` |
| `markup.MapWordTimestamps(line, fn)`, `cue.MapWordTimestamps(fn)` | the retiming edits move `Words`. For another change, set `Start` and `End` of each word |
| `StripOptions{RemoveWordTimestamps: true}` | `StripOptions{}`, then `cue.Words = nil` on each cue |
| a reader of WebVTT, ASS, TTML, iTT, LRC, CSV or library JSON left the tags in `cue.Lines` | `cue.Lines` holds no word timestamps and no whole-cue voice tag. `cue.TaggedLines()` gives the 3.x lines |
| `RemoveDuplicateCues` compared `<v>` tags in the text | it compares `Speaker` and the words too |
| a WebVTT cue kept `<v.loud Anna>Hi</v>` in its lines | `Speaker` is `Anna`, and the `vtt` cue format data holds `voiceClass` `.loud` and `voiceEnd` true. Deleting that data writes `<v Anna>` |
| a text edit kept the tags in place | it searches the words again with `SyncWords`. A word that the edit changed beyond case loses its timing |

## Typed words

`Cue.Words` holds the timed words of a cue. A `Word` has a `Text`, a `Start`, an `End` and a `Confidence`. `Confidence` is a `*float64` from 0 to 1, and nil means the source gives none.

<!-- example: ExampleWord -->
```go
c := stbox.Cue{
	End:   5 * time.Second,
	Lines: []string{"The beach"},
	Words: []stbox.Word{
		{Text: "The", Start: 0, End: 240 * time.Millisecond},
		{Text: "beach", Start: 240 * time.Millisecond, End: time.Second},
	},
}
fmt.Println(c.TaggedLines())
// Output: [<00:00:00.000>The <00:00:00.240>beach]
```

- **Order rule**: the text of each word occurs in `cue.PlainText()`, after the end of the word before it. `ValidateWords` checks this and the time rules. `SetCue` calls it. `AddCue` does not.
- **Retiming**: `Shift`, `Scale`, `ConvertFrameRate`, `SyncByPoints`, `Append` with an offset and `Slice` with move to zero move `Start` and `End` of each word.
- **Old lines**: `cue.ExtractWordTags()` reads the 3.x form, such as `<00:00:01.000>Hi <00:00:01.400>there`. The end of each word is the start of the next word.

## Speaker

`Cue.Speaker` is the voice of the whole cue. A cue with 2 voices keeps its `<v>` tags in the lines, and `Speaker` stays empty. `StripFormatting` clears `Speaker` unless `KeepTags` holds `v`, so `--strip-tags` output stays the same.

## Markup helpers

The helpers of package `markup` for word timestamps are gone: `TimedWord`, `InsertWordTimestamps`, `WordTimestamp` and `MapWordTimestamps`. `markup.VoiceTag` stays for the `<v>` tags of a cue with 2 voices. `cue.TaggedLines()` returns the lines with the word timestamp tags and the voice tag of `Speaker`, as 3.x held them. The writers use it, so an unedited file writes the same bytes.

## Text edits

An edit that changes the lines of a cue calls `cue.SyncWords()`. It searches each word in the new text with simple case folding. A found word takes the matched text. A word that is not found loses its timing.

| Edit | Result |
|:--- |:--- |
| `Hello there` to `HELLO THERE` | both words found, with the new texts |
| `damn it` to `d*** it` | `damn` is dropped. The profanity filter masks the word too, so it keeps `d***` |
| `New York` to `NYC` | both words dropped |

## Module path

Every import path gets `/v4` in place of `/v3`. `go install github.com/yama6a/subtitle-toolbox-go/v4/cmd/stbox@latest` installs the CLI.
