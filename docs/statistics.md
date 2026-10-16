# Statistics

A QC script wants numbers about a subtitle file. `stats.Of(s)` returns them in one value and never changes `s`. Example: for a 3-cue SubRip file, `stbox info trip.srt` prints `Characters per second: min 12.5, average 27.533, max 42.5` and `Most used words: the (3), train (1), ...`.

<!-- example: ExampleOf -->
```go
s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}

st := stats.Of(s)
fmt.Println(st.CueCount, "cues,", st.WordCount, "words,", st.CharacterCount, "characters")
fmt.Printf("characters per second: min %.1f, max %.1f\n", st.CharactersPerSecond.Min, st.CharactersPerSecond.Max)
fmt.Println("gaps:", st.Gaps.Min, st.Gaps.Max)
fmt.Println("most used:", st.MostUsedWords[0].Word, st.MostUsedWords[0].Count)
fmt.Println("span:", *st.Span)
// Output:
// 3 cues, 22 words, 111 characters
// characters per second: min 12.5, max 42.5
// gaps: -500ms 1s
// most used: the 3
// span: 5.4s
```

## Numbers

A range holds the minimum, the arithmetic mean and the maximum. A range is nil when no value qualifies, so a reader tells "no data" from 0.

| Field | Definition |
|:--- |:--- |
| `CueCount` | all cues, also those without text |
| `WordCount` | the words of the cues with visible text |
| `CharacterCount` | the sum of the visible line lengths |
| `TotalDisplayTime` | the sum of `End - Start` of every cue. A cue that ends before it starts adds a negative value |
| `Span` | the latest end minus the start of the first cue. Nil without cues |
| `CharactersPerSecond` | for each cue with characters and a duration above 0: characters divided by the duration in seconds |
| `WordsPerMinute` | for each of the same cues: words divided by the duration in seconds, times 60 |
| `CharactersPerLine` | for each line with a visible length above 0 |
| `Gaps` | for each cue after the first: its start minus the latest end of the earlier cues. An overlap gives a negative gap. Nil with fewer than 2 cues |
| `MostUsedWords` | every counted word with its count, the most used first. Words with the same count keep the order of their first use |

- **Visible length**: tags count 0, an entity such as `&amp;` counts 1, outer spaces and bidi marks count 0, and a grapheme cluster counts 1. [markup.md](markup.md#helpers) has the rules. Invalid UTF-8 counts bytes.
- **Cues without text**: a cue with no visible character counts in `CueCount`, `TotalDisplayTime`, `Span` and `Gaps` only.
- **Word**: the text of the lines joined with LF, without tags and with entities decoded, split at white space. A dialogue dash counts as a word.
- **Counted word**: a word with the punctuation (Unicode category P) and symbols (category S) removed at both ends, in lower case with the full Unicode mapping, as PHP `mb_strtolower` does. A word that is empty after that is in `WordCount` but not in `MostUsedWords`. A word of digits such as `2024` stays a string.
- **Invalid UTF-8**: the words split at ASCII white space. A word that is not valid UTF-8 loses ASCII punctuation only and lower-cases ASCII letters only. A valid word in the same cue gets the full rules.
- **Durations**: every cue time is a whole number of milliseconds, so sums and differences are exact. `DurationRange.Average` holds whole nanoseconds. `TotalDisplayTime` stops at the largest or smallest `time.Duration` instead of wrapping.
- **White space**: U+180E, the Mongolian vowel separator, splits words, as it does in PHP.

## JSON

`Statistics` marshals to the shape of PHP `SubtitleStatistics::toArray()`, which `stbox info --json` prints under `statistics`.

- **Keys**: `cueCount`, `wordCount`, `characterCount`, `totalDisplayTime`, `span`, `charactersPerSecond`, `wordsPerMinute`, `charactersPerLine`, `gaps`, `mostUsedWords`, in this order.
- **Durations**: seconds in a number. 4.9 s writes `4.9`, 1 s writes `1`. The average of the gaps has at most 9 decimals, because it is whole nanoseconds: 2/3 s writes `0.666666667`, where PHP writes `0.6666666666666666`.
- **Ranges**: `{"min": 12.5, "average": 27.53333333333333, "max": 42.5}`. A nil value writes `null`.
- **Most used words**: the first 10 entries as `[{"word": "the", "count": 3}, ...]`, `[]` for none.

## Differences from PHP

[php-differences.md](php-differences.md) lists the rows for #44. They concern the gap average and the Unicode tables.
