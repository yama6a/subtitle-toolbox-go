# Formats

`Format` is a string type. Its value is the format name that the CLI takes for `--from` and `--to`. It is also the key of the format data of that format. Example: `stbox.SubRip` is `"srt"`. `stbox.Formats()` lists the registry. Each `Format` has the methods `Extensions`, `CanRead`, `CanWrite` and `IsAutoDetected`.

| Format | Go constant | Name | Extensions | Section |
|:--- |:--- |:--- |:--- |:--- |
| ASS and SSA | `stbox.ASS` | `ass` | `.ass`, `.ssa` | [ASS and SSA](#ass-and-ssa) |
| library JSON | `stbox.JSON` | `json` | `.json` | [JSON](#json) |
| LRC | `stbox.LRC` | `lrc` | `.lrc` | [LRC](#lrc) |
| MicroDVD | `stbox.MicroDVD` | `microdvd` | `.sub` | [MicroDVD](#microdvd) |
| MPSub | `stbox.MPSub` | `mpsub` | `.mpsub` | [MPSub](#mpsub) |
| SAMI | `stbox.SAMI` | `sami` | `.smi`, `.sami` | [SAMI](#sami) |
| SBV | `stbox.SBV` | `sbv` | `.sbv` | [SBV](#sbv) |
| SubRip | `stbox.SubRip` | `srt` | `.srt` | [SubRip](#subrip) |
| TTML, IMSC, DFXP | `stbox.TTML` | `ttml` | `.ttml`, `.dfxp`, `.xml` | [TTML](#ttml) |
| plain text | `stbox.PlainText` | `txt` | `.txt` | [Plain text](#plain-text) |
| MPL2 | `stbox.MPL2` | `mpl2` | `.txt` | [MPL2](#mpl2) |
| TMPlayer, TMPlayer+ | `stbox.TMPlayer` | `tmplayer` | `.txt` | [TMPlayer](#tmplayer) |
| WebVTT | `stbox.WebVTT` | `vtt` | `.vtt` | [WebVTT](#webvtt) |

- **Shared extensions**: some formats use the same extension, such as `.sub` for MicroDVD and SubViewer. The first format in the registry owns the extension for `FormatFromPath` and for the extension fallback of `Load`. MicroDVD owns `.sub`, so `Load` finds MPSub files named `.sub` only by their content. Plain text owns `.txt`, so `Load` finds MPL2 and TMPlayer files only by their content. To read a file as another format, pass `ReadOptions.Format`.
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
- **Times**: `hh:mm:ss,mmm`, with 3 hour digits from 100 hours. A negative time writes as 0. A time of 1000 hours or more writes as `999:59:59,999`, because the reader takes at most 3 hour digits.
- **Tags**: the writer keeps `<b>`, `<i>`, `<u>`, `<s>` and `<font>`. The 8 WebVTT color classes become `<font color>`, ruby becomes `base (text)`, and other tags and word timestamps go. `StripTags` drops every tag. Entities are decoded, so `&amp;` writes `&`.
- **Alignment**: an alignment other than `BottomCenter` writes `{\anN}` before the first line.
- **Empty lines**: a line that is blank after the tags go drops out, because an empty line ends a SubRip cue. A cue without text writes its number and timing line only.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: cue numbers, comments, `<v>` speakers, colors outside the 8 classes of `<c>`, `<lang>`, word timestamps, the ruby structure, WebVTT settings, all format data except the coordinates, and empty lines inside a cue.

### Options

`srt.ReadOptions` and `srt.WriteOptions` have no fields. The reader reads `Lenient`. The writer reads `StripTags`, `LineEnding` and `BOM`.

## ASS and SSA

ASS (Advanced SubStation Alpha) is the styled subtitle format of Aegisub, libass and most anime releases. SSA (SubStation Alpha v4.00) is its older form. Package `format/ass` reads and writes both. The format name is `ass`, the extensions `.ass` and `.ssa`.

```text
[V4+ Styles]
Format: Name, Italic, Alignment
Style: Thoughts,-1,2

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.50,0:00:04.00,Thoughts,Anna,0,0,0,,{\an8}Not {\b1}again{\b0}
```

This file reads as one cue from 1.5 s to 4 s with the alignment `TopCenter` and the line `<v Anna><i>Not <b>again</b></i>`. Written back without a change, the event keeps its `Text` field as stored, so tags such as `\pos`, `\fad` and drawings survive a retiming.

- **Dialects**: a file is SSA when its `ScriptType` is `v4.00` or its styles section is `[V4 Styles]`. A subtitle read from SSA writes SSA, also to a `.ass` file. Cues of other formats write ASS, also to a `.ssa` file.

### Read

| Input | Goes to |
|:--- |:--- |
| `Title:` in `[Script Info]` | the metadata `title` |
| the other `Key: Value` lines and the `;` lines of `[Script Info]` | file data `ass.scriptInfo` and `ass.scriptInfoComments`. A repeated key keeps the last value at the place of the first |
| the styles section, its `Format:` line and its `Style:` lines | file data `ass.stylesSection`, `ass.styleFormat` and `ass.styles` |
| the `Format:` line of `[Events]` | file data `ass.eventFormat`. Without it, the default ASS fields, or `Marked` in place of `Layer` for SSA |
| a `Dialogue:` event | a cue. Its fields except `Start`, `End` and `Text` go to cue data `ass.fields`, the `Text` field to `ass.text` |
| a `Comment:` event | a comment before the next `Dialogue:` event, with the raw `Text`, and file data `ass.commentEvents` |
| every other section, such as `[Fonts]` or `[Aegisub Project Garbage]` | file data `ass.sections`, its trimmed lines by section name |
| the section order | file data `ass.sectionOrder` |
| the `Name` field | the speaker: a `<v Name>` tag before the first line |

| Override tag | Core markup |
|:--- |:--- |
| `\an1` to `\an9`, SSA `\a1` to `\a11` | the cue alignment. The first tag of the event wins, else the style alignment, else `BottomCenter` |
| `\b1`, `\i1`, `\u1`, `\s1`, and `0` or no value | `<b>`, `<i>`, `<u>`, `<s>` and their end tags. `\b700` is dropped |
| `\c&HBBGGRR&` or `\1c&HBBGGRR&` with 1 to 8 hex digits, and `\c` alone | `<font color="#rrggbb">` and `</font>` |
| `\k`, `\K`, `\kf`, `\ko` in centiseconds | a word timestamp at the karaoke time. The first syllable starts at the event start |
| `\r` and `\rName` | all tags close, then the flags of the event style or of style `Name` open |
| `\p1` to `\p0` | a drawing: its text is dropped |
| other tags such as `\pos`, `\fad`, `\t`, `\fs`, and `{note}` blocks | dropped |

- **Times**: `h:mm:ss.cc` with 1 to 3 fraction digits, so `0:00:01.5` is 1.5 s. Minutes and seconds of 60 or more fail.
- **Styles**: a style name ignores a leading `*`, the last style of a name wins, and `Default` matches in any case. An event with an unknown style takes the `Default` style. The style flags `Bold`, `Italic`, `Underline` and `StrikeOut` of `-1` or `1` open their tags at the start of each cue.
- **Text**: `\N` is a line break, `\n` a space or a line break with `WrapStyle: 2`, `\h` the hard space U+00A0. `&`, `<` and `>` are text, so the line holds `&amp;`, `&lt;` and `&gt;`.
- **Nesting**: a tag that changes an open tag closes the tags opened after it and opens them again, so the tags always nest.
- **Missing sections**: a file without `[Events]` and without `[Script Info]` fails in both modes. `[Script Info]` alone gives an empty subtitle.
- **Field names that are numbers**: a `Format:` line of `[Events]` with a field name such as `1` fails, because cue data `ass.fields` cannot hold such a key.
- **Invalid UTF-8** stays as bytes, also in the `Name` field.

### Write

- **File data**: the writer writes the stored `ass` file data. An absent key takes the default below. A present key keeps its value, also when it is null or empty.

| Key | Default |
|:--- |:--- |
| `sectionOrder` | `Script Info`, `V4+ Styles`, `Events` |
| `scriptInfo` | `ScriptType: v4.00+`, `PlayResX: 384`, `PlayResY: 288`, `ScaledBorderAndShadow: yes` |
| `stylesSection` | `V4+ Styles` |
| `styleFormat` and `styles` | the 23 ASS style fields and one `Default` style: `Default,Arial,16,&H00FFFFFF,&H00FFFFFF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,1,0,2,10,10,10,1` |
| `eventFormat` | the ASS event fields |

- **Header**: cues of another format get a header in the shape of the FFmpeg header, with the defaults above. The `Title:` line comes from the title metadata, after the `;` comments.
- **Sections**: in stored order. `[Script Info]` goes first and `[Events]` last when the order lacks them. One empty line between sections.
- **Events**: in time order, so an unsorted input changes order. Each comment writes its stored `Comment:` event, or a new one at the start of the next cue.
- **Times**: `h:mm:ss.cc`, centiseconds rounded half away from zero. A negative time writes as 0.
- **Unchanged cue**: a cue whose lines and alignment equal the stored `ass.lines` and `ass.alignment` writes its stored `Text` and `Name` fields. `StripTags` converts every cue.
- **Changed cue**: the writer converts the core markup to override tags: `\an` (or the SSA `\a` code) for an alignment other than that of the event style, `\b1` and `\b0` and the other toggles, `\c&HBBGGRR&` for a `<font>` color or a WebVTT color class, and karaoke tags for word timestamps. A pair such as `<i>` around all text whose flag the style sets is left out. The speaker of the first `<v>` tag goes to the `Name` field.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: in changed cues every override tag outside `\b \i \u \s \c \an \a \k`, `<v>` beyond the first speaker, `<c>` classes other than the 8 colors, `<lang>`, ruby structure. The event order. The place of `[Script Info]` comments. Empty lines and leading white space in other sections. Text with `{`, `}` or `\` is written raw, because ASS has no escape for them, so it reads back as override tags or line breaks.

### Options

`ass.ReadOptions` has no fields. The reader reads `Lenient`. The root `WriteOptions.ASS` holds an `ass.WriteOptions`. The writer also reads `StripTags`, `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#ass-and-ssa).

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `KaraokeTag` | `ass.KaraokeTag` | `""`: `\k` | the tag of a karaoke syllable in a converted cue: `KaraokeInstant` (`k`), `KaraokeFill` (`kf`) or `KaraokeOutline` (`ko`) |
| `Style` | `string` | `""`: keep the styles | comma-separated `Field=Value` pairs, as FFmpeg `force_style`, that change every style named `Default` |

- **Style check**: `Validate` rejects a pair without `=` and a field outside the 22 ASS style fields without `Name`, with an error that matches `ErrInvalidArgument`. A later pair for the same field wins.
- **Style at write time**: a field that the `Format:` line of the styles lacks, such as `Underline` in SSA, fails with an `*UnwritableError`. A file without a `Default` style gets one first. A file without a styles section gets one before `[Events]`.
- **Cue tags**: the writer computes the cue tags against the styles before the option. So `Italic=-1` makes every cue without `<i>` italic, with no `{\i0}`.

## TTML

TTML is the W3C XML format for timed text. IMSC is its profile for streaming and broadcast, and DFXP is its draft form that Netflix deliveries and Flash players use. Package `format/ttml` reads and writes all 3. The format name is `ttml`, the extensions `.ttml`, `.dfxp` and `.xml`. iTT files read as TTML.

```xml
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en">
  <head><layout><region xml:id="top" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/></layout></head>
  <body><div><p begin="00:00:01.000" end="00:00:03.000" region="top">Hello<br/><span tts:fontStyle="italic">world</span></p></div></body>
</tt>
```

This file reads as one cue from 1 s to 3 s with the alignment `TopCenter` and the lines `Hello` and `<i>world</i>`. The metadata `language` is `en`, and the file data `ttml.head` holds the `<head>` as XML text.

### Read

| Input | Goes to |
|:--- |:--- |
| the root namespace: TTML, DFXP `2006/10`, DFXP `2006/04`, or none | file data `ttml.namespace`, `""` for none. Another root gives `The root element is not a TTML <tt> element.` |
| the `xmlns` declarations on `<tt>` | file data `ttml.namespaces`, a `*model.Data` of prefix to URI, sorted by prefix, `""` for the default namespace. An empty list `[]any{}` when the root declares none, as PHP writes it |
| the other attributes of `<tt>` except `xml:lang` | file data `ttml.attributes`, a `*model.Data` in file order. An empty list `[]any{}` when the root has none, as PHP writes it |
| `xml:lang` of `<tt>` | the metadata `language` |
| the first `ttm:title` in `<head>` | the metadata `title`, trimmed. The reader removes the element from the stored head |
| `<head>` | file data `ttml.head`, the XML text as libxml writes it, so it equals the head that the PHP library stores |
| the attributes of `<body>` except `begin`, `end` and `dur` | file data `ttml.body` |
| each `<p>` | a cue. `xml:id` gives `Cue.ID` |
| the attributes of the `<p>` except the times, `xml:id` and `ttm:agent` | cue data `ttml.attributes`. An inherited `region` is added |
| the attributes of the `<div>` elements around the `<p>` | cue data `ttml.div`, without the times, `xml:id` and `region`. An inner value replaces an outer one |
| `<br/>`, and LF with `xml:space="preserve"` | a line break. Other runs of white space become 1 space |
| `tts:fontWeight`, `tts:fontStyle`, `tts:textDecoration`, `tts:color` of the region, `<body>`, `<div>`, `<p>` and `<span>`, and of the styles they reference | `<b>`, `<i>`, `<u>`, `<s>` and `<font color>`. White and the alpha channel give no markup |
| `ttm:agent` | `<v Name>`, with the name of the agent element in `<head>`, else the agent ID |
| a `<span>` with `begin`, `end` or `dur` | a word timestamp before its text, when it lies inside the cue |
| `itts:forcedDisplay="true"` on the `<p>`, its container or its region, or on one `<span>` | `Cue.Forced` |
| other elements, such as `<set>` and `<metadata>` | skipped with their text |

- **Times**: clock times such as `00:00:01.500`, `00:00:01,500` and `00:00:01:12` with frames, and offsets such as `1.5s`, `1500ms`, `36f` and `15000000t`. The reader reads `ttp:frameRate`, `ttp:frameRateMultiplier`, `ttp:subFrameRate`, `ttp:tickRate`, and `ttp:timeBase="smpte"` with `ttp:dropMode`. Without `ttp:frameRate`, frames count at 30 fps.
- **Timing model**: a time adds to the begin of its parent. A paragraph ends at the earliest of its parent end, its `end` and its begin plus `dur`. A paragraph without an end takes the times of its timed spans. A paragraph without an end and without timed spans fails. `seq` containers read as `par`.
- **Arithmetic**: the reader computes in float64 seconds as PHP does, then rounds each cue time to whole milliseconds. So `smpte_drop_ntsc.ttml` gives starts such as 57.391 s and 3599.996 s.
- **End before begin**: `<p begin="5s" end="2s">` fails with `The cue ends at 2 s, before it starts at 5 s.` Lenient mode swaps the times, as in SubRip.
- **Alignment**: `tts:textAlign` `left`, `center` or `right` gives the column. The `tts:origin`, `tts:extent` and `tts:displayAlign` of the region give the row, in `%`, `px` or cells. A file without regions has one region over the whole screen from the top, so `tts:textAlign="center"` alone gives `TopCenter`. `start`, `end` and an unknown region give `BottomCenter` and no `ttml.alignment`.
- **Namespaces**: `tts:`, `ttp:` and `ttm:` attributes count in the namespaces of TTML and of both DFXP drafts, and with an undeclared prefix.
- **Encoding**: the reader reads the content as UTF-8 and ignores the `encoding` of the XML declaration, because the root decodes other encodings first.
- **Invalid UTF-8** fails in strict mode. Lenient mode reads each bad byte as U+FFFD.
- **No DTD**: the reader loads no DTD, no external entity and nothing from the network. An entity that a DOCTYPE declares is unknown, so `&xxe;` makes the file not well-formed.

### Write

The writer keeps what the reader stored, so a DFXP file stays DFXP after a retiming. A SubRip cue from 1 s to 3 s with the line `{\an8}<i>Hello</i>` writes as:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="">
  <head>
    <layout>
      <region xml:id="topCenter" tts:origin="10% 10%" tts:extent="80% 80%" tts:displayAlign="before" tts:textAlign="center"/>
    </layout>
  </head>
  <body>
    <div>
      <p begin="00:00:01.000" end="00:00:03.000" region="topCenter"><span tts:fontStyle="italic">Hello</span></p>
    </div>
  </body>
</tt>
```

| Output | Comes from |
|:--- |:--- |
| the root namespace | `ttml.namespace`, else TTML. A file without a namespace comes out in the TTML namespace |
| the `xmlns` declarations, sorted by prefix | `ttml.namespaces`, an object or an empty list. The writer binds `tts` and `ttm` when no prefix is bound to the styling and metadata namespaces of the dialect. A new forced flag binds `itts`. A taken prefix gets a number: `tts2` |
| `xml:lang` | the metadata `language`, else `""` |
| the attributes of `<tt>`, `<body>` and `<div>` | `ttml.attributes`, `ttml.body` and the cue data `ttml.div`. Consecutive cues with the same `div` attributes share one `<div>` |
| `<head>` | `ttml.head`, else `<head/>`, with the title metadata as a new first `ttm:title` |
| each `<p>` | a cue: `xml:id` from `Cue.ID`, media clock times such as `00:00:01.500`, then the stored attributes of the cue data `ttml.attributes` |
| `<span>` with inline `tts:` styles | the core markup: `<b>`, `<i>`, `<u>`, `<s>`, `<font color>`, the 8 WebVTT color classes. A `<v Name>` before all text gives `ttm:agent` on the `<p>`, other `<v>` tags give a `<span ttm:agent>` |
| `ttm:agent` elements in the head | each new speaker name, with the ID `agent1`, `agent2` and so on. An agent of the head with the same name keeps its ID |

- **Attributes that are lost**: an attribute whose prefix the output does not declare, such as a prefix that the input declared on the `<p>` itself. On `<tt>`, `ttp:timeBase`, `ttp:clockMode`, `ttp:dropMode` and `ttp:markerMode` whatever their value, because the output holds media times. `ttp:frameRate` and `ttp:tickRate` stay.
- **Regions from alignments**: a cue without a stored region gets the region of its alignment. A `BottomCenter` cue gets none, unless it was realigned or the subtitle has no `ttml` data. The writer adds each region once, at the end of the last `<layout>` of the head. It has the shape of the IMSC 1.1 examples with a safe area of 10%: `tts:origin="10% 10%"`, `tts:extent="80% 80%"`, and the `tts:displayAlign` and `tts:textAlign` of the row and the column. Its ID is the alignment name in lower camel case, such as `bottomCenter`.
- **Realigned cue**: a cue whose alignment differs from the stored `ttml.alignment` loses its stored `region`, as [format-data.md](format-data.md#an-explicit-alignment-wins-over-a-stored-position) says. When the styles of `<body>`, `<div>` or `<p>` set another `tts:textAlign` than the new column, the `<p>` gets `tts:textAlign` of the new column.
- **Forced flag**: the `<p>`, the `<div>` or the region can give the forced flag already. The writer writes `Cue.Forced` only when it differs from that flag. A stored attribute on the `<p>` changes in place, else a new `itts:forcedDisplay` comes last.
- **Resets**: core markup has no tag that turns an inherited style off. The styles of the region, `<body>`, `<div>` and `<p>` can make text bold, italic, underlined, struck or colored. When no open span repeats such a style for some text, the whole content goes inside one `<span>` that turns it off. The span holds only the needed attributes of `tts:fontWeight="normal"`, `tts:fontStyle="normal"`, `tts:textDecoration="noUnderline noLineThrough"` and `tts:color="white"`.
- **Unique IDs**: each `xml:id` of the output is unique. An ID that is taken gets `_2`, `_3` and so on. The head IDs keep their value. New agents and regions take their IDs next, then `<tt>`, `<body>`, each `<div>` and each `<p>` in output order. So 2 cues with the ID `c1` come out as `c1` and `c1_2`. A cue ID that is no XML name, such as `1`, gives no `xml:id`.
- **Forbidden characters**: the writer removes the characters that XML 1.0 forbids from text and attribute values: the C0 controls except tab, LF and CR, U+FFFE and U+FFFF. Text that is not valid UTF-8 returns an `*UnwritableError` whose `Err` is `ErrInvalidUTF8`.
- **Attribute values**: the writer writes TAB, LF and CR as `&#9;`, `&#10;` and `&#13;`, because a raw one reads back as a space.
- **Stored head that is no head**: a `ttml.head` that is not well-formed XML, or whose first node is no `<head>` element, returns an `*UnwritableError` with `The stored TTML head is not a well-formed <head> element.`.
- **Lost on write**: word timestamps and timed spans. Span style references, because spans are written inline. Styles that core markup does not hold: a background color of the input, fonts, sizes, positions. `<set>`, comments and `<metadata>` in the body. The `<div>` tree, div IDs and div regions. The time format: the output holds media clock times in whole milliseconds.

### Options

`ttml.ReadOptions` and `ttml.WriteOptions` have no fields, so the root `WriteOptions` has no TTML field. The reader reads `Lenient`. The writer reads `StripTags`, `LineEnding` and `BOM`. By default the output has no BOM. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#ttml).

## SAMI

SAMI is the HTML-like caption format of Windows Media Player, still common for Korean subtitles. Package `format/sami` reads and writes it. The format name is `sami`, the extensions `.smi` and `.sami`.

```text
<SAMI>
<HEAD>
<TITLE>Station news</TITLE>
<STYLE TYPE="text/css"><!--
.ENUSCC { Name: English; lang: en-US; }
--></STYLE>
</HEAD>
<BODY>
<SYNC Start=1000><P Class=ENUSCC>The bridge is <b>open</b><br>again.
<SYNC Start=4000><P Class=ENUSCC>&nbsp;
</BODY>
</SAMI>
```

This file reads as one cue from 1 s to 4 s with the lines `The bridge is <b>open</b>` and `again.`, the title `Station news` and the language `en-US`. A file can hold several languages, one CSS class each. A read gives the cues of one class.

### Read

| Input | Goes to |
|:--- |:--- |
| `<TITLE>`, trimmed and with entities decoded | metadata `title` |
| the `lang` of the chosen class in `<STYLE>` | metadata `language` |
| the raw text of `<SAMIParam>` and of `<STYLE>`, the chosen class as written | file data `sami.samiParam`, `sami.style`, `sami.class` |
| `Start` of each `SYNC` tag, in milliseconds | the cue start. A negative `Start` reads as 0 |
| `<b>`, `<i>`, `<u>`, `<s>`, `<strike>` | markup. `<strike>` becomes `<s>` |
| `<font color>` with `#rrggbb`, `rrggbb` or one of the 16 HTML 4.01 color names | `<font color="#rrggbb">`. Another color, such as `#ABC`, keeps only the text |
| `<br>` | a new line. Other line ends and runs of white space become 1 space |
| other tags and comments | dropped, their text stays |
| the paragraphs of the class in each `SYNC` block | cue data `sami.paragraphs`, each with its other attributes and its HTML, and `sami.lines` |

- **Cues**: a `SYNC` block with a paragraph of the class, or with a paragraph without class, ends the cue before it. It starts a cue when those paragraphs hold text. So a `&nbsp;` block ends a cue. A block without `<P>` ends the cue of every class. The blocks sort by start.
- **Last cue**: it lasts `LastCueDuration`, 5 s by default. Its end stops at 99999:59:59.999, as [reading-writing.md](reading-writing.md#last-cue-default) says.
- **`&nbsp` without `;`** reads as a no-break space, as browsers read it.
- **Invalid UTF-8** fails in strict mode. Lenient mode reads each bad byte as U+FFFD. Korean CP949 and EUC-KR files need `ReadOptions.Encoding` when the detection of [encodings.md](encodings.md) misses them.

### Class choice

| # | Case | Class |
|---:|:--- |:--- |
| 1 | `sami.ReadOptions.LanguageClass` is set | that class of `<STYLE>` or of a `<P>`, in any case. Missing: `The SAMI file has no class X.`, also in lenient mode |
| 2 | `<STYLE>` has no class | the first class that a `<P>` uses, or none |
| 3 | no `<P>` uses a class | the first class of `<STYLE>` |
| 4 | a `<P>` uses a class of `<STYLE>` | the first such class in `<STYLE>` order |
| 5 | no `<P>` class is in `<STYLE>` | the first class that a `<P>` uses. Lenient mode warns |

Without a class, every paragraph counts.

### Tag scanner

The reader parses the content of each `SYNC` block with a tag scanner of its own, in place of the libxml2 HTML parser of PHP. The cue data `sami.paragraphs` holds the HTML that the scanner writes back. So the scanner gives the libxml2 tree for the markup of real SAMI files:

- A `<P>` closes an open `<P>` and the inline tags `b`, `i`, `u`, `s`, `strike`, `tt`, `big`, `small` and `h1` to `h6` before it, so 2 paragraphs of one block are siblings. It does not close `<font>` or `<span>`.
- An end tag closes the innermost open element of its name and every element after it. An end tag without such an element is ignored.
- The named entities of HTML 4.01 and `&apos;` decode. An unknown name, or a name without `;`, stays text. A numeric reference such as `&#39` decodes also without `;`.

**Limit**: lists, tables, forms and scripts follow more libxml2 rules than the scanner knows. Such content keeps its text, but the cue data can differ from PHP. SAMI files do not use them.

### Write

- **Class**: the stored `sami.class`, or none when only `sami.style` is stored. Else the letters and digits of the language in upper case plus `CC`, so `ko-KR` writes `KOKRCC`. Without a language: `SUBTTL`.
- **Head**: `<TITLE>` with the title, `<SAMIParam>` when stored, and `<STYLE>`. A stored style loses the rules of every other class, because a player shows the first class by default. Without a stored style, the writer writes a style with one class.
- **Cues**: one `SYNC` line per cue, and a `&nbsp;` line at the end of a cue when the next cue starts later. An overlapping cue ends where the next one starts. A negative time writes as 0.
- **Unchanged cue**: a cue whose lines equal the cue data `sami.lines` writes its stored paragraphs. `StripTags` turns this off.
- **Changed cue**: ruby becomes `base (annotation)`. The writer keeps `<b>`, `<i>`, `<u>`, `<s>` and `<font>`, and turns the WebVTT color classes into `<font color>`. `StripTags` drops every tag. The lines join with `<br>`. The tags that a cue leaves open close at its end, because an HTML parser keeps them open into the next `<P>`. U+00A0 writes as `&nbsp;`.
- **Invalid UTF-8**: an `*UnwritableError` whose `Err` is `ErrInvalidUTF8`.
- **BOM**: none by default.
- **Lost on write**: the rules and paragraphs of the other classes, comments, `<font face>`, speakers, word timestamps, the end times of overlapping cues and the line breaks of the source.

### Options

`sami.ReadOptions` has the field `LanguageClass`, in the root `ReadOptions.SAMI`. Empty picks the class by the table above. A value of only white space fails `Validate`. The reader reads `Lenient`, `LastCueDuration` and `Encoding`. `sami.WriteOptions` has no fields. The writer reads `StripTags`, `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#sami).

## MicroDVD

MicroDVD counts time in video frames. Many old DivX downloads use it with the `.sub` extension. Package `format/microdvd` reads and writes it. The format name is `microdvd`, the extension `.sub`. No BOM by default.

```text
{1}{1}23.976
{24}{72}Where are you?|{y:i}Home.
```

This file reads as one cue from 1.001 s to 3.003 s with the lines `Where are you?` and `<i>Home.</i>`. `|` separates the lines of a cue.

### Frame rate

A read needs the frame rate of the video. The reader takes the first source that has one:

| # | Source | File data |
|---:|:--- |:--- |
| 1 | `ReadOptions.MicroDVD.FrameRate` | `frameRate`, and `frameRateLine: true` when the file has a `{1}{1}<fps>` line |
| 2 | a first line `{1}{1}<fps>`, such as `{1}{1}25`. The reader removes the line | `frameRate`, and `frameRateLine: true` |
| 3 | lenient mode only: 23.976, with the warning `The file has no {1}{1}<fps> line. The parser used 23.976 fps.` | `frameRate` |

- **No frame rate**: strict mode fails with a `*ParseError` whose `Option` is `ReadOptions.MicroDVD.FrameRate`. The CLI turns it into the advice to pass `--fps`.
- **First line**: strict mode looks at the first non-empty line. Lenient mode looks at the first line of the shape `{N}{N}text`, so the frame rate line may follow junk such as a release name.
- **Line value 0**: `{1}{1}0` fails in both modes with `The frame rate must be greater than 0, got 0.`. With the option set, the reader does not check the line value.
- **Not a frame rate line**: `{1}{1}Hello` is a cue from frame 1 to frame 1.
- **23.976**: the plain float, not 24000/1001, as mantas-done/subtitles, pysubs2 and subtitle_it use it.
- **Write**: `WriteOptions.MicroDVD.FrameRate`, else the stored `frameRate`, else an `*InvalidArgumentError` with `Option` `WriteOptions.MicroDVD.FrameRate`. So a file read at 25 fps writes back at 25 fps without options. A stored `frameRate` that is not above 0 fails with the same `Option` and `The MicroDVD frame rate must be greater than 0, got X.`, for example `got -2.` for -2.

### Read

- **Lines**: one cue per non-empty line `{start}{end}text`. Each line is trimmed. A line of another shape fails: `The line "L" is not a MicroDVD cue.`. So `{24}{}Hello` detects as MicroDVD and then fails.
- **Times**: frames divided by the frame rate, rounded to whole milliseconds. A time of 100000 hours or more fails with `The time "{N}" is not below 100000 hours.`. A frame number beyond the int64 range reads as the int64 maximum.
- **Text**: `&`, `<` and `>` are text, so the line holds `&amp;`, `&lt;` and `&gt;`. A line that is empty after its codes is dropped. Invalid UTF-8 stays as bytes.

| Control code | Core markup |
|:--- |:--- |
| `{y:i}`, `{y:b,u}` with only the letters `b i u s`, commas and spaces | `<i>`, `<b>`, `<u>`, `<s>` |
| `{c:$BBGGRR}` with 6 hex digits | `<font color="#rrggbb">`, with the bytes swapped to RGB |
| other codes, such as `{f:Arial}`, `{s:20}`, `{P:0}`, `{y:x}` and `{c:red}` | none. The codes go to cue data `microdvd.lines[i].otherCodes` |

- **Scope**: a lower-case letter styles its own line. An upper-case letter, such as `{Y:i}`, styles every line of the cue, also the lines before it. A line color wins over a cue color.
- **Place**: only the codes at the start of a line count. `Hello, {y:i}world` keeps the code as text.
- **Cue data**: `microdvd.lines` holds one object per kept line: `codes`, the raw code prefix, `otherCodes`, `color` and `tags`.

### Write

- **Cue line**: `{` + start frame + `}{` + end frame + `}` + text + LF. A frame is the time in seconds times the frame rate, rounded half away from zero. A negative time writes as frame 0. A cue without text writes as `{24}{72}`.
- **Frame rate line**: `{1}{1}` + the frame rate + LF, when `WriteFrameRateLine` is set or the file data holds `frameRateLine: true`. The rate has at most 14 significant digits: `25`, `23.976`, `29.97002997003`.
- **Codes**: a style that wraps a whole line becomes a code: `{c:$BBGGRR}`, then `{y:b}`, `{y:i}`, `{y:u}`, `{y:s}`, then the stored `otherCodes` of the line. A tag open across lines wraps each line, so `<i>One` and `Two</i>` write `{y:i}One|{y:i}Two`. A `<v>` tag does not hide the styles inside it.
- **Stored codes**: when every line has its stored color and tags, each line writes its stored `codes` unchanged, so a round trip keeps the case and the order of the codes.
- **StripTags** writes no codes, also no stored `otherCodes`.
- **Lost on write**: tags that do not wrap a whole line, speakers, word timestamps, alignment. Times move to whole frames.

### Options

The root `ReadOptions.MicroDVD` holds a `microdvd.ReadOptions`, `WriteOptions.MicroDVD` a `microdvd.WriteOptions`. Zero means "not set", because 0 fps is no frame rate. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#microdvd).

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `ReadOptions.MicroDVD` | `FrameRate` | `float64` | 0: the `{1}{1}<fps>` line | the frame rate of the video. It wins over the line |
| `WriteOptions.MicroDVD` | `FrameRate` | `float64` | 0: the stored frame rate | the output frame rate |
| `WriteOptions.MicroDVD` | `WriteFrameRateLine` | `bool` | false: only when the input had the line | write `{1}{1}<fps>` first |

- **Check**: `Validate` rejects a frame rate that is negative, NaN or infinite, for example `The frame rate must be greater than 0, got -1.`, with an error that matches `ErrInvalidArgument`.

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

## MPL2

MPL2 is a one-line-per-cue format from Polish subtitle downloads. Package `format/mpl2` reads and writes it. The format name is `mpl2`, the extension `.txt`.

```text
[12][45]Where are you?|/Home.
```

This line reads as one cue from 1.2 s to 4.5 s with the lines `Where are you?` and `<i>Home.</i>`.

### Read

- **Line**: `[start][end]text`, with both times in tenths of a second. Empty lines are skipped. Every other line that does not match is an error.
- **Line break**: `|` ends a text line. Each part is trimmed, and an empty part is dropped.
- **Italics**: a part that starts with `/` is italic. The reader drops the `/`, trims the part and wraps it in `<i>`. A `/` later in a part stays text. So `[50][60]/ Both| / lines` reads as `<i>Both</i>` and `<i>lines</i>`.
- **Escaping**: MPL2 has no other markup, so `&`, `<` and `>` become entities. `<b>` reads as `&lt;b&gt;`.
- **Order**: the cues sort by start.
- **Encoding**: most downloads are Windows-1250. Pass `ReadOptions.Encoding` when the detection of [encodings.md](encodings.md) misses it. Invalid UTF-8 stays in the text.

### Write

- **Line**: `[start][end]` and the lines joined with `|`. A time rounds to tenths of a second, half away from zero, and a negative time writes as 0. So 1.24 s writes `12` and 4.46 s writes `45`.
- **Every cue**: a cue without text writes `[10][20]`.
- **Italics**: ruby becomes `base (annotation)`. A line gets a `/` prefix when all its visible text is inside `<i>`. The italic state runs on into the next line of the cue, so `<i>Three` and `Four</i>` write `/Three|/Four`. Every other tag drops, and entities decode.
- **BOM**: none by default.
- **Lost on write**: all styles except whole-line italics, speakers, word timestamps and alignment. Times keep only 0.1 s.

### Options

`mpl2.ReadOptions` and `mpl2.WriteOptions` have no fields. The reader reads `Lenient` and `Encoding`. The writer reads `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#mpl2-and-tmplayer).

## TMPlayer

TMPlayer is a one-line-per-entry format from Polish subtitle downloads. It has a start time in whole seconds and no end time. Package `format/tmplayer` reads and writes it, with the TMPlayer+ variants. The format name is `tmplayer`, the extension `.txt`.

```text
00:00:01:Rain
00:00:04:
00:00:09:Sun
```

This file reads as `Rain` from 1 s to 4 s and `Sun` from 9 s to 14 s.

### Read

| Form | Example | Meaning |
|:--- |:--- |:--- |
| TMPlayer | `00:00:01:Where are you?\|Home.` | start 1 s, 2 lines |
| TMPlayer+ | `0:00:01=Where are you?` | `=` instead of `:` |
| TMPlayer+ with line numbers | `00:00:01,1=Where are you?` then `00:00:01,2=Home.` | one cue with 2 lines |
| the same with `:` | `00:00:01,1:Where are you?` then `00:00:01,2:Home.` | one cue with 2 lines |
| end entry | `00:00:04:` | no text: ends the cue before it |

- **Line**: `H:MM:SS`, an optional `,N` line number, `:` or `=`, then the text. The hours have 1 or more digits. The minutes and seconds are `00` to `59`. Colons in the text stay. Empty lines are skipped. Every other line that does not match is an error.
- **Line break and escaping**: as in MPL2. `|` ends a text line, and `&`, `<` and `>` become entities. There are no italics.
- **Line numbers**: an entry with a line number above 1 and the same time as the entry before it in the file adds its lines to that entry. Line number 1 or no number starts a new entry.
- **End times**: the entries sort by time, and entries with the same time keep their file order. An entry with text becomes a cue that ends at the time of the next entry. An entry without text gives no cue. The last cue ends at its start plus `LastCueDuration`, as [reading-writing.md](reading-writing.md#last-cue-default) says.
- **Equal times**: the next entry ends a cue also when it has the same time. So `00:00:01,1=One` and `00:00:01,1=Two` give `One` from 1 s to 1 s and `Two` from 1 s to 6 s.
- **Out of order**: `00:00:07:B` before `00:00:04:A` gives `A` from 4 s to 7 s. The reader reports no time order error.
- **Encoding**: as in MPL2.

### Write

- **Cues**: only cues with visible text. An entry without text would end the cue before it.
- **Line**: `HH:MM:SS:`, then the lines joined with `|`. The start rounds to whole seconds, half away from zero, and a negative start writes as 0. The separator is always `:`.
- **Gap entries**: after each cue except the last, the writer takes the end rounded to whole seconds, at least 1 s after the start. When that end is before the start of the next written cue, it writes `HH:MM:SS:` without text at the end. After the last cue it writes nothing.
- **BOM**: none by default.
- **Lost on write**: everything but plain text and whole-second starts. End times survive only as gap entries.

The cues `Where are you?` and `Home.` from 1 to 4 s, `Next` from 4 to 6 s, `Later` from 3725.4 to 3727 s and `Last` from 3727.6 to 3730 s write as:

```text
00:00:01:Where are you?|Home.
00:00:04:Next
00:00:06:
01:02:05:Later
01:02:07:
01:02:08:Last
```

The cue `Short` from 1.1 to 1.3 s, then the cue `Next` at 5 s, write `00:00:01:Short`, `00:00:02:` and `00:00:05:Next`.

### Options

`tmplayer.ReadOptions` and `tmplayer.WriteOptions` have no fields. The reader reads `Lenient`, `LastCueDuration` and `Encoding`. The writer reads `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#mpl2-and-tmplayer). The detection rule is in the doc comment of `tmplayer.Detect` and in [detection.md](detection.md).

## MPSub

MPSub is the subtitle format of MPlayer. Package `format/mpsub` reads and writes it. The format name is `mpsub`, the extension `.mpsub`. Many MPSub files use `.sub`, which `FormatFromPath` maps to MicroDVD. Content detection runs first, so `Load` reads them as MPSub. A UTF-8 BOM by default.

```text
TITLE=Harbour walk
FORMAT=TIME

1 2.5
The boat leaves.

0.5 2
Mind the gap.
```

A timing line has 2 numbers. The first is the wait after the end of the cue before. The second is the duration. So the first cue runs from 1 s to 3.5 s, and the second from 4 s to 6 s. `FORMAT=25` makes the numbers count frames at 25 fps.

### Read

Each line is trimmed first.

| Position | Line | Meaning |
|:--- |:--- |:--- |
| inside a cue | not empty | a text line. `&`, `<` and `>` become entities |
| inside a cue | empty | ends the cue. The end of the file ends it too |
| outside a cue | empty, or starts with `#` | skipped. The reader keeps no comments |
| outside a cue | `KEY=value`, where KEY is 1 or more ASCII capitals | a header. A trailing ` # comment` is dropped, and the value is trimmed |
| outside a cue | `wait duration`, 2 numbers with an optional `-` and an optional fraction | a timing line. It starts a cue |
| outside a cue | anything else | an error |

- **Times**: the cue starts at the end of the cue before plus the wait, and ends at its start plus the duration. The first cue counts from 0.
- **Seconds and frames**: `FORMAT=TIME` counts seconds. `FORMAT=<n>` counts frames at n frames per second, and a frame count may have a fraction. At 25 fps, `12.5` is 0.5 s. A `FORMAT` line between cues changes the unit for the later cues.
- **Leading integer**: only the leading integer of n counts, as MPlayer and FFmpeg read it. So `FORMAT=29.97` means 29 fps, and `29 58` is 1 s to 3 s.
- **No FORMAT line**: the times are seconds. Strict mode reads without a warning. Lenient mode warns once.
- **Headers**: `TITLE` and `AUTHOR` set the metadata `title` and `author`, and an empty value deletes the key. Every other key with a non-empty value goes to the file format data `mpsub`, in file order. `FORMAT`, `TITLE` and `AUTHOR` are never stored.
- **Negative wait**: allowed. The cue then overlaps the cue before it, and a start before 0 stays. A negative duration is an error.
- **Rounding**: the reader keeps the running position in `float64` seconds and rounds only the cue times to whole milliseconds. So fractional frames do not drift.
- **Time limit**: both times of a cue must be below 100000 hours after the rounding. The sum counts, not each number alone. The message quotes the whole timing line. A time of -100000 hours or less fails with the same message. The bound is the same in both directions, as in TTML and library JSON.
- **Order**: the cues sort by start.
- **Encoding**: invalid UTF-8 stays in the text. Lenient mode adds the decode warning of [encodings.md](encodings.md).

### Write

The header has one `KEY=value` line each, in this order:

1. `TITLE=` and the metadata title, or nothing.
2. `AUTHOR=` and the metadata author, or nothing.
3. `TYPE=` and the stored `TYPE`, else `VIDEO`.
4. Every other stored key, in stored order. `TITLE`, `AUTHOR`, `TYPE`, `FORMAT` and `NOTE` are not repeated.
5. `FORMAT=TIME`, or `FORMAT=` and the whole frame rate.
6. `NOTE=` and the stored `NOTE`, else `Created with subtitle-toolbox-go (https://github.com/yama6a/subtitle-toolbox-go)`.

Then each cue with visible text writes an empty line, its timing line and its text lines.

- **Header lines**: a CR or LF in the title, the author or a stored value becomes a space. A stored key that is not 1 or more ASCII capitals is not written. So the output always reads back.
- **Seconds**: the wait is the start minus the end of the last written cue. The duration is the end minus the start. Both are whole milliseconds as PHP prints a float: `1`, `1.5`, `-1.5`, `0.456`. A negative time is 0 first.
- **Frames**: `WriteOptions.MPSub.FrameRate` writes frame counts. A frame is the time in seconds times the frame rate, rounded to the nearest frame. The wait and duration are differences of frames, so the rounding errors do not add up. The writer computes in binary `float64`, as PHP 8.5 does, so a time on an exact half frame can round down: 64.82 s at 25 fps writes frame 1620, not 1621. 4 cues of 0.06 s at 25 fps write `0 2`, `0 1`, `0 2` and `0 1`.
- **Time limit**: a frame stops at the largest frame whose time is below 100000 hours. At 25 fps that is `8999999999`, so every output reads back.
- **Overlap**: cues `First` from 1 to 4 s and `Second` from 2.5 to 5 s write `1 3` and `-1.5 2.5`. The second reads back as 2.5 s to 5 s.
- **Skipped cues**: a cue without visible text is not written and does not move the end of the last written cue.
- **Text**: the visible text of each line, with entities decoded. So `<i>I &lt;3 bread &amp; jam</i>` writes `I <3 bread & jam`.
- **Frame input**: the reader does not store the `FORMAT` line. A file with `FORMAT=25` writes `FORMAT=TIME` and seconds, unless `FrameRate` is set.
- **NOTE**: the default names this library. PHP writes a `NOTE` that names the PHP library.
- **BOM**: yes by default.
- **Lost on write**: all markup, `#` comments, empty header values and the frame unit of the input. Frame counts round each time to the nearest frame, 40 ms at 25 fps. A header value with ` #` reads back without the comment, so `Part #2` reads as `Part`.

### Options

`mpsub.ReadOptions` has no fields. The reader reads `Lenient` and `Encoding`. The root `WriteOptions.MPSub` holds a `mpsub.WriteOptions`. The writer reads `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#mpsub). The detection rule is in the doc comment of `mpsub.Detect` and in [detection.md](detection.md).

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `FrameRate` | `float64` | 0: seconds and `FORMAT=TIME` | write frames at this rate and `FORMAT=<rate>` |

- **Check**: `Validate` rejects a frame rate that is negative, not whole, NaN or infinite, for example `The MPSub frame rate must be a positive integer, got 29.97.`, with an error that matches `ErrInvalidArgument`. MPlayer and FFmpeg read `FORMAT=29.97` as 29 fps, so a fractional rate would not read back.

## LRC

LRC holds song lyrics with a time per line. Music players and karaoke tools read it. Package `format/lrc` reads and writes it, with the word times of enhanced LRC. The format name is `lrc`, the extension `.lrc`. A BOM by default.

```text
[ti:Morning Train]
[offset:+500]
[00:12.00][01:15.30]The train leaves the station at seven
[00:17.20]
[00:21.10]<00:21.10>Bread <00:21.60>is <00:21.90>warm
```

This file has the title `Morning Train` and 3 cues:

| Start | End | Text |
|---:|---:|:--- |
| 11.5 s | 16.7 s | `The train leaves the station at seven` |
| 20.6 s | 74.8 s | `<00:00:20.600>Bread <00:00:21.100>is <00:00:21.400>warm` |
| 74.8 s | 79.8 s | `The train leaves the station at seven` |

### Read

- **Time tag**: `[mm:ss.xx]`, with 1 to 3 minute digits and 0 to 3 fraction digits, such as `[00:12]`, `[0:01.5]` or `[123:45.678]`. `[01:02:03]` holds hours. Spaces or tabs may stand inside the brackets. `[00:12,00]`, `[00:61.00]`, `[1:75:00.00]` and `[00:12.0000]` are no time tags.
- **Several time tags**: each time tag at the start of a line makes its own cue with the same text.
- **Text on the next line**: a time tag line without text takes the next line as its text when that line is plain text. Plain text is a line that is no time tag line, no ID tag and does not start with `[` and a digit. So `[00:02:35]` then `Hello` gives one cue at 155 s.
- **ID tags**: a line `[key:value]`, with the key in any case and the value trimmed. `ti`, `ar`, `al` and `au` set the metadata `title`, `artist`, `album` and `author`. `#` gives a comment before the next cue in the file. Other keys, such as `by` or `length`, go to the file format data `idTags`.
- **Offset**: the first `[offset:N]` with an integer N moves every time N milliseconds earlier. A time never goes below 0. So `[offset:+500]` turns `[00:12.00]` into 11.5 s, and `[offset:-250]` turns it into 12.25 s. An offset that is no integer, such as `[offset:soon]`, goes to `idTags`.
- **Word times**: `<mm:ss.xx>` in the text becomes the word timestamp `<hh:mm:ss.mmm>`, with the offset applied. The reader drops the spaces after a word time, and before a word time at the line end. Then the file data `spacedWordTimes` is true.
- **Escaping**: `&`, `<` and `>` of the other text become entities. So `I <3 bread` reads as `I &lt;3 bread`.
- **End times**: a cue ends at the next time tag in time, also one without text. When that time tag has no text, the cue data `endLine` is true. The last cue ends at its start plus `LastCueDuration`, as [reading-writing.md](reading-writing.md#last-cue-default) says.
- **Other lines**: every other line is ignored. A file with plain text and no cue fails with `The file has text but no cues.`.
- **Encoding**: invalid UTF-8 stays in the text.

### Write

- **ID tags**: `[ti:]`, `[ar:]`, `[al:]` and `[au:]` for each metadata key that is set, then the `idTags` of the file format data. Each value goes on one line. A stored key that is not a letter, then letters, digits or `_`, is not written, because it would not read back as the same tag. Only the first key of each key in lower case is written, because the reader lower-cases the keys. An `offset` with an integer value is not written either, because the times already hold the offset.
- **Line**: `[mm:ss.cc]`, a space and the text. The time rounds to whole centiseconds, half away from zero, and the hours go into the minutes. So 59.996 s writes `01:00.00` and 3661.905 s writes `61:01.91`. A negative time writes as 0. A time from 999:59.99 on writes as `999:59.99`, the largest time that the reader takes. This holds for line times, end lines and word times.
- **Text**: the lines join with a space. Ruby becomes `base (annotation)`, tags drop and entities decode. A word timestamp writes as `<mm:ss.cc>`. With `spacedWordTimes`, a space separates each word time from the words.
- **End lines**: a cue with `endLine` gets a line `[mm:ss.cc]` without text at its end.
- **Comments**: `[#:text]` before their cue, or at the end.
- **Lost on write**: styles, speakers, the line layout of a cue, times below a centisecond and the `[offset:]` tag. A line with several time tags writes as one line per cue, in time order.

### Options

`lrc.ReadOptions` and `lrc.WriteOptions` have no fields. The reader reads `Lenient`, `LastCueDuration` and `Encoding`. The writer reads `LineEnding` and `BOM`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#lrc). The detection rule is in the doc comment of `lrc.Detect` and in [detection.md](detection.md).

## Plain text

Plain text is a transcript of the words, without times or markup. Package `format/plaintext` writes it. The format name is `txt`, the extension `.txt`. The format has no reader and no detection, so `Parse` with `ReadOptions.Format` set to `stbox.PlainText` returns `ErrUnsupportedFormat` with `The format txt can be written but not read.` Plain text owns `.txt` in `FormatFromPath`. `Load` still finds MPL2 and TMPlayer files named `.txt` by their content.

<!-- example: format/plaintext/ExampleWrite -->
```go
s := &model.Subtitle{}
s.AddCues(
	model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Hello world."}},
	model.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}},
	model.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"<i>Home</i> &amp; bed."}},
)
if err := plaintext.Write(os.Stdout, s, model.CommonWriteOptions{}, plaintext.WriteOptions{}); err != nil {
	fmt.Println(err)
}
// Output:
// Hello world. Where are you going?
//
// Home & bed.
```

The first two cues are 0.5 s apart, so they share a paragraph. The third starts 2 s after the second ends, so it starts a new one.

### Write

- **Text of a line**: ruby becomes `base (annotation)`. All tags go, also `<v>` and word timestamps, and entities are decoded. Each run of spaces and tabs becomes 1 space, and the line is trimmed. Empty lines drop out.
- **Skipped cues**: a cue without a line after that. It takes no part in the paragraphs.
- **Cue text**: the lines of a cue join with a space, or with LF under `KeepLineBreaks`.
- **Paragraphs**: the first cue starts a paragraph. A cue whose start minus the latest end so far is `ParagraphGap` or more starts a new one. The latest end is the largest end of all earlier cues. So `Long` from 0 to 10 s, `Short` from 1 to 2 s and `Overlapped` from 11 to 12 s make one paragraph `Long Short Overlapped`. The gap is exact: cue times are whole milliseconds, so cues at 1.3 s and 3.3 s are 2 s apart. PHP computes it in floats and can split one gap too late ([php-differences.md](php-differences.md)).
- **Paragraph text**: the cues of the paragraph, joined with a space or, under `OneCuePerLine`, with LF. With `WithTimes` the paragraph starts with `[HH:MM:SS] `, the start floored to whole seconds. A negative start writes as 0 and an hour count of 100 or more keeps all digits.
- **Output**: each paragraph ends with LF, and one empty line separates the paragraphs. No cues give empty output.
- **Line feed inside a line**: a decoded `&#10;` stays in the text, as in PHP. The line then spans 2 lines of the output, also without `KeepLineBreaks`. The format has no reader, so nothing reads it back.
- **BOM**: none by default. `StripTags` changes nothing, because the writer always removes the markup.
- **Lost on write**: everything but the text. That is the times except the optional paragraph start, markup, speakers, word timestamps, alignment, and the line layout under the default options.

| Options | Output, with `\n` for LF |
|:--- |:--- |
| zero | `Hello world. Where are you going?\n\nHome & bed.\n\nFish <3\n` |
| `KeepLineBreaks` | `Hello\nworld. Where are you going?\n\nHome & bed.\n\nFish <3\n` |
| `OneCuePerLine` | `Hello world.\nWhere are you going?\n\nHome & bed.\n\nFish <3\n` |
| `ParagraphGap` 2.5 s | `Hello world. Where are you going? Home & bed.\n\nFish <3\n` |
| `ParagraphGap` 0.5 s | `Hello world.\n\nWhere are you going?\n\nHome & bed.\n\nFish <3\n` |
| `ParagraphGap` `NoParagraphBreaks` | `Hello world. Where are you going? Home & bed. Fish <3\n` |
| `WithTimes` | `[00:00:01] Hello world. Where are you going?\n\n[00:00:06] Home & bed.\n\n[01:02:05] Fish <3\n` |

All rows use these 4 cues. They are `<v Mara><b>Hello</b>` and `world.` from 1 to 2 s, `Where are you  going?` from 2.5 to 4 s, `<font color="#ff0000">Home</font> &amp; <00:00:06.500>bed.` and `  ` from 6 to 7 s, and `Fish &lt;3` from 3725 to 3727 s.

### Options

`plaintext.WriteOptions` is the root field `WriteOptions.PlainText`. The writer reads `LineEnding` and `BOM` too. PHP has no CLI flags for these options, and `stbox convert --to txt` writes with the defaults.

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `KeepLineBreaks` | `bool` | false: join the lines of a cue with a space | keep each cue line on its own line |
| `OneCuePerLine` | `bool` | false: join the cues of a paragraph with a space | write each cue on its own line |
| `ParagraphGap` | `*time.Duration` | nil: 2 s | a gap of this length or more after the latest end so far starts a new paragraph. 0 is legal |
| `WithTimes` | `bool` | false | start each paragraph with `[HH:MM:SS] ` |

- **Names**: PHP has 2 bools that default to true, `joinLines` and `joinCues`. Go names them so that false is the default. `ParagraphGap` is a pointer because 0 is legal and differs from the default.
- **One paragraph**: `ParagraphGap: new(plaintext.NoParagraphBreaks)`. PHP passes `INF`, and a `time.Duration` has no infinity. `NoParagraphBreaks` is the largest `time.Duration`, about 292 years, which no gap between cues reaches.
- **Check**: `Validate` rejects a negative gap, for example `The paragraph gap must be 0 or more seconds, got -1.`, with an error that matches `ErrInvalidArgument`.
