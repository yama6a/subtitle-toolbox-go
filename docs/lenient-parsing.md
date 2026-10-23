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

## MicroDVD

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| no frame rate in the options and no `{1}{1}<fps>` first line | `The frame rate is unknown. Start the file with {1}{1}<fps>, set the frame rate, or read the file in lenient mode for 23.976 fps.` | reads at 23.976 fps. `The file has no {1}{1}<fps> line. The parser used 23.976 fps.` `Repaired`, no line and no block |
| junk before the `{1}{1}<fps>` line, such as a release name | the frame rate error above | takes the frame rate of the first line of the shape `{N}{N}text` and skips the junk line |
| a line of another shape, such as `{x}{120}Text` | `The line "L" is not a MicroDVD cue.` | skips the line |
| a time of 100000 hours or more, such as `{9000000000000}` at 25 fps | `The time "{N}" is not below 100000 hours.` | skips the line |
| `{1}{1}0` | `The frame rate must be greater than 0, got 0.` | the same error |

- **Block numbers**: B counts the non-empty lines from 0, the frame rate line too. The warnings report the line and hold it trimmed.
- **Quotes**: the message quotes the whole line, as PHP does, without the cut at 60 characters.

## SBV

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| an empty line inside a cue | joins the text to the cue | joins it. `Block #B has an empty line before line N inside the cue. The parser kept the text after it in the cue.` `Repaired` |
| no empty line before a cue | splits the block before the timing line | splits it. `Block #B has no empty line before line N. The parser split the block there.` `Repaired`, once per new part |
| a timing line with a dot between the times, commas before the fractions, or 1 to 4 fraction digits | `Block #B has no timing line on its first line. The line is "Q".` or `The time "T" is not valid.` | reads it, the fraction rounded half up to whole milliseconds. `Block #B has a timing line with other separators or fraction digits than SBV uses. The parser read it.` `Repaired` |
| a first line without a comma, or with more than one, that is no loose timing line | `Block #B has no timing line on its first line. The line is "Q".` | skips the block |
| a half of a timing line that is not `H:MM:SS.mmm`, such as `0:00:61.5` or a note word | `The time "T" is not valid.` | skips the block |

- **Loose timing line**: `H:M:S` with 1 or 2 digit minutes and seconds up to 59, then `.`, `,` or `:` and 1 to 4 fraction digits. A `.` or `,` with optional spaces separates the two times. Example: `0:00:07.9995 , 0:00:11,3` reads as 8 s and 11.3 s.
- **Bound message**: a loose time of 100000 hours or more quotes its digits as written with a dot, such as `99999999999999999999:00:04.0`.
- **Block numbers**: B counts the blocks after the join, from 0.
- **Line numbers**: the cue warnings report the timing line.

## MPL2 and TMPlayer

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| a line that is not a cue, such as a site banner or `[4x][60]Text` | `The line "L" is not an MPL2 cue.` or `The line "L" is not a TMPlayer line.` | skips the line. `Skipped` |
| a time of 100000 hours or more | `The time "[N]" is not below 100000 hours.` for MPL2, `The time "H:MM:SS" is not below 100000 hours.` for TMPlayer, with the digits as written | skips the line. `Skipped` |
| an MPL2 cue that ends before it starts | `The cue ends at E s, before it starts at S s.` | swaps the times when the cue then lasts 30 s or less. `Repaired`. Else skips the line. `Skipped` |

- **Block numbers**: the block of a warning is the index of the line among the non-empty lines, from 0. It counts the lines that fail.
- **Line numbers**: the 1-based line of the file, empty lines included.
- **TMPlayer order**: TMPlayer has no time order error. The reader sorts the entries.

## MPSub

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| no `FORMAT` line before the first timing line | reads the times as seconds, without a warning | the same. `The file has no FORMAT line before line N. The parser read the times as seconds.` `Repaired`, once, with the line and the block of the first timing line |
| a `FORMAT` value that is not `TIME` and has no leading digit | `The FORMAT value "V" is not known.` | keeps the unit before it. `Skipped` |
| a value whose leading digits are all 0, such as `FORMAT=0`, `00` or `0.5` | `The frame rate "V" is not valid.` | keeps the unit before it. `Skipped` |
| a line outside a cue that is no header, comment or timing line | `The line "L" is not a header, a comment or a timing line.` | skips the line and every line up to the next empty line. `Skipped` |
| a negative duration | `The timing line "L" has a negative duration.` | skips up to the next empty line. `Skipped` |
| a timing line without text lines | `The cue has no text lines.` | drops the cue. The position still moves to its end. `Skipped` |
| a time of 100000 hours or more | `The time "L" is not below 100000 hours.` | skips up to the next empty line. `Skipped` |

- **Block numbers**: B counts the timing lines from 0, the bad ones too. A `FORMAT` warning has the count of timing lines before it.
- **Line numbers**: the 1-based line of the file. A cue without text reports its timing line.
- **Quotes**: L is the whole timing line, as PHP does, without the cut at 60 characters.
- **Position**: a timing line that fails does not move the position, so the next cue counts from the end of the last good cue.

## LRC

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| a line that starts with `[` and a digit but holds no valid time tag, such as `[00:1x.00]Text` or `[1:75:00.00]Text` | ignores the line | `The line "L" has a time tag that is not valid.`, with the line and its block. `Skipped` |
| a time of 100000 hours or more, which only a negative offset gives | `The time "[00:12.00]" is not below 100000 hours.`, with the time tag or word time as written and no line | skips the line. `Skipped`, with no line |
| plain text but no cue | `The file has text but no cues.` at the first line of plain text | one warning and an empty subtitle. `Skipped` |

- **Block numbers**: the block of a warning is the index of the line among the non-empty lines, from 0.
- **Line numbers**: the warning of a broken time tag has the 1-based line of the file, empty lines included.
## SubViewer

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| SubViewer 1: a header line that is no `[TAG]value` | `The line "L" is not a SubViewer 1 header tag.` | skips the line. `Skipped`, block 0 |
| SubViewer 2: a line before the first cue that is no tag and no timing line | `The line "L" is neither a header tag nor a timing line.` | skips the line. `Skipped`, block 0 |
| SubViewer 2: a timing line with a time in a loose form: no fraction, 4 fraction digits, or 1-digit hours, minutes or seconds | `The timing line "L" is not in the form hh:mm:ss.ff,hh:mm:ss.ff.` | reads both times, rounded to whole milliseconds. `The timing line "L" is not in the form hh:mm:ss.ff,hh:mm:ss.ff. The parser read it as S s to E s.` `Repaired` |
| SubViewer 2: a timing line with a time that is not valid, such as `00:75:06.00`, a 20-digit hour field or a time that rounds to 100000 hours | the message of the row above | skips the cue. `The timing line "L" has a time that is not valid.` `Skipped` |
| SubViewer 2: a cue that ends before it starts | `The cue ends at E s, before it starts at S s.` | swaps the times when the cue then lasts 30 s or less. `Repaired`. Else skips the cue with the same message. `Skipped` |
| SubViewer 1: a time line with minutes or seconds above 59 | `The time "L" is not valid.` | skips the time line. `Skipped` |
| SubViewer 1: a time plus the delay at or beyond 100000 hours in size | `The time "L with DELAY N" is not below 100000 hours.` L is the time line and N the `[DELAY]` value as read. A delay of 0 quotes the time line alone | skips the time line. `Skipped` |
| SubViewer 1: a cue that a time line ends before it starts | `The cue ends at E s, before it starts at S s.` | as for SubViewer 2 |

- **Skipped cue**: a SubViewer 2 warning for a skipped cue holds the timing line and every text line that followed it. The reader records it when the next timing line or the end of the file comes.
- **Block numbers**: for SubViewer 2, B counts the timing lines from 0, the bad ones too. For SubViewer 1, a time line error has the count of cues read so far, and an order error the index of the cue. A header line has block 0.
- **Line numbers**: the 1-based line of the file. A SubViewer 1 cue reports the line of its start time line.
- **L**: the whole line, as PHP does, without the cut at 60 characters.
- **Bad time line**: after a SubViewer 1 time line that fails, the text line below it is not read as a cue.

## CSV and TSV

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| a quoted cell without a closing quote | `A quoted CSV cell has no closing quote.` at the line where the row starts | ends the cell at the end of its line. `A quoted CSV cell has no closing quote. The cell ends at the end of the line.` `Repaired`, at the line of the quote, with that line |
| rows before the header row, such as an export note | `The table has no column "start" for start.` | drops them. `The table has 2 rows before the header row.` `Skipped`, at the line of the first row, with the dropped rows |
| a time of another shape, such as `soon` | `The time "soon" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.` | skips the row. `Skipped` |
| a time of 100000 hours or more | `The time "T" is not below 100000 hours.` | skips the row. `Skipped` |
| a row that ends before it starts, such as a cut-off last row | `The cue ends at E s, before it starts at S s.` | skips the row. `Skipped` |
| a row of more than 1000 cells | `The table has N columns. The limit is 1000.` | the same error |
| a missing `start` or `text` column | `The table has no column "NAME" for ROLE.` | the same error |

- **Block numbers**: the block of a row warning is the index of the row among the data rows, from 0. The warning holds the cells joined with the delimiter.
- **Line numbers**: the line where the row starts. A quoted line break counts as a line.
- **Both modes**: the synonym warning and the millisecond warning of [formats.md](formats.md#csv-and-tsv) come also in strict mode.

## QuickTime text

| Damage | Strict | Lenient |
|:--- |:--- |:--- |
| the first header line does not start with `{QTtext}` | `The file does not start with {QTtext}.` on line 1 | reads the file as QuickTime text. `The file does not start with {QTtext}. The parser read it as QuickTime text.` `Repaired`, line 1, no block |
| a fraction at or above the time scale, such as `[00:00:01.45]` at `{timeScale:30}` | `The timestamp "[00:00:01.45]" counts 45 units, but the time scale is 30 per second.` | adds the fraction to the seconds: 2.5 s. The same text plus ` The parser added them to the seconds.` `Repaired`, with the timestamp as its line |
| a sample that ends before it starts, which relative or out-of-order timestamps give | `The cue ends at E s, before it starts at S s.` | swaps the times when the cue then lasts 30 s or less. `Repaired`. Else skips the sample. `Skipped` |
| a time of 100000 hours or more, also as a sum of relative timestamps | `The time "T" is not below 100000 hours.`, with the timestamp as written and no brackets | skips the timestamp line and its text lines. `Skipped` |

- **Block numbers**: none. A warning of QuickTime text has `NoBlock`.
- **Line numbers**: the 1-based line of the file. A swapped or skipped sample reports its timestamp line.
- **Skipped timestamp**: the text lines after a skipped timestamp line are skipped too, up to the next timestamp line.

## Tests

[lenient_test.go](../lenient_test.go) reads each fixture of [testdata/lenient](../testdata/lenient) in both modes. The file of the same name in `testdata/lenient/expected` holds the expected cues, warnings and strict result, taken from the PHP tests. A row runs once its format has a reader.
