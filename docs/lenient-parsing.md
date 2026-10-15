# Lenient reading

A subtitle download is often broken in one place. Example: `bad_timestamp.srt` has the end time `00:00:0G,000` in cue 2.

- **Strict mode**, the default: `stbox.Parse` returns a `*ParseError` with `Line` 6 and the message `The time "00:00:0G,000" is not valid.`.
- **Lenient mode**, `ReadOptions{Lenient: true}`: the reader skips cue 2, reads the other 3 cues and returns a nil error. `ReadInfo.Warnings` holds one **parse warning** per problem.

## Parse warnings

| Field | Holds |
|:--- |:--- |
| `Message` | the full sentence, for example `The time "00:00:0G,000" is not valid.` |
| `Line` | the 1-based input line, or `NoLine` (0) |
| `Block` | the 0-based block index as in `Block #n` messages, or `NoBlock` (-1) |
| `Lines` | the lines of the block, trimmed as the reader read them |
| `Action` | `Skipped`: the reader dropped the block. `Repaired`: the reader kept the block and fixed it |

- **Where**: `ReadInfo.Warnings` of `stbox.Load` and `stbox.Parse`, and the warning slice of each format package `Read`. The `Subtitle` holds no warnings, so they do not travel with copies and edits.
- **Plain ints**: `Line` and `Block` use 0 and -1 for "none", so a warning prints without nil checks. Only the library builds warnings.

## Mode rules

| Situation | Strict | Lenient |
|:--- |:--- |:--- |
| a broken block | the `*ParseError` | a `Skipped` warning, the reader reads on |
| a repair | most repairs run without a warning | a `Repaired` warning |
| a whole-file error: invalid JSON, invalid XML without a root, an ASS file without `[Script Info]` and `[Events]` | the `*ParseError` | the same `*ParseError` |
| a byte that the chosen source encoding cannot decode | the `*ParseError` | the same `*ParseError` |

- **Strict reads can warn**: some repairs warn in both modes, for example the CSV guess of milliseconds. So `ReadInfo.Warnings` can hold warnings after a strict read with a nil error.
- **Every reader honours `Lenient`.**
- **Fresh state**: each read starts without warnings. A strict read after a lenient read of the same input returns the error.
- **Skipped message**: a skipped block takes `Msg` of the `*ParseError` as its message. When the warning has no line and the error has one, the message ends with ` (line N)`.

## Shared warnings

`ReadInfo.Warnings` holds the warnings in this order: the decode warning, the warnings of the text pre-pass, then the warnings of the reader in input order.

| Message | Action | Line | Block |
|:--- |:--- |:--- |:--- |
| `The file has control characters, the first on line N. The parser removed them.` | Repaired | N | none |
| `The cue ends at E s, before it starts at S s. The parser swapped the times.` | Repaired | the timing line | the block |
| `The file has text but no cues.` | Skipped | the first text line | none |

- **Control characters**: the pre-pass of [reading-and-writing.md](reading-and-writing.md#text-pre-pass) removes them in both modes and warns once per file in lenient mode.
- **Text but no cues**: only readers that skip unknown text without an error check for it. The read gave no cue and skipped no block.

## Time order and limits

| Check | Strict | Lenient |
|:--- |:--- |:--- |
| a cue ends before it starts | `The cue ends at E s, before it starts at S s.` | swap the times and warn, when the cue then lasts 30 s or less. A longer cue fails as in strict mode |
| a time of 100000 hours or more | `The time "T" is not below 100000 hours.` | the block fails |
| a JSON time of 100000 hours or more | `The field PATH is not below 100000 hours.` | the cue fails |
| a word timestamp such as `<100000:00:00.000>` | `The time "100000:00:00.000" is not below 100000 hours.` | the block fails |

- **Equal times**: a cue whose end equals its start is valid.
- **No swap**: CSV and TSV never swap. A comma in a time or a cut-off last row gives such an end, and a swap would invent times.
- **Seconds**: E and S print as PHP prints a float: 3 s gives `3`, 1.5 s gives `1.5`.
- **Quotes**: a message that quotes a line cuts it to 60 characters. A longer line keeps 57 characters and `...`.

## Tests

[lenient_test.go](../lenient_test.go) reads each fixture of [testdata/lenient](../testdata/lenient) in both modes. The file of the same name in `testdata/lenient/expected` holds the expected cues, warnings and strict result, taken from the PHP tests. A row runs once its format has a reader.
