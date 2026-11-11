# Validation

A QC step checks every cue against limits for reading speed, line length, timing and text style before delivery. `validation.Check(s, rules)` returns one `Violation` per broken rule and changes nothing. Example: `stbox validate trip.srt --max-cpl 42 --check-overlaps` prints `trip.srt: cue 2: maxCharactersPerLine 57, limit 42` and `trip.srt: cue 2: noOverlap 0.5`, and exits 1.

<!-- example: ExampleCheck -->
```go
const content = "1\n00:00:01,000 --> 00:00:03,000\n<i>The train leaves at noon.</i>\n\n" +
	"2\n00:00:02,500 --> 00:00:05,000\n[BELL RINGS]\nANNA: We need two tickets for the long ride to the coast.\n"
s, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}

violations, err := validation.Check(s, validation.Rules{MaxCharactersPerLine: 42, NoOverlap: true})
if err != nil {
	fmt.Println(err)
	return
}
for _, v := range violations {
	fmt.Println("cue", v.Cue+1, v.Rule, v.Value)
}
// Output:
// cue 2 maxCharactersPerLine 57
// cue 2 noOverlap 0.5
```

## Rules

A **rule** is one check that a field of `Rules` turns on. Its name is a `Rule` string such as `maxCharactersPerLine`. The name appears in the CLI text, in the JSON output and in this page. The zero `Rules` checks nothing, and each limit is off at its zero value.

A violation holds the measured value and, for a rule with a number limit, the limit. Both are `float64` in the unit of the rule. The 6 time rules report seconds. A cue of 0.4 s that breaks `MinDuration` of 5/6 s has the value 0.4 and the limit 0.833333333.

| Rule | Go field | Violation when | Value | Limit |
|:--- |:--- |:--- |:--- |:--- |
| `requireCues` | `RequireCues` | the subtitle has no cue | 0, with cue `NoCue` | none |
| `noNegativeDuration` | `NoNegativeDuration` | `End` is before `Start` | seconds, negative | none |
| `noEmptyCues` | `NoEmptyCues` | the cue has no visible character | 0 | none |
| `maxCharactersPerLine` | `MaxCharactersPerLine` | a line is longer than the limit. One violation per line | visible length | the limit |
| `maxLinesPerCue` | `MaxLinesPerCue` | more lines with visible text than the limit | line count | the limit |
| `maxCharactersPerSecond` | `MaxCharactersPerSecond` | the cue has characters and reads faster than the limit | characters per second, `+Inf` for a duration of 0 or less | the limit |
| `minDuration` | `MinDuration` | the duration is below the limit | seconds | the limit in seconds |
| `maxDuration` | `MaxDuration` | the duration is above the limit | seconds | the limit in seconds |
| `noDoubleSpaces` | `NoDoubleSpaces` | a run of 2 or more horizontal spaces has a character that is not white space on both sides | number of runs | none |
| `noLeadingOrTrailingSpaces` | `NoLeadingOrTrailingSpaces` | a text line starts or ends with a horizontal space | number of lines | none |
| `noUnbalancedTags` | `NoUnbalancedTags` | a tag has no partner | number of tags | none |
| `dialogueDashStyle` | `DialogueDashStyle` | a dialogue line uses another dash style | number of lines | none |
| `maxSpeakersPerCue` | `MaxSpeakersPerCue` | more speakers than the limit | speakers | the limit |
| `maxWordsPerMinute` | `MaxWordsPerMinute` | the cue has words and reads faster than the limit | words per minute, `+Inf` for a duration of 0 or less | the limit |
| `minSecondsPerWord` | `MinDurationPerWord` | the duration is below the limit times the word count | seconds per word | the limit in seconds |
| `allowedCharacters` | `AllowedCharacters` | a character is not allowed | number of characters | none |
| `noAllCapsLines` | `NoAllCapsLines` | a line is in upper case only | number of lines | none |
| `noOverlap` | `NoOverlap` | the cue starts before the latest end of the earlier cues | the overlap in seconds | none |
| `minGap` | `MinGap` | the gap is 0 or more and below the limit | gap in seconds | the limit in seconds |

- **Field name of `minSecondsPerWord`**: the Go field is `MinDurationPerWord`, because a Go name for a duration does not hold `Seconds`. The `Rule` value stays `minSecondsPerWord`, because it is the CLI and JSON value.
- **Dialogue dash style**: `DialogueDashStyle` is a pointer, because the zero `model.DialogueDashStyle` is a legal style. nil turns the rule off.
- **Infinite limits**: `MaxCharactersPerSecond` and `MaxWordsPerMinute` accept `+Inf`, which checks nothing.
- **Order**: `requireCues` comes first. Then each cue in start order gets its violations in the order of the table above, from `noNegativeDuration` to `minGap`. `maxCharactersPerLine` gives its violations in line order.
- **No violation**: `Check` returns a nil slice.
- **Read only**: `Check` never changes the subtitle.

### Definitions

| Term | Meaning |
|:--- |:--- |
| duration | `End - Start` of the cue |
| seconds | the duration as `float64(d.Milliseconds()) / 1000` |
| visible length | the length that [markup.md](markup.md#helpers) defines: tags count 0, an entity counts 1, a grapheme cluster counts 1, and outer spaces count 0. Invalid UTF-8 counts bytes |
| characters of a cue | the sum of the visible lengths of its lines |
| plain line | a line without tags and with entities decoded |
| text line | a plain line that is not empty after trimming space, tab, LF, CR, NUL and VT. U+00A0 is not trimmed |
| words | the plain lines joined with LF, split at white space. A lone dash is a word. [statistics.md](statistics.md) counts words the same way |
| latest end | the latest `End` of all cues before this cue in start order |
| gap | the start of the cue minus the latest end |
| horizontal space | tab, U+0020, U+00A0, U+1680, U+180E, U+2000 to U+200A, U+202F, U+205F and U+3000. For a line that is not valid UTF-8, tab and U+0020 only |

Limits compare in whole milliseconds. A cue of 0.833 s meets `MinDuration` of 5/6 s, because the limit rounds to 833 ms.

### Unbalanced tags

The rule reads the raw lines joined with LF. A tag starts with `<` and an optional `/`. Its name is `b`, `i`, `u`, `s`, `font`, `v`, `c`, `lang`, `ruby` or `rt` in any case. White space, `.` or `>` follows the name. The rule walks the tags in order with a stack of open names.

1. An opening `rt`: when the top of the stack is `rt`, it pops. When no `ruby` is on the stack, the count grows by 1. Then `rt` is pushed.
2. Any other opening tag is pushed.
3. A closing tag finds the last stack entry with its name. Without one, the count grows by 1. Else the entries above it are open tags with no end. For `</ruby>`, the first of them is dropped when it is `rt`. Each of the others that is not `v` adds 1. The stack is cut below the match.
4. At the end, each entry left that is not `v` adds 1.

So an open `<v>` needs no `</v>`, and `</ruby>` or the next `<rt>` closes an open `<rt>`. A word timestamp, `<br>`, `&lt;i&gt;` and other tags do not count. `<b><i>Crossed</b></i>` gives 2. `<ruby>a<rt>b<rt>c` gives 2.

### Dialogue dash

A line opens a dialogue turn when it starts with a hyphen-minus, U+2010, U+2013 or U+2014, and no second one of them follows. Optional space, tab or U+00A0 follow. Then comes a character that is neither white space nor a number. So `-20 degrees` and `--- a line` open no turn. Spaces and bidi controls before the dash do not matter.

A line breaks `dialogueDashStyle` when it opens a turn and does not start with the text of the style followed by a character that is not white space. With the style `- `, the line `- Yes.` passes and `-No.` counts. With the style `-`, `- Yes.` counts.

### Speakers

The speaker count is the largest of 3 numbers:

- the text lines that open a dialogue turn
- the distinct names of the `<v>` tags, compared byte by byte
- 1 when the cue has a text line, else 0

So a plain cue has 1 speaker, and `<v Anna>Hi`, `<v.loud Tom>Hello`, `<v Anna>Bye` has 2.

### Allowed characters

`AllowedCharacters` is a **class** when it has 2 or more bytes, starts with `[` and ends with `]`, for example `[A-Za-z0-9 .,!?]`. Else it is a **list** of allowed characters, such as `Cafab`. A character is a code point, or a byte of a line that is not valid UTF-8. U+0020 is always allowed. Every other character of a text line counts when the list does not hold it or the class does not match it.

- **Class syntax**: the class compiles with `github.com/dlclark/regexp2/v2` as `^(?:CLASS)$`, with a match timeout of 1 second. A timeout returns an `*model.InvalidArgumentError`.
- **Invalid UTF-8**: a line that is not valid UTF-8 matches the class byte by byte. An ASCII byte is its own character. Any other byte is the code point of the same number.
- **POSIX classes**: a class with `[:`, such as `[[:alpha:]]`, fails `Validate`, because regexp2 skips POSIX classes without an error. Use `\p{L}`.

### All caps

The rule removes each `[...]` and `(...)` span, without nesting. A line counts when the rest has 2 or more upper case letters and no lower case letter. For a line that is not valid UTF-8, only `A` to `Z` and `a` to `z` count. So `I`, `[BELL RINGS]` and `(SHOUTS) Stop!` do not count.

## Errors

`Rules.Validate` returns an `*model.InvalidArgumentError` that matches `ErrInvalidArgument`. Its `Option` is the field path, such as `Rules.MaxCharactersPerLine`. `Check` calls `Validate` first.

| Case | Message |
|:--- |:--- |
| a number limit below 0, or NaN | `The limit maxCharactersPerSecond must be 0 or more, got -1.` |
| a time limit below 0 | `The limit minDuration must be 0 or more, got -1.5.` The number is in seconds |
| a dialogue dash style that is none of the 8 constants | `The dialogue dash style 9 is not one of the 8 styles.` |
| a class that regexp2 rejects | `The allowed characters "[z-a]" are not a valid character class: REASON.` |
| a class with `[:` | `The allowed characters "X" hold a POSIX class such as [:alpha:]. Use a Unicode class such as \p{L}.` |

## Command line

`stbox validate <input>... [rule options] [--json] [input options]` reads each input and prints its violations. [cli.md](cli.md#validate) lists the options.
