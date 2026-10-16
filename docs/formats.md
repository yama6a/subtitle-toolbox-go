# Formats

`Format` is a string type. Its value is the format name that the CLI takes for `--from` and `--to`. It is also the key of the format data of that format. Example: `stbox.SubRip` is `"srt"`. `stbox.Formats()` lists the registry. Each `Format` has the methods `Extensions`, `CanRead`, `CanWrite` and `IsAutoDetected`.

| Format | Go constant | Name | Extensions | Section |
|:--- |:--- |:--- |:--- |:--- |
| library JSON | `stbox.JSON` | `json` | `.json` | [JSON](#json) |
| SubRip | `stbox.SubRip` | `srt` | `.srt` | [SubRip](#subrip) |
| WebVTT | `stbox.WebVTT` | `vtt` | `.vtt` | [WebVTT](#webvtt) |

- **Shared extensions**: some formats use the same extension, such as `.sub` for MicroDVD and SubViewer. The first format in the registry owns the extension for `FormatFromPath` and for the extension fallback of `Load`. To read a file as another format, pass `ReadOptions.Format`.
- **Time limit**: every time in every format is below 100000 hours. A reader rejects a time at or above it. A retime that moves a cue past it returns an error that matches `ErrInvalidArgument`. Writers clamp a negative time to 0, except the JSON writer.
- **Options**: each section lists the fields of the format in `ReadOptions` and `WriteOptions`. The fields that every format shares are in [reading-writing.md](reading-writing.md). A reader ignores the fields of other formats.
- **Sections**: each section has what the reader accepts, what the writer writes, the options and the limits. The godoc of the format package holds the reference of each function.

## JSON

Library JSON holds every field of the model, so a trip through it loses nothing. Package `format/stboxjson` reads and writes it. The format name is `json`, the extension `.json`. [json.md](json.md) holds the shape, the warnings and the version.

### Options

`stboxjson.ReadOptions` has no fields. The reader reads `Lenient`. The root `WriteOptions.JSON` holds a `stboxjson.WriteOptions`.

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `PrettyPrint` | `bool` | false: one compact line | indent with 4 spaces and end the output with LF |
| `OmitFormatData` | `bool` | false: write it | leave out `formatData` on the subtitle and on every cue |

- **Shared fields**: `StripTags` has no effect, because library JSON keeps the markup. `BOMDefault` writes no BOM. See [Output rules](reading-writing.md#output-rules).

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

<!-- example: format/webvtt/ExampleWriteWithReport -->
```go
s := &model.Subtitle{}
s.AddCue(model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{`<font color="#123456">Rain</font> <font color="#ffff00">and sun</font>`}})
var out strings.Builder
report, err := webvtt.WriteWithReport(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Print(out.String())
for _, d := range report.DroppedColors {
	fmt.Println(d.Message)
}
// Output:
// WEBVTT
//
// 1
// 00:00:04.000 --> 00:00:06.000
// Rain <c.yellow>and sun</c>
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
