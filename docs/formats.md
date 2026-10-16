# Formats

## WebVTT

WebVTT is the caption format of HTML video and of HLS streams. Package `format/webvtt` reads and writes it. The format name is `vtt`, the extension `.vtt`.

```text
WEBVTT
Kind: captions

NOTE Morning show

intro
00:00:01.000 --> 00:00:04.000 line:0 align:left
<v Anna>Good morning.
```

This file reads as one cue from 1 s to 4 s with the identifier `intro`, the alignment `TopLeft` and the line `<v Anna>Good morning.`. The comment `Morning show` comes before the cue.

### Read

| Input | Goes to |
|:--- |:--- |
| text after `WEBVTT` on line 1 | file data `vtt.header`, trimmed |
| the other lines of the header block | file data `vtt.headerLines`, as written |
| `STYLE` block before the first cue | file data `vtt.styles`, the CSS as written. Not parsed |
| `REGION` block before the first cue | file data `vtt.regions`, one object per region with the settings `id`, `width`, `lines`, `regionanchor`, `viewportanchor` and `scroll` |
| `NOTE` block | a comment before the next cue |
| cue identifier | `Cue.ID` |
| cue settings `vertical`, `line`, `position`, `size`, `align`, `region` | cue data `vtt`, values as written, in file order. `vtt.alignment` holds the alignment read from them |
| `&nbsp;`, `&lrm;`, `&rlm;` | the characters U+00A0, U+200E and U+200F |
| `&` that starts no character reference, as in `Rye & spelt` or `&foo;` | text: the line holds `&amp;` |
| a start tag without attributes, such as `<today>`, whose end tag is not in the cue | text: the line holds `&lt;today&gt;`. FFmpeg writes such text from ASS |
| an inline timestamp without hours, such as `<00:03.000>` | the word timestamp `<00:00:03.000>`, so retiming moves it |

- **Signature**: the content starts with `WEBVTT`, then a space, a tab, a line end or the end. `WEBVTTX` is no signature. `Detect` uses the same rule.
- **Alignment from cue settings**: `line:0`, `line:0%` and `line:0,start` give the top row. `line:50%,center` gives the middle row. No `line`, `line:-1` and `line:100%,end` give the bottom row. `align:left`, `center` and `right` set the column. Other values, such as `align:start` or `line:85%`, and every cue with `vertical` give `BottomCenter` and no `vtt.alignment`.
- **Settings without a space**: `00:01.000line:40%` reads the end `00:01.000` and the setting `line:40%`.
- **Hours**: hours of any number of digits read, such as `0:00:00.800` and `0010:00:40.750`. A time without hours has exactly 2 minute digits. Every time must be below 100000 hours.
- **Lines of white space**: only an empty line ends a cue. A line of spaces or tabs inside a cue is an empty text line, which the cue drops. YouTube auto captions have such a line in each cue.
- **Cues without text**: a timing line without text gives a cue without lines.
- **YouTube auto captions**: yt-dlp `--write-auto-subs` saves rolling cues. Each cue repeats the line before it above a new line with `<c>` word timestamps, and a 10 ms cue repeats the new line. The reader drops the 10 ms cues and the repeated lines, so each spoken line becomes one cue. The next cue starts where the dropped 10 ms cue started. A file without `<c>` word timestamps stays as it is.
- **Chapters and metadata**: a chapters file is a WebVTT file with one cue per chapter, and the cue text is the title. A metadata cue reads as text lines. JSON payloads are not parsed.
- **NUL**: each NUL becomes U+FFFD in both modes, as the spec says.

### Write

- **Order**: the header, every `REGION` block, every `STYLE` block, then each cue with the comments before it, then the comments after the last cue. So a `NOTE` before a `STYLE` block in the input comes out after it.
- **Identifiers**: a cue without an identifier, or with one that holds `-->`, LF or CR, gets its cue number from 1.
- **Times**: always with hours, at least 2 hour digits. A negative time is written as 0.
- **Settings**: the stored settings of the 6 names come back in stored order while the cue alignment is unchanged. After a change of the alignment, the writer keeps `vertical`, `size` and `region` and writes the settings of the new alignment: `line:0` or `line:50%,center` for the row, `align:left` or `align:right` for the column. See [format-data.md](format-data.md#an-explicit-alignment-wins-over-a-stored-position).
- **Tags**: the writer keeps `<b>`, `<i>`, `<u>`, `<strong>`, `<v>`, `<lang>`, `<c>`, `<ruby>` and `<rt>`, their class forms such as `<c.yellow>` or `<v.first.loud Fred>`, and inline timestamps. It drops every other tag and keeps its text. `StripTags` drops every tag and the inline timestamps.
- **Comments**: the writer replaces `-->` in a comment with `->` until none is left, trims each line and drops the empty lines, because an empty line would end the `NOTE` block. A comment of one line is written as `NOTE text`, a longer one starts on the line after `NOTE`.
- **Arrow in text**: `-->` in cue text is written as `--&gt;`, because a raw `-->` would start a new cue.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: `<s>`, colors without a WebVTT class, font faces, unknown tags, the place of comments next to `STYLE` and `REGION` blocks, empty lines inside cue text, the line layout of `NOTE` blocks, settings with other names, and the layout of the header text.

### Dropped colors

The writer turns `<font color>` into a [color class](markup.md#colors) when the color is one of the 8 WebVTT colors, so `#FFFF00`, `#ff0` and `yellow` all become `<c.yellow>`. WebVTT has no class for other colors, so the writer drops them. `webvtt.WriteWithReport` returns the output and one `DroppedColor` per dropped color. `stbox.Write` returns no report.

```go
s := &model.Subtitle{}
s.AddCue(model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{`<font color="#123456">Rain</font> <font color="#ffff00">and sun</font>`}})
var out strings.Builder
report, _ := webvtt.WriteWithReport(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
fmt.Println(report.DroppedColors[0].Message)
// Cue #0 at 4 s: dropped the color "#123456", because WebVTT has classes for 8 colors only.
```

### Options

`webvtt.ReadOptions` and `webvtt.WriteOptions` have no fields. The reader reads `Lenient`. The writer reads `StripTags`, `LineEnding` and `BOM`.

## SubRip

SubRip is the most common text subtitle format. Package `format/srt` reads and writes it. The format name is `srt`, the extension `.srt`.

```text
1
00:00:01,500 --> 00:00:04,000 X1:10 X2:20 Y1:30 Y2:40
{\an8}<i>Fish & Chips</i>
```

This block reads as one cue from 1.5 s to 4 s with the alignment `TopCenter`, the line `<i>Fish &amp; Chips</i>` and the coordinates in the cue data `srt.coordinates`. The writer writes back the same 3 lines.

### Read

| Input | Goes to |
|:--- |:--- |
| the cue number line | dropped. It must be a PHP numeric string, such as `1`, `+1`, `1.5` or `1e3` |
| `X1:a X2:b Y1:c Y2:d` after the end time | cue data `srt.coordinates`, 4 integers. A value above the int64 range becomes the int64 maximum |
| `vertical`, `line`, `position`, `size`, `align` after the end time, as yt-dlp writes them | cue data `vtt`, values as written. `vtt.alignment` holds the numpad value of an `\an` or `\a` tag of the cue |
| `<b>`, `<i>`, `<u>`, `<s>`, `<font>`, `<v>`, and tags whose attributes are all `name=value` pairs | markup |
| `&`, `<` and `>` in other text, also in `&amp;` and in `<a sentence in brackets>` | text. Players show `&amp;` as typed, so the line holds `&amp;amp;` |
| a start tag without attributes, such as `<today>`, whose end tag is not in the cue | text: `&lt;today&gt;` |
| the ASS tags `{\an1}` to `{\an9}`, and the SSA tags `{\a1}` to `{\a11}` with a legacy code | the cue alignment. The first tag of the cue wins |
| the ASS toggles `{\b1}`, `{\i1}`, `{\u1}`, `{\s1}` and their `0` forms | `<b>`, `<i>`, `<u>`, `<s>` and their end tags. Tags still open at the end of the cue close there |
| other ASS tags, such as `{\pos(1,2)}` | text, in one brace block after the converted tags |

- **Times**: `h:mm:ss,mmm` with 1 to 3 hour digits. The fraction has 1 to 3 digits after `,` or `.`, or exactly 3 after `:`. A time without fraction reads, so `00:00:01` is 1 s.
- **Blocks**: an empty line ends a block, also in strict mode. A block of text after the block of a timing line joins that cue. A block with 2 timing lines splits before the second, and the cue number line above it goes with it.
- **Joined files**: each line loses the UTF-8 BOMs at its start, so 2 files joined into one read as one file.
- **Invalid UTF-8** stays as bytes and comes back unchanged on write.

### Write

- **Layout**: the cue number from 1, the timing line, the text lines. One empty line between cues. The output ends with one LF.
- **Times**: `hh:mm:ss,mmm`, more hour digits from 100 hours. A negative time writes as 0.
- **Tags**: the writer keeps `<b>`, `<i>`, `<u>`, `<s>` and `<font>`. The 8 WebVTT color classes become `<font color>`, ruby becomes `base (text)`, and other tags and word timestamps go. `StripTags` drops every tag. Entities are decoded, so `&amp;` writes `&`.
- **Alignment**: an alignment other than `BottomCenter` writes `{\anN}` before the first line.
- **Empty lines**: a line that is blank after the tags go drops out, because an empty line ends a SubRip cue. A cue without text writes its number and timing line only.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: cue numbers, comments, `<v>` speakers, colors outside the 8 classes of `<c>`, `<lang>`, word timestamps, the ruby structure, WebVTT settings, all format data except the coordinates, and empty lines inside a cue.

### Options

`srt.ReadOptions` and `srt.WriteOptions` have no fields. The reader reads `Lenient`. The writer reads `StripTags`, `LineEnding` and `BOM`.

## SBV

SBV is the caption format of YouTube Studio. Package `format/sbv` reads and writes it. The format name is `sbv`, the extension `.sbv`.

```text
0:00:01.500,0:00:04.000
Where are you going?

0:00:04.500,0:00:06.000
Home.
```

The first block reads as one cue from 1.5 s to 4 s. SBV has no markup, so `<i>` in an SBV file is text and reads as `&lt;i&gt;`.

### Read

- **Blocks**: an empty line ends a block. Several empty lines count as one.
- **Timing line**: line 1 of a block, `H:MM:SS.mmm,H:MM:SS.mmm`. The hour field has any number of digits. The reader splits the line at its only comma and trims both halves. A time of 100000 hours or more fails.
- **Text**: every other line of the block, with `&`, `<` and `>` escaped. A timing line without text gives a cue with no lines.
- **Timing line inside a block**: starts a new cue.
- **Text block after an empty line**: joins the cue before it, when that cue has a timing line. A block whose first line looks like a broken time, such as `1:00` or `-->`, does not join, so the reader reports it.
- **Invalid UTF-8** stays as bytes. Lenient mode adds one `Repaired` warning.
- **Format data**: none.

| Timing line | Strict | Lenient |
|:--- |:--- |:--- |
| `0:00:07.980,0:00:11.300` | reads | reads |
| `0:00:07.980 , 0:00:11.300` | reads | reads |
| `0:00:07.980.0:00:11.300` | `Block #B has no timing line on its first line. The line is "Q".` | reads, warns |
| `0:00:07,980,0:00:11,300` | `Block #B has no timing line on its first line. The line is "Q".` | reads, warns |
| `0:00:07.98,0:00:11.3` | `The time "0:00:07.98" is not valid.` | reads, warns |

### Write

- **Layout**: the timing line, then the text lines. One empty line between cues. The output ends with one LF.
- **Times**: `H:MM:SS.mmm`, with 1 hour digit below 10 hours. A negative time writes as 0.
- **Text**: ruby becomes `base (annotation)`. All tags go, also `<v>` and word timestamps, and entities are decoded. Each line is trimmed and empty lines drop out.
- **Skipped cues**: a cue without text after that, because an empty line would end the SBV cue.
- **BOM**: none by default. `StripTags` changes nothing.
- **Lost on write**: markup, speakers, word timestamps, alignment, identifiers, comments and cues without text.
- **Round trip**: `&lt;i&gt;` writes as `<i>` and reads back as `&lt;i&gt;`. A file with `<`, `>` and `&` in the text comes back byte for byte.

### Options

`sbv.ReadOptions` and `sbv.WriteOptions` have no fields. The reader reads `Lenient`. The writer reads `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#sbv).
