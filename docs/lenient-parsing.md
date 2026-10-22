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

`ReadInfo.Warnings` holds the warnings in this order: the decode warning, the warnings of the text pre-pass, then the warnings of the reader in input order. The library JSON reader orders its warnings by cue, see [json.md](json.md#warnings).

| Message | Action | Line | Block |
|:--- |:--- |:--- |:--- |
| `The file has control characters, the first on line N. The parser removed them.` | Repaired | N | none |
| `The cue ends at E s, before it starts at S s. The parser swapped the times.` | Repaired | the timing line | the block |
| `The file has text but no cues.` | Skipped | the first text line | none |

- **Control characters**: the pre-pass of [reading-writing.md](reading-writing.md#text-pre-pass) removes them in both modes and warns once per file in lenient mode.
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

## WebVTT

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| no `WEBVTT` line, a damaged one such as `WEBVTS`, or text before it | `The file does not start with WEBVTT.` | skips the lines before the first `WEBVTT` line. Without one before the first timing line, skips the lines before the first cue. `Repaired`, block 0, the skipped lines |
| no empty line after the header | `The WEBVTT header has no empty line before the first cue.` | splits the header block before its first timing line. `Repaired`, block 0 |
| an empty line inside a cue | the text after it fails as an unknown block | joins the text to the cue. `Repaired`, the join warning of the shared table |
| a time with 1, 2 or 4 fraction digits, or `,` before the fraction, as in `00:01,5` | `The time "T" is not valid.` | reads it. `Block #B has the time "T", which is not in the form hh:mm:ss.mmm. The parser read it as S s.` `Repaired`, once per time string |
| a cut-off time such as `00:00:0`, minutes or seconds of 60 or more, `->`, `--->` | the error of the block | skips the block |
| a block that is no cue, comment, style or region, such as the `WEBVTT` header of a second joined file | `Block #B is not a WebVTT cue, comment, style or region. The line is "Q".` | skips the block |
| a `STYLE` or `REGION` block after the first cue | ignored, as the spec says | skips it with `Block #B is a STYLE block after a cue. The spec allows it only before the first cue, so the parser skipped it.` `Skipped` |

- **Not WebVTT**: lenient mode still fails without a `WEBVTT` line when the first timing line is no WebVTT timing line, such as `00:00:01,000 --> 00:00:02,000`, or when the file has no timing line. Detection always needs the `WEBVTT` line.
- **Damaged signature**: the reader skips the header text and the `STYLE` and `REGION` blocks before the first cue.

## SubRip

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| an empty line inside a cue | joins the text to the cue | joins it. `Block #B has an empty line before line N inside the cue. The parser kept the text after it in the cue.` `Repaired` |
| no empty line before a cue | splits the block before the timing line | splits it. `Block #B has no empty line before line N. The parser split the block there.` `Repaired`, once per new part |
| no cue number before the timing line | `Block #B has no cue number on its first line. The line is "Q".` | reads the cue. `Block #B has no cue number on line N. The parser read the cue without it.` `Repaired` |
| `->` or `--->` as the arrow | `Block #B has no timing line on its second line. The line is "Q".` | reads it. `Block #B has the arrow "A" in its timing line. The parser read it as "-->".` `Repaired` |
| the full-width delimiters U+FF1A, U+FF0C, U+FF0E or U+3002 in the timing line | the error of the time or of the timing line | reads them as `:`, `,` and `.`. `Block #B has full-width delimiters in its timing line. The parser read them as ASCII.` `Repaired` |
| a time such as `0:0:1,500`, `00:00:01,5000`, `00:00:01:50` or `07:03,920` | `The time "T" is not valid.` | reads it, a 4-digit fraction rounded to whole milliseconds. `Block #B has the time "T", which is not in the form hh:mm:ss,mmm. The parser read it as S s.` `Repaired`, once per time string |
| a cut-off time such as `00:00:0`, minutes or seconds of 60 or more | `The time "T" is not valid.` | skips the cue |
| unknown text after the end time | `The text "U" after the end time is not valid.` | keeps the cue settings and ignores the rest. `Block #B has the unknown text "U" after the end time. The parser ignored it.` `Repaired` |

- **Block numbers**: B counts the blocks after the join, from 0.
- **Line numbers**: the cue warnings report the timing line.
- **Order** inside a block: the join warnings, the split warnings, then for each cue the swap, the unknown text, the loose times, the full-width delimiters, the arrow and the missing cue number.

## ASS and SSA

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| an event with fewer fields than the `Format:` line | `The line "L" has fewer fields than the Format line of the [Events] section.` | skips the event |
| a `Format:` line without `Start`, `End` or `Text` | `The Format line of the [Events] section needs the fields Start, End and Text.` | skips each event |
| a field name that is a number, such as `1`, in the `Format:` line of `[Events]` | `The field name "1" in the Format line of the [Events] section is a number.` on the `Format:` line | drops the values of that field. The same message and ` The parser dropped the values of this field.` `Repaired`, no block |
| a time without fraction, with 4 fraction digits, or with `,` or `:` before the fraction, such as `0:00:01`, `0:00:01.5000`, `0:00:01,50` or `0:00:01:50` | `The time "T" is not valid.` | reads it, rounded to whole milliseconds. `The time "T" is not in the form h:mm:ss.cc. The parser read it as S s.` `Repaired` |
| minutes or seconds of 60 or more, such as `0:75:02.00` | `The time "T" is not valid.` | skips the event |
| a time or a karaoke time of 100000 hours or more | `The time "T" is not below 100000 hours.`, with the karaoke tag as T | skips the event |

- **Comma fractions**: before the split of an event, lenient mode joins a `Start` or `End` value such as `0:00:01` with a next value of 1 to 4 digits, when another value follows. So `0:00:07,50,0:00:08,00` reads as 7.5 s and 8 s.
- **Block numbers**: B counts the `Dialogue:` and `Comment:` events from 0. The warnings report the event line and hold it as their lines.
- **Order** inside an event: the swap warning, then the loose times.
- **Text but no cues**: the first line in `[Events]` without `:` that does not start with `;`.

## TTML

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| text before the XML, such as a credit line | `The file has text before the XML: "L".` on the line of the text | skips it. The message plus ` The parser skipped it.` `Repaired`, the skipped lines |
| an HTML entity that XML does not define, such as `&eacute;` | `The file is not well-formed XML.` | reads it as the character. `The file has HTML entities that XML does not define, such as "&eacute;". The parser read them as characters.` `Repaired`, once per file, on the line of the first one |
| markup that is not well-formed: a bare `&`, `<br>` without `/`, a missing end tag | `The file is not well-formed XML.` | escapes the `&`, closes the `<br>` and repairs the elements as libxml recover mode does. `The file is not well-formed XML. M. The parser repaired it.` `Repaired`, on the line of the parser error M |
| a file without a root element after the repair | `The file is not well-formed XML.` | the same error |
| a frame label of 30 or more, such as `00:00:01:45`, without `ttp:frameRate` | `The frame label L in "E" is not below the default frame rate of 30, and the file has no ttp:frameRate.` | reads the frames at 50 fps below label 50, at 60 fps below 60, and the last field as hundredths of a second from 60. The message plus what the reader did. `Repaired` |
| a time such as `0:01:02.500`, `02:15.500`, `00:00:02:250`, `4200` or `19.764` | `The time expression "E" is not valid.` | reads it. `The time expression "E" is not valid. The parser read it as Ns.` `Repaired`, the paragraph index inside a `<p>` |
| a bad time on `<body>` or a `<div>` | the error of the time | uses the times of the parent. The message plus ` The parser ignored the timing of the div.` `Repaired` |
| a bad time on a `<p>` | the error of the time | skips the paragraph |
| a bad time on a `<span>` | drops the word timestamp | drops the word timestamp |
| a `<p>` that begins at or after the end of its parent | `The paragraph begins at Bs, but its parent ends at Es.` | reads the times of the `<p>` without the offset of the parent. The message plus ` The parser read its times as absolute: As to Zs.` `Repaired`. When that gives no end after the begin, skips the paragraph |
| a `<p>` without an end and without timed spans | `The paragraph that begins at Bs has no end time.` | ends it at the next paragraph that begins later. The message plus ` The parser ended it at the next paragraph at Ns.` `Repaired`. A last open paragraph is skipped with the strict message |

- **Paragraph index**: the block of a warning is the index of the `<p>` in document order from 0, also of the paragraphs that fail.
- **Line numbers**: a warning reports the line of the element, which is the line of the `>` that ends its start tag. [errors.md](errors.md#line-numbers) lists the strict errors with line 0.
- **Time limit**: the limit applies to the expression and to its sum with the times of the parents, after the rounding to whole milliseconds. So `99999999999999999999:00:04.000` fails in both modes.
- **Order**: the text warning, the entity warning and the repair warning come first, then the frame rate warning, then the warnings of the containers and paragraphs in document order.

## SAMI

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| a `SYNC` tag without `>` or without a number in `Start`, such as `<SYNC Start=>` | `SYNC tag N has no valid Start attribute.` | skips the block. `Skipped` |
| a `Start` of 100000 hours or more | `The time "D" is not below 100000 hours.` | skips the block. `Skipped` |
| a negative `Start` | reads it as 0 | reads it as 0. `SYNC tag N has a negative Start. The parser read it as 0.` `Repaired` |
| no `<P>` class in the classes of `<STYLE>` | reads the first `<P>` class | reads it. `No <P> class matches a class of the STYLE block. The parser read the class X.` `Repaired`, no block, the first line of the body that holds `Class=X` |

- **Block numbers**: N counts the `SYNC` tags from 1. The warning block is N minus 1.
- **Line numbers**: the line of the `<SYNC`. The reader counts lines after it removes every `</SYNC>`, as PHP does. A `</SYNC>` with a line end inside it moves the later lines up by one.

## Tests

[lenient_test.go](../lenient_test.go) reads each fixture of [testdata/lenient](../testdata/lenient) in both modes. The file of the same name in `testdata/lenient/expected` holds the expected cues, warnings and strict result, taken from the PHP tests. A row runs once its format has a reader.
