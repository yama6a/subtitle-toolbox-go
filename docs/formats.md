# Formats

`Format` is a string type. Its value is the format name that the CLI takes for `--from` and `--to`. It is also the key of the format data of that format. Example: `stbox.SubRip` is `"srt"`. `stbox.Formats()` lists the registry. Each `Format` has the methods `Extensions`, `CanRead`, `CanWrite` and `IsAutoDetected`.

| Format | Go constant | Name | Extensions | Section |
|:--- |:--- |:--- |:--- |:--- |
| ASS and SSA | `stbox.ASS` | `ass` | `.ass`, `.ssa` | [ASS and SSA](#ass-and-ssa) |
| Cavena 890 | `stbox.Cavena890` | `cavena890` | `.890` | [Cavena 890](#cavena-890) |
| CSV and TSV | `stbox.CSV`, `stbox.TSV` | `csv`, `tsv` | `.csv`, `.tsv` | [CSV and TSV](#csv-and-tsv) |
| iTunes Timed Text | `stbox.ITT` | `itt` | `.itt` | [iTT](#itt) |
| library JSON | `stbox.JSON` | `json` | `.json` | [JSON](#json) |
| AssemblyAI JSON | `stbox.AssemblyAI` | `assemblyai` | `.json` | [AssemblyAI JSON](transcripts.md#assemblyai-json) |
| LRC | `stbox.LRC` | `lrc` | `.lrc` | [LRC](#lrc) |
| MicroDVD | `stbox.MicroDVD` | `microdvd` | `.sub` | [MicroDVD](#microdvd) |
| MPSub | `stbox.MPSub` | `mpsub` | `.mpsub` | [MPSub](#mpsub) |
| PAC and PAC Unicode | `stbox.PAC` | `pac` | `.pac`, `.fpc` | [PAC](#pac) |
| PGS | `stbox.PGS` | `pgs` | `.sup` | [PGS](#pgs) |
| SAMI | `stbox.SAMI` | `sami` | `.smi`, `.sami` | [SAMI](#sami) |
| SBV | `stbox.SBV` | `sbv` | `.sbv` | [SBV](#sbv) |
| SCC | `stbox.SCC` | `scc` | `.scc` | [SCC](#scc) |
| MCC | `stbox.MCC` | `mcc` | `.mcc` | [MCC](#mcc) |
| SubRip | `stbox.SubRip` | `srt` | `.srt` | [SubRip](#subrip) |
| EBU STL | `stbox.EBUSTL` | `stl` | `.stl` | [EBU STL](#ebu-stl) |
| Spruce STL | `stbox.SpruceSTL` | `spruce-stl` | `.stl` | [Spruce STL](#spruce-stl) |
| SubViewer 1 and 2 | `stbox.SubViewer` | `subviewer` | `.sub` | [SubViewer](#subviewer) |
| TTML, IMSC, DFXP | `stbox.TTML` | `ttml` | `.ttml`, `.dfxp`, `.xml` | [TTML](#ttml) |
| DCP subtitles | `stbox.DCP` | `dcp` | `.xml` | [DCP subtitles](#dcp-subtitles) |
| plain text | `stbox.PlainText` | `txt` | `.txt` | [Plain text](#plain-text) |
| MPL2 | `stbox.MPL2` | `mpl2` | `.txt` | [MPL2](#mpl2) |
| TMPlayer, TMPlayer+ | `stbox.TMPlayer` | `tmplayer` | `.txt` | [TMPlayer](#tmplayer) |
| QuickTime text | `stbox.QuickTimeText` | `qttext` | `.txt` | [QuickTime text](#quicktime-text) |
| VobSub | `stbox.VobSub` | `vobsub` | `.idx` | [VobSub](#vobsub) |
| WebVTT | `stbox.WebVTT` | `vtt` | `.vtt` | [WebVTT](#webvtt) |
| YouTube chapters | `stbox.YouTubeChapters` | `youtube-chapters` | `.txt` | [Chapters](chapters.md) |

- **Shared extensions**: some formats use the same extension, such as `.sub` for MicroDVD and SubViewer. The first format in the registry owns the extension for `FormatFromPath` and for the extension fallback of `Load`. MicroDVD owns `.sub`, so `Load` finds MPSub and SubViewer files named `.sub` only by their content. Plain text owns `.txt`, so `Load` finds MPL2 and TMPlayer files only by their content. To read a file as another format, pass `ReadOptions.Format`. Spruce STL uses `.stl`, which EBU STL uses too. The first format in the registry owns it, so the EBU STL row owns `.stl`. Content detection tells the two apart.
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
- **Offset from cue settings**: a percent `line` or `position` gives `Cue.Position.Offset`, the point of the alignment in percent of the video. Example: `line:95%,end` gives (50, 95), and `line:0 position:30% align:left` gives (30, 0). An axis counts only when its setting names the same anchor as the alignment. `line:90%` has the anchor `start`, which is no bottom anchor, so it gives no offset. A `line` number such as `line:5` counts lines of text and gives none. A cue with `vertical`, or with no percent value, gets no offset. `line:0` alone gives none too. Each value has at most 3 decimals. An offset that equals the default anchor of its alignment (10, 50 or 90 percent on each axis, such as (50, 90) for `BottomCenter`) is not stored, because it says no more than the alignment.
- **Offset on write**: a cue with no stored `line`, `position` or `align`, no stored `vertical`, and an offset inside the screen cell of its alignment gets the offset as settings, after the kept `size` and `region`. The writer rounds the offset to 3 decimals before the cell test. Only `BottomCenter` writes its height: (45, 95) gives `line:95%,end position:45%,center`. The other rows keep the row setting of the alignment, because the reader maps any other `line` value to `BottomCenter`. So (20, 5) with `TopLeft` writes `line:0 position:20%,line-left align:left` and reads back as (20, 0). The cell is the third of the screen that holds the alignment: below 100/3 percent is the left column or top row, above 200/3 the right column or bottom row.
- **Settings**: the stored settings of the 6 names come back in stored order while the cue alignment is unchanged. After a change of the alignment, the writer keeps `vertical`, `size` and `region` and writes the settings of the new alignment: `line:0` or `line:50%,center` for the row, `align:left` or `align:right` for the column. See [format-data.md](format-data.md#an-explicit-alignment-wins-over-a-stored-position).
- **Tags**: the writer keeps `<b>`, `<i>`, `<u>`, `<strong>`, `<v>`, `<lang>`, `<c>`, `<ruby>` and `<rt>`, their class forms such as `<c.yellow>` or `<v.first.loud Fred>`, and inline timestamps. It drops every other tag and keeps its text. `StripTags` drops every tag and the inline timestamps.
- **Comments**: the writer replaces `-->` in a comment with `->` until none is left, trims each line and drops the empty lines, because an empty line would end the `NOTE` block. A comment of one line is written as `NOTE text`, a longer one starts on the line after `NOTE`.
- **Arrow in text**: `-->` in cue text is written as `--&gt;`, because a raw `-->` would start a new cue.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: `<s>`, colors without a WebVTT class, font faces, unknown tags, the place of comments next to `STYLE` and `REGION` blocks, empty lines inside cue text, the line layout of `NOTE` blocks, settings with other names, and the layout of the header text.

### Dropped colors

The writer turns `<font color>` into a [color class](markup.md#colors) when the color is one of the 8 WebVTT colors, so `#FFFF00`, `#ff0` and `yellow` all become `<c.yellow>`. WebVTT has no class for other colors, so the writer drops them. `webvtt.Write` and `stbox.Write` return a `WriteReport` with one `DroppedColor` per dropped color. In `stbox.Write`, the field `WebVTT` of the report holds it.

<!-- example: format/webvtt/ExampleWrite_droppedColors -->
```go
s := &model.Subtitle{}
s.AddCue(model.Cue{Start: 4 * time.Second, End: 6 * time.Second, Lines: []string{`<font color="#123456">Rain</font> <font color="#ffff00">and sun</font>`}})
var out strings.Builder
report, err := webvtt.Write(&out, s, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{})
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

## Spruce STL

Spruce STL is the text subtitle file of DVD Studio Pro and Spruce DVDMaestro. Package `format/sprucestl` reads and writes it. The format name is `spruce-stl`, the extension `.stl`. No BOM by default.

```text
$VertAlign = Bottom
00:02:17:10 , 00:02:20:10 , The bakery opens at seven, | every day of the week.
```

At 25 fps this file reads as one cue from 137.4 s to 140.4 s with 2 lines. The file does not name its frame rate. The time code counts frames in the last field.

### Grammar

| Line, after trimming | Rule |
|:--- |:--- |
| cue | `hh:mm:ss:ff<sep>hh:mm:ss:ff<sep>text`. The hours have 2 or more digits. Minutes and seconds are 00 to 59. The separator is a comma with optional spaces or tabs around it |
| command | `$Name = value`. Names and values compare without case |
| comment | starts with `//` |
| line break in the text | `\|` with optional spaces or tabs around it |
| toggle in the text | `^B` bold, `^I` italics, `^U` underline. Each flips its flag |

- **Frame rate**: `ReadOptions.FrameRate`. Without it, the highest frame number of all cue lines decides: 25 or more gives 30 fps, else 25 fps. A file without cue lines gives 25 fps. In lenient mode, the guess adds the warning `The file does not give its frame rate. The parser read the frames at 25 fps.`, `Repaired`, with no line and no block.
- **Times**: hours, minutes and seconds are clock time. The frames add `frames / rate` seconds, rounded to whole milliseconds. `99:00:00:00` is 356400 s. The option rate can be a non-integer rate such as 29.97. The frame number must be below the rate rounded up, else the read fails with `The frame number in "hh:mm:ss:ff" is not below the frame rate of R fps.`
- **Detection**: the file starts with `$` or `//` lines and a cue line, or with a cue line whose text does not start with a double quote. The signature needs exactly 2 hour digits. So a file whose first cue starts at hour 100 is not detected, but the reader reads it. `sprucestl.Detect` holds the rule.
- **Shared extension**: EBU STL uses `.stl` too. `FormatFromPath` returns the format of the first registry row.

### Read

1. Each line is trimmed for matching. A blank line is ignored after the first cue line.
2. A line that is no cue, command, comment or blank fails with `The line "L" is no Spruce STL subtitle, command or comment.`, and lenient mode skips it.
3. A cue line with a bad frame number, a time of 100000 hours or more, or an end before the start fails as in [lenient-parsing.md](lenient-parsing.md#spruce-stl) says.
4. **Style**: bold, italics and underline start off, the alignment starts at `BottomCenter`. `$Bold`, `$Italic` and `$Underlined` take `True` or `False`. `$HorzAlign` takes `Left`, `Center` or `Right`. `$VertAlign` takes `Top`, `Center` or `Bottom`. Every `$` line applies to the state, also in the header. Other commands and other values change nothing.
5. **Cue text**: the text splits at the line breaks. Each part is trimmed and split at the toggles. The style starts from the commands for every cue and carries from one line of the cue to the next. `&`, `<` and `>` become entities. Empty lines drop.
6. **Alignment**: the vertical and horizontal commands give one of the 9 alignments. `$VertAlign = Center` gives the middle row.
7. **After the first cue**: a `//` line becomes a comment before the next cue. A `$` line goes to the cue data `commands` of the next cue.

| Input | Lines |
|:--- |:--- |
| `$Italic = TRUE`, then `... , Sung ^Ispoken^I sung\|still sung` | `<i>Sung</i> spoken <i>sung</i>` and `<i>still sung</i>` |
| the next cue `... , ^IPlain` | `Plain` |
| `... , Tickets are ^Bsold out^B today.` | `Tickets are <b>sold out</b> today.` |

The file data `spruce-stl` holds:

| Level | Field | Type | Meaning |
|:--- |:--- |:--- |:--- |
| file | `frameRate` | number | the rate that the reader used |
| file | `header` | list of strings | every raw line before the first cue line, with blank, `$` and `//` lines, not trimmed |
| file | `separator` | string | the first separator, such as ` , ` or a tab, comma and tab. Absent without a cue line |
| file | `lineBreak` | string | the first `\|` with its spaces. Absent without a `\|` |
| file | `trailer` | list of strings | the `$` lines after the last cue. Absent without such lines |
| cue | `commands` | list of strings | the raw `$` lines between the previous cue and this one |

The text after the last line end is no line, so a header without cues does not grow a blank line on each round trip.

### Write

1. **Rate**: `WriteOptions.FrameRate`, else the stored `frameRate`, else 25 fps.
2. **Separator and line break**: the stored `separator` when it is a comma with optional spaces or tabs, else ` , `. The stored `lineBreak` when it is `|` with optional spaces or tabs, else `|`.
3. **Header**: the stored `header` lines, also when the list is empty. Without stored data, the 17 default commands `$FontName = Arial` to `$TapeOffset = False` and an empty line. DVD Studio Pro can fail to import a file without them. The header lines update the style.
4. **Per cue**: the comments as `//` lines, one per comment line. The stored `commands`. Then `$HorzAlign` and `$VertAlign` when the alignment of the cue differs from the style. Then the cue line.
5. **Time code**: whole seconds, then the frame `round(fraction * rate)`. A frame at or above the rate rounded up rolls into the next second as frame 0. 2.99 s at 25 fps writes `00:00:03:00`. A time within half a frame below 100000 hours stops at `99999:59:59:24`, so the file reads back.
6. **Text**: ruby becomes text and each tag but `<b>`, `<i>` and `<u>` goes. A toggle goes before each text that needs another style than the one before, in the order `^B`, `^I`, `^U`. The first style is the one of the commands. Lines are trimmed, empty lines drop, and the lines join with the line break.
7. **Comments** after the last cue come after it. The `trailer` lines follow them.

| Case | Output |
|:--- |:--- |
| a new file, a cue 137.4 s to 140.36 s with 2 lines | the default header, then `00:02:17:10 , 00:02:20:09 , The bakery opens at seven,\|every day of the week.` |
| a cue `One`, a cue `Sign` at `TopLeft`, a cue `Two` | `... One`, `$HorzAlign = Left`, `$VertAlign = Top`, `... Sign`, `$HorzAlign = Center`, `$VertAlign = Bottom`, `... Two` |
| 1.6 s read at 25 fps and written at 30 fps | `00:00:01:18` |

- **Escapes**: the text of a line cannot hold `|` or a toggle. A `|` in the text writes as U+00A6 BROKEN BAR. A `^` before `B`, `I` or `U` gets U+2060 WORD JOINER after it. A line break inside a line becomes the line break of the format.
- **Stored lines**: a CR or LF in a stored `header`, `commands` or `trailer` line writes as a space, so each stored line stays one line. The writer skips a stored line that the reader would not read as that kind of line: a header line that is not blank, `$` or `//`, and a command or trailer line that does not start with `$`.
- **Frame rate limit**: a rate above 100 fps fails with an `*UnwritableError` that names `WriteOptions.SpruceSTL.FrameRate`, because a frame number has 2 digits.
- **Lost on write**: colors, strike, voices, fonts and ruby structure. The input line ending and the blank lines after the first cue. The output ends with LF.
- **Round trip**: a file with a final line end comes out byte for byte with the same `LineEnding`, except for what the writer does not keep. That is the order of `$` and `//` lines between 2 cues (comments come first), blank lines after the first cue, white space around the line break of a later cue, and a separator that differs from the first one.

### Options

The root `ReadOptions.SpruceSTL` holds a `sprucestl.ReadOptions`, `WriteOptions.SpruceSTL` a `sprucestl.WriteOptions`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#spruce-stl).

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `ReadOptions.SpruceSTL` | `FrameRate` | `FrameRate` | zero value: guess 25 or 30 fps | the frame rate of the time codes |
| `WriteOptions.SpruceSTL` | `FrameRate` | `FrameRate` | zero value: the stored frame rate, else 25 | the output frame rate |

- **Check**: `Validate` rejects a frame rate that fails `FrameRate.Validate`. The `Option` is `ReadOptions.SpruceSTL.FrameRate` or `WriteOptions.SpruceSTL.FrameRate`. The messages are `The frame rate must be greater than 0, got X.` and `The Spruce STL frame rate must be greater than 0, got X.`

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
| the `Name` field | `Cue.Speaker`, trimmed |

| Override tag | Core markup |
|:--- |:--- |
| `\an1` to `\an9`, SSA `\a1` to `\a11` | the cue alignment. The first tag of the event wins, else the style alignment, else `BottomCenter` |
| `\b1`, `\i1`, `\u1`, `\s1`, and `0` or no value | `<b>`, `<i>`, `<u>`, `<s>` and their end tags. `\b700` is dropped |
| `\c&HBBGGRR&` or `\1c&HBBGGRR&` with 1 to 8 hex digits, and `\c` alone | `<font color="#rrggbb">` and `</font>` |
| `\k`, `\K`, `\kf`, `\ko` in centiseconds | a word in `Cue.Words`. The first syllable starts at the event start, and the word ends at its start plus the syllable length. A syllable without text is no word. |
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
- **Changed cue**: the writer converts the core markup to override tags: `\an` (or the SSA `\a` code) for an alignment other than that of the event style, `\b1` and `\b0` and the other toggles, `\c&HBBGGRR&` for a `<font>` color or a WebVTT color class, and karaoke tags for words, with a gap syllable where a word ends before the next one starts. A pair such as `<i>` around all text whose flag the style sets is left out. `Cue.Speaker` goes to the `Name` field. A cue without `Speaker` takes the name of its first `<v>` tag.
- **BOM**: the output starts with a UTF-8 BOM by default.
- **Lost on write**: in changed cues every override tag outside `\b \i \u \s \c \an \a \k`, `<v>` beyond the first speaker, `<c>` classes other than the 8 colors, `<lang>`, ruby structure. The event order. The place of `[Script Info]` comments. Empty lines and leading white space in other sections. Text with `{`, `}` or `\` is written raw, because ASS has no escape for them, so it reads back as override tags. Only `\N`, `\n` and `\h` get U+2060 after the backslash, as [Cue text that reads as structure](reading-writing.md#cue-text-that-reads-as-structure) says.

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

TTML is the W3C XML format for timed text. IMSC is its profile for streaming and broadcast, and DFXP is its draft form that Netflix deliveries and Flash players use. Package `format/ttml` reads and writes all 3. The format name is `ttml`, the extensions `.ttml`, `.dfxp` and `.xml`. An iTT file has the TTML signature, so `Detect` and `Parse` read it as TTML. See [iTT](#itt) for `.itt` files.

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
| `ttm:agent` | one agent over all the text gives `Speaker`. Other agents give `<v Name>` spans that close after their text. The name is the one of the agent element in `<head>`, else the agent ID |
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
| `<span>` with inline `tts:` styles | the core markup: `<b>`, `<i>`, `<u>`, `<s>`, `<font color>`, the 8 WebVTT color classes. `Speaker` gives `ttm:agent` on the `<p>`, and so does a lone `<v Name>` before all text of a cue without `Speaker`. Other `<v>` tags give a `<span ttm:agent>` |
| `ttm:agent` elements in the head | each new speaker name, with the ID `agent1`, `agent2` and so on. An agent of the head with the same name keeps its ID |

- **Attributes that are lost**: an attribute whose prefix the output does not declare, such as a prefix that the input declared on the `<p>` itself. On `<tt>`, `ttp:timeBase`, `ttp:clockMode`, `ttp:dropMode` and `ttp:markerMode` whatever their value, because the output holds media times. `ttp:frameRate` and `ttp:tickRate` stay.
- **Offset from a region**: a region with an explicit `tts:origin` gives `Cue.Position.Offset`, the anchor of the cue in percent of the root container. The vertical anchor follows `tts:displayAlign`. The horizontal anchor follows the column of the alignment: the origin for left, the middle for center, the far edge for right. The units are `%`, `px` and cells, with the sizes of `tts:extent` on `<tt>` and the `ttp:cellResolution` as for the row. A missing extent counts as 100%, and values are clamped to 0 to 100. Example: `tts:origin="10% 10%" tts:extent="80% 85%" tts:displayAlign="after" tts:textAlign="center"` gives (50, 95). The same region with `80% 80%` gives (50, 90), the default anchor of `BottomCenter`, so no offset. An anchor that falls outside the cell of its alignment, such as exactly 1/3 of the height, gives none. A region without `tts:origin`, or with another unit, gives none.
- **Regions from offsets**: a cue with no stored region and an offset inside the screen cell of its alignment gets an offset region. It reaches from the anchor point to the screen edges, so it stays inside the root. Example: `BottomCenter` at (50, 95) gets `tts:origin="0% 0%"` and `tts:extent="100% 95%"`. Its ID is the alignment name, the X and the Y: `bottomCenter-50-95`. Cues with the same offset share one region. The reader gives the same alignment and offset back.
- **Regions from alignments**: a cue without a stored region and without a fitting offset gets the region of its alignment. A `BottomCenter` cue gets none, unless it was realigned or the subtitle has no `ttml` data. The writer adds each region once, at the end of the last `<layout>` of the head. It has the shape of the IMSC 1.1 examples with a safe area of 10%: `tts:origin="10% 10%"`, `tts:extent="80% 80%"`, and the `tts:displayAlign` and `tts:textAlign` of the row and the column. Its ID is the alignment name in lower camel case, such as `bottomCenter`.
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

## DCP subtitles

A digital cinema package (DCP) holds subtitles as XML in 2 dialects. Interop `<DCSubtitle>` comes from the CineCanvas specification. SMPTE ST 428-7 `<SubtitleReel>` comes in the 2007, 2010 and 2014 namespaces. Package `format/dcp` reads and writes both. The format name is `dcp`, the extension `.xml`. TTML owns `.xml` in `FormatFromPath`, so `Load` finds a DCP file by [detection](detection.md) and a write needs `Format: stbox.DCP`.

```xml
<DCSubtitle Version="1.0">
  <Subtitle SpotNumber="1" TimeIn="00:01:30:020" TimeOut="00:01:33:031" FadeUpTime="20" FadeDownTime="20">
    <Text VAlign="bottom" VPosition="15">The ferry leaves at nine,</Text>
    <Text VAlign="bottom" VPosition="8">from the north pier.</Text>
  </Subtitle>
</DCSubtitle>
```

This file reads as one cue from 90.08 s to 93.124 s with 2 lines and the alignment `BottomCenter`. The last field of an Interop time counts ticks of 4 ms, 250 per second. The line at `VPosition` 15 comes first, because it is higher on the screen.

### Read

| Input | Goes to |
|:--- |:--- |
| the root: `DCSubtitle` is Interop, `SubtitleReel` is SMPTE, with any prefix | file data `dcp.dialect`. Another root gives `The root element must be <DCSubtitle> or <SubtitleReel>, not <NAME>.` with the name as written |
| `MovieTitle` (Interop) or `ContentTitleText` (SMPTE), a child of the root | the metadata `title`, trimmed. An empty title gives none |
| SMPTE `Language` | the metadata `language`, trimmed. Interop names the language in words, such as `English`, so the reader skips it |
| the root namespace URI and prefix of an SMPTE root | file data `dcp.namespace` and `dcp.prefix`. The reader does not check the URI |
| the rate of the SMPTE times | file data `dcp.timeCodeRate` |
| the root attributes in file order, plus `xmlns:xs` when declared | file data `dcp.attributes`. The names are local names, as the PHP library writes them |
| each child of the root that has no child element, other than `Font`, `Subtitle`, `SubtitleList` and `LoadFont` | file data `dcp.header`, the local name to the text, not trimmed |
| each `LoadFont` child of the root | file data `dcp.loadFonts`, a list of `attributes` and `text` |
| the attributes of the first `Font` child of the subtitle list | file data `dcp.font`. The subtitle list is the root for Interop and `SubtitleList` for SMPTE, or the root when SMPTE has none |
| each `Subtitle` with `Text` lines | a cue. The cue data `dcp.subtitle` holds its attributes |
| each `Text` that gives a line, top line first | a line of the cue. The cue data `dcp.texts` holds the attributes of each |

- **Rate**: Interop counts 250 ticks per second. SMPTE uses `TimeCodeRate` when it is a whole number from 1 to 1000. Else it uses `EditRate` as `a b`, the quotient rounded, when both are above 0 and the result is from 1 to 1000. Else it uses 24. So `<EditRate>48 1</EditRate>` gives 48.
- **`StartTime`**: the reader does not subtract it. It stays in `dcp.header`, so a caller can shift the cues. A `StartTime` of `10:00:00:00` and `TimeIn="00:00:13:00"` give 13 s.
- **Times**: `HH:MM:SS:EE` with 1 to 3 digits in EE, below the rate. `00:01:30:020` is 90.08 s, and SMPTE `00:00:30:04` at 25 fps is 30.16 s. Interop also reads `HH:MM:SS.sss` with any count of fraction digits, so `01:12:42.5` is 4362.5 s. Minutes and seconds have 1 or 2 digits and no range check. The reader rounds each time to whole milliseconds. Another shape, or an EE at or above the rate, fails with `The time "T" is not a DCP time code at R units per second.` R prints as `250` or `25`.
- **Fonts**: `Font` elements nest around `Subtitle`, inside it and inside `Text`. Each passes its attributes to the elements inside it, with lower-case names and values. `color` of 8 hex digits `AARRGGBB` gives `<font color="#rrggbb">` unless it is white, and the alpha is dropped. `weight` `bold` gives `<b>`. `italic` `yes`, `left` or `right` gives `<i>`. `underlined` (Interop) or `underline` (SMPTE) `yes` gives `<u>`. Font name, size, effect and script are ignored.
- **Lines**: each `Text` is one line. Inside it, CR, LF and tab drop, `Space` is a space, and `Ruby` becomes `base (note)`, or `base` without a note.
- **Height**: `VAlign` `top` gives `VPosition` as the height from the top in percent. `bottom` gives 100 minus `VPosition`. `center` or no `VAlign` gives 50 plus `VPosition`. A `VPosition` that is no number counts as 0. Lines sort by height, top first, and empty lines drop.
- **Alignment**: the mean height of the lines gives the row. Below a third it is top, above two thirds it is bottom, else middle. `HAlign` of the top line gives the column: `left`, `right`, else center.
- **Offset**: `Cue.Position.Offset` is the anchor edge of the alignment. `Y` is the height of the top line for a top row, of the bottom line for a bottom row, and the mean height for a middle row. So a 1-line and a 2-line cue at the same place read the same `Y`. `HPosition` of the top line counts from the edge that `HAlign` names, so it gives `X`: left gives `HPosition`, right gives 100 minus it, center gives 50 plus it. A line without `HPosition` has the default anchor of its column: 10, 50 or 90. Values are clamped to 0 to 100. A top or bottom row whose edge line lies within 10 % of the screen edge, such as `VAlign="bottom" VPosition="8"`, has no offset when `X` is the default anchor of the column. `HAlign="left" HPosition="10"` is such a place.
- **No lines**: a `Subtitle` without `Text` gives no cue. When it holds an `Image` element, it fails with `The subtitle at T is an image. The parser reads only text subtitles.` The reader reads only text subtitles.
- **Text before the XML**: strict mode fails and lenient mode skips it, as in [lenient-parsing.md](lenient-parsing.md#dcp-subtitles).
- **XML**: the reader loads no DTD and does no repair. Malformed XML fails with `The content is not well-formed XML.` in both modes. Text that is not valid UTF-8 makes the XML malformed. Elements match by local name.

### Write

The writer keeps the dialect, root, header, fonts and positions of the input. A subtitle without a stored dialect gets the defaults of DCP-o-matic: SMPTE ST 428-7:2014 at 24 fps. A cue from 5 s to 7.125 s with the lines `The ferry leaves at nine,` and `from the <i>north</i> pier.` writes as:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<SubtitleReel xmlns="http://www.smpte-ra.org/schemas/428-7/2014/DCST" xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <Id>urn:uuid:7f9b6950-3c59-3ac4-899d-fe2670a92d7e</Id>
  <ContentTitleText></ContentTitleText>
  <IssueDate>2026-10-12T00:00:00</IssueDate>
  <ReelNumber>1</ReelNumber>
  <Language>en</Language>
  <EditRate>24 1</EditRate>
  <TimeCodeRate>24</TimeCodeRate>
  <StartTime>00:00:00:00</StartTime>
  <LoadFont ID="font">urn:uuid:089a80de-bed7-3cfc-8bde-d41ea8e3cec0</LoadFont>
  <SubtitleList>
    <Font AspectAdjust="1.0" Color="FFFFFFFF" Effect="none" EffectColor="FF000000" ID="font" Italic="no" Script="normal" Size="42" Underline="no" Weight="normal">
      <Subtitle SpotNumber="1" TimeIn="00:00:05:00" TimeOut="00:00:07:03" FadeUpTime="00:00:00:00" FadeDownTime="00:00:00:00">
        <Text Valign="bottom" Vposition="15">The ferry leaves at nine,</Text>
        <Text Valign="bottom" Vposition="8">from the <Font Italic="yes">north</Font> pier.</Text>
      </Subtitle>
    </Font>
  </SubtitleList>
</SubtitleReel>
```

`IssueDate` is the time of the write in UTC. A caller who needs the same bytes sets `dcp.header.IssueDate` in the format data. The `Id` and the `LoadFont` text are name-based UUIDs of version 3. The `Id` is the MD5 hash of the `Font` and `Subtitle` elements as written, so the same subtitles give the same file. A stored header value replaces its default in place, and its other fields come after.

| Output | Comes from |
|:--- |:--- |
| the root | the stored dialect, namespace, prefix and attributes. With a prefix such as `dcst`, every element uses it and the root declares `xmlns:dcst` |
| a title | `ContentTitleText` (SMPTE, always) or `MovieTitle` (Interop, only when the header has it) |
| a language | SMPTE `Language`, only when the header has it |
| the header and the `LoadFont` elements | `dcp.header` and `dcp.loadFonts`, in stored order |
| the `Font` around the subtitles | `dcp.font`, when present. One `Font` surrounds all subtitles |
| `SpotNumber` | the position of the cue, from 1 |
| `TimeIn` and `TimeOut` | `HH:MM:SS:EE` from the time at the rate of the file. EE has 3 digits for Interop and for a rate above 100, else 2 |
| `FadeUpTime` and `FadeDownTime` | the stored values. A stored subtitle without them gets `20` for Interop or `00:00:00:00` for SMPTE |
| the other `Subtitle` attributes | `dcp.subtitle` of the cue |
| each `Text` | a line of the cue, after ruby becomes text and open tags carry over. Lines without visible text drop |
| the `Text` attributes | see below |
| an inner `Font` | a run whose style differs from the `Font` around the subtitles. It holds only the differing attributes in the order `Color`, `Italic`, `Weight`, `Underlined` (Interop) or `Underline` (SMPTE) |

- **Positions**: the cue keeps its stored `Text` attributes while it has the same number of lines and the stored attributes give its alignment. Else the alignment sets the positions: a bottom row gets `Valign="bottom"` and `Vposition` 8, 15, 22, from the bottom line up. A top row gets `Valign="top"` and 8, 15, 22 from the top line down. A middle row gets `Valign="center"` and (i - (n - 1) / 2) x 7, so 2 lines give `-3.5` and `3.5`. `Halign` comes first, only for left and right. Interop writes `VAlign`, `HAlign` and `VPosition`. A cue whose `Offset` lies in the screen cell of its alignment moves the anchor edge: the bottom line of a bottom row sits at `Vposition` 100 minus `Y`, the top line of a top row at `Y`, and the middle of a middle row at `Y` minus 50. The other lines keep 7 % apart on the same side, so they stay on the screen. `Hposition` is `X` for left, 100 minus `X` for right, and `X` minus 50 for center when that is not 0.
- **Runs**: the base style is the stored `font`, white and plain by default. `<b>`, `<i>`, `<u>` and `<font color>` change the style of a run. Text is entity-decoded, loses the characters that XML 1.0 forbids, and gets `&`, `<`, `>` and `"` escaped. `<s>`, `<v>`, word timestamps and other tags drop.
- **Time limit**: a time that rounds to 100000 hours stops at the last unit below it, so the output reads back.
- **Check**: the writer parses its output. A result that is not well-formed returns an `*UnwritableError` with `The formatter wrote XML that is not well-formed.` Text that is not valid UTF-8 returns an `*UnwritableError` whose `Err` is `ErrInvalidUTF8`.
- **Lost on write**: comments, indentation, header elements with child elements, `Font` elements inside `Subtitle`, which the writer rebuilds per run, font name, size and effect, `<s>`, speakers, word timestamps, images, and more than one `Font` around the subtitles.

### Options

`dcp.ReadOptions` and `dcp.WriteOptions` have no fields, so the root `ReadOptions` and `WriteOptions` have no DCP field. The reader reads `Lenient`. The writer reads `LineEnding` and `BOM`. By default the output has no BOM. `StripTags` has no effect. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#dcp-subtitles).
## iTT

iTunes Timed Text (iTT) is the TTML profile that Apple asks for in iTunes Store and Apple TV deliveries. It uses SMPTE time codes, one `<div>` and a top and a bottom region. Package `format/itt` reads and writes it on top of the TTML code. The format name is `itt`, the extension `.itt`. No BOM by default.

```xml
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="999 1000">
  <body><div><p begin="00:00:01:12" end="00:00:04:00">Hello</p></div></body>
</tt>
```

This file reads as one cue from 1.502 s to 4.004 s, because the effective frame rate is 24 times 999/1000, which is 23.976 fps.

- **Detection**: iTT has no signature of its own. `Detect` returns `ttml` for an iTT file. `Load` returns `itt` when the content is TTML and the file extension is `.itt`, see [detection.md](detection.md#load-and-parse-without-a-format). `Parse` without `ReadOptions.Format` has no file name and reads the same bytes as TTML.

### Read

`itt.Read` calls the TTML reader. The cues, the warnings and the `ttml` format data are what `ttml.Read` gives, with the same lenient rules. It adds the file data `itt`. Its fields are the root attributes `timeBase`, `frameRate`, `frameRateMultiplier` and `dropMode`, in this order, as strings trimmed of white space. A field is there only when the root has the attribute. The prefix of the attribute must map to a TTML parameter namespace, or must be `ttp` without a declaration. A file without such an attribute has no `itt` data.

- **Two namespaces**: `ttp:frameRate="25"` and `p2:frameRate="30"` can name a parameter in 2 namespaces, the TTML one and the DFXP one. The `itt` data keeps the first, which is also the frame rate that the TTML reader reads the times with.

### Write

The writer builds a TTML document with the TTML writer and changes these parts:

| Output | Rule |
|:--- |:--- |
| head | the fixed Apple head: the style `normal` and the regions `top` and `bottom`. A title adds `ttm:title` first, as in TTML. The stored head, styles and regions are dropped |
| `<body>` | `style="normal"`, no other attribute |
| `<div>` | one for all cues |
| region | `top` for the alignments `TopLeft`, `TopCenter` and `TopRight`, else `bottom`. The writer ignores `Offset` and the column, because the Apple profile has only these 2 regions |
| markup | `<b>`, `<i>`, `<u>` and `<font color>`. Ruby becomes `base (annotation)` and WebVTT color classes become `<font color>`. A value that is no color gives a plain `<font>`, which writes no span. `<s>`, speakers, `<c>` without a color, `<lang>` and word timestamps go |
| `<tt>` attributes | after the ones of TTML: `ttp:timeBase="smpte"`, `ttp:frameRate`, `ttp:frameRateMultiplier`, `ttp:dropMode="nonDrop"` |
| `begin`, `end` | SMPTE labels `HH:MM:SS:FF` of the frame count model, see [frame-rates.md](frame-rates.md) |

- **Frame rates**: the writer supports 5 rows. A rate matches a row when it is within 0.01 fps of the effective rate.

| Rate | `ttp:frameRate` | `ttp:frameRateMultiplier` | Effective rate |
|:--- |:--- |:--- |:--- |
| 23.976 | `24` | `999 1000` | 2997/125 |
| 24 | `24` | `1 1` | 24/1 |
| 25 | `25` | `1 1` | 25/1 |
| 29.97 | `30` | `999 1000` | 2997/100 |
| 30 | `30` | `1 1` | 30/1 |

- **Which rate**: `WriteOptions.ITT.FrameRate` wins over the stored rate. The stored rate is `itt.frameRate` times `itt.frameRateMultiplier` (`1 1` when missing), when it matches a row and its `frameRate` equals the `ttp:frameRate` of that row. When the option and the stored rate name the same row, the stored strings stay, so `1000 1001` is not rewritten. A subtitle without a usable stored rate and without the option returns an `*InvalidArgumentError` with `Option` `WriteOptions.ITT.FrameRate` and `iTT output needs the frame rate of the video.`. An option outside the table returns the same type with `The ITT formatter accepts the frame rates 23.976, 24, 25, 29.97 and 30, got 50.`.
- **Multiplier**: the writer counts the stored multiplier as the TTML reader does: 2 numbers above 0, split at white space or `:`. Anything else counts as `1 1`. So `1000x 1001` is 1000/1001.
- **Drop-frame**: `FrameRate2997DF` is near the 29.97 row. The writer writes that row with `ttp:dropMode="nonDrop"`. Labels are never drop-frame.
- **Times**: begin frame is the start rounded to the nearest frame, and 0 for a negative start. End frame is the end rounded the same way, and at least the begin frame plus 1. So each cue lasts 1 frame or more. Both stop at the last frame below 100000 hours, `99999:59:59:24` at 25 fps. At 29.97 fps the label `01:00:00:00` is 3603.6 s.
- **Forced flag**: a forced cue writes `itts:forcedDisplay="true"` and declares `itts`. Output without a forced cue has no `itts`.
- **Unique IDs**: the rules of TTML apply, and the head holds the IDs `normal`, `top` and `bottom`. A cue with the ID `top` comes out as `top_2`.
- **Lost on write**: everything that the TTML writer loses. Also the input head, styles and regions other than `top` and `bottom`, the left and right column, the middle row, `<s>`, speakers, color classes without a color, drop-frame mode and time below one frame.

### Options

`itt.ReadOptions` has no fields, so the root `ReadOptions` has no iTT field. The reader reads `Lenient`. The lenient rules are those of [TTML](lenient-parsing.md#ttml).

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `WriteOptions.ITT` | `FrameRate` | `FrameRate` | zero value: the stored frame rate | the output frame rate. 23.976, 24, 25, 29.97 or 30, each within 0.01 fps |

The writer reads `StripTags`, `LineEnding` and `BOM`. `Validate` rejects a frame rate outside the table.

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
| `ReadOptions.MicroDVD` | `FrameRate` | `FrameRate` | zero value: the `{1}{1}<fps>` line | the frame rate of the video. It wins over the line |
| `WriteOptions.MicroDVD` | `FrameRate` | `FrameRate` | zero value: the stored frame rate | the output frame rate |
| `WriteOptions.MicroDVD` | `WriteFrameRateLine` | `bool` | false: only when the input had the line | write `{1}{1}<fps>` first |

- **Check**: `Validate` rejects a frame rate that fails `FrameRate.Validate`, for example `The frame rate must be greater than 0, got -1.`, with an error that matches `ErrInvalidArgument`.

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

## SCC

Scenarist Closed Captions (SCC) is the delivery format of US broadcast captions. Package `format/scc` reads and writes it. The format name is `scc`, the extension `.scc`. The reader decodes CEA-608, the line 21 caption standard of NTSC video. The writer writes pop-on captions on data channel 1.

```text
Scenarist_SCC V1.0

00:00:01;12	9420 9420 94ae 94ae 9452 9452 97a1 97a1 c8e5 ecec efa1 942f 942f

00:00:04;00	942c 942c
```

The file reads as one cue from 1.768 s to 4.004 s with the line `Hello!` on row 14, aligned `BottomCenter`. The text starts at column 5 when counted from 0, as the format data `columns` does. `Cue.Position.Column` counts from 1, so it is 6.

### Read

- **Header**: the first non-empty line must be `Scenarist_SCC V1.0`. Else the read fails, also in lenient mode.
- **Code line**: a time code, white space and byte pairs. The time code is `HH:MM:SS:FF` or `HH:MM:SS;FF`. The `;` marks drop-frame for that line. The fields have no range check.
- **Byte pairs**: 4 hex digits in any case, one pair for each frame. Word k of a line plays k frames after the start of the line.
- **Times at 29.97 fps**: a frame lasts 1001/30000 s. The time code `00:00:01;12` is frame 42. Its first end-of-caption command is the 12th word, frame 53, so the cue starts at 53 * 1001 / 30000 = 1.768 s. Drop-frame skips the labels `;00` and `;01` at the start of each minute, except every tenth minute. So `00:01:00;02` and `00:01:00:00` are both frame 1800, and `01:00:00;00` is frame 107892.
- **Order**: the reader sorts the code lines by start frame before it decodes them, because some writers emit lines out of time order.
- **Cues**: each change of the captions on screen starts a new cue. A caption that nothing erases lasts `LastCueDuration`, 5 s by default.
- **Position**: `Cue.Position` holds the first row of the cue, its first column counted from 1 (the format data `columns` count from 0), and the 15 by 32 grid. Rows 1 to 4 give `TopCenter`. Rows 5 to 15 give `BottomCenter`.
- **Roll-up captions**: `RollUpScreen` gives one cue per screen. `RollUpLines` gives one cue per row, from its first character until it rolls up or the screen clears.
- **Text**: `&`, `<` and `>` are escaped. Mid-row codes become `<i>`, `<u>` and `<font color>` tags.
- **Format data**: the file holds `dropFrame`, true when the first code line has `;` before the frames. It is absent when the file has no code line. Each cue holds `mode` (`pop-on`, `roll-up` or `paint-on`), `rows` (the row of each line, 1 to 15) and `columns` (the first column of each line, 0 to 31). The cue data has no schema row, so it passes unchecked.
- **Lenient repairs**: see [lenient-parsing.md](lenient-parsing.md#scc).

### Write

The writer turns each cue into a pop-on caption. The caption loads into hidden memory in the frames before the cue start. The EOC command (end of caption) shows it on the start frame. The EDM command (erase displayed memory) clears it at the cue end. A cue from 1 s to 3 s with `Hello!` writes the file at the top of this section, except that the writer puts ENM (erase hidden memory) before RCL (resume caption loading): `94ae 94ae 9420 9420 9476 9476 97a1 97a1 c8e5 ecec efa1 942f 942f`.

- **Output**: the header, an empty line, then one line for each run of consecutive frames. A line holds the label of its first frame, a tab and the words as 4 lowercase hex digits. Lines are separated by an empty line. The output ends with one LF. A subtitle without text writes the header and an LF.
- **Time codes**: `DropFrameOn` writes `00:01:00;02`. `DropFrameOff` writes `00:01:00:00`. `DropFrameKeep` takes the stored `dropFrame` of the file, else drop-frame. A cue from 58 s to 61 s ends with `00:01:01;00` by default and `00:01:00:28` without drop-frame.
- **Line ending and BOM**: `LineEnding` and `BOM` apply. The default writes no BOM. The writer ignores `StripTags`.
- **Limits**: a cue holds at most 4 lines of 32 characters, and only characters that CEA-608 has. A cue that breaks a limit gives an `*UnwritableError`. Its message says the fix in words, for example `Cue #0 at 1 s has a line with 33 characters, but SCC allows 32. Wrap the lines to 4 lines of 32 characters first.` The error wraps `scc.ErrWrapFirst` when the text fits 4 lines of 32 characters after a wrap, and `scc.ErrSplitFirst` when it needs a split into cues first. A cue without text writes nothing.
- **Markup**: `<i>`, `<u>` and `<font color>` in the 7 CEA-608 colors become PAC styles and mid-row codes. Another color becomes white. Other tags drop. A style change inside a word costs a space, because a mid-row code shows as a space. Text that looks like a tag stays: `a &lt; b &gt; c` reads back as the same text.
- **Positions**: the first rule that holds sets the rows and columns. First, the cue data `rows` and `columns` of a read, when both list one integer for each line, the rows rise from 1 to 15, a first row of 4 or less goes with a top alignment, and `Position.Row` equals the first stored row. A column is moved to fit the line. Second, `Position.RowOn` on the 15 by 32 grid, with every line at `Position.Column` when the position is on the CEA-608 grid. Third, the alignment: row 1 for top, the middle rows for middle and the last rows for bottom, with column 0 for left, the end for right and the middle for center.
- **Row edge**: the reader gives rows 1 to 4 a `TopCenter` alignment and rows 5 to 15 a `BottomCenter` alignment. The top third of a 15-row grid is rows 1 to 5, so a cue on SCC row 5 with `BottomCenter` loses its row in other writers that use `RowOn`, such as EBU STL.
- **Scheduler**: each word takes one frame. The EOC goes on the cue start frame when the load fits in the frames before it. Else the EOC moves later, and the caption shows late. The EDM of the caption before goes at its cue end, when there is room. Without room, the new EOC replaces the shown caption. Cues from 1 s to 5 s and from 3 s to 4 s read back as 1.001 to 3.003 s and 3.003 to 4.004 s.
- **Time limit**: a time code can show an hour of up to 6 digits, because drop-frame labels run ahead of real time. The reader takes them. The writer leaves out a word on a frame at or after 100000 hours.

#### Fit

`WriteOptions.Fit` changes what SCC cannot hold, in place of failing. `WriteReport.Changes` lists each change in cue order, as `FitChange` values with `CueIndex`, `Action` and `Message`. `stbox.WriteReport.SCC` is non-nil after every SCC write, also with no change.

| Action | What the fit does | Message |
|:--- |:--- |:--- |
| `Transliterated` | replaces each character that CEA-608 lacks and that has a replacement, such as U+0160 with `S`. It decodes the entities of the text first | `Cue #0 at 1 s: replaced "U+0160" with "S".`, with the character itself in place of `U+0160` |
| `Wrapped` | wraps the lines to at most 4 lines of 32 characters. It stops for more than 128 visible characters without spaces | `Cue #0 at 1 s: wrapped the text into 3 lines of at most 32 characters.` |
| `Delayed` | writes the caption late, because the frames before its start cannot hold its data | `Cue #0 at 1 s shows at 1.969 s. The frames before its start cannot hold its caption data.` |
| `Dropped` | leaves the cue out, because its caption data needs frames up to or after the cue end | `Cue #3 at 5 s is left out. Its caption data needs the frames up to 5.839 s, but the cue ends at 5.4 s.` |

Without `Fit`, a late caption writes late and no change is reported. A cue that still breaks a limit after the fit gives the error above.

### Options

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `WriteOptions.SCC` | `DropFrame` | `DropFrameMode` | `DropFrameKeep` | `DropFrameOn`, `DropFrameOff`, or the stored mode of the file, else drop-frame |
| `WriteOptions.SCC` | `Fit` | `bool` | false | change the text and timing that SCC cannot hold, in place of failing |
| `ReadOptions.SCC` | `Channel` | `int` | 0, read as 1 | 1 reads CC1 and CC3. 2 reads CC2 and CC4 |
| `ReadOptions.SCC` | `RollUp` | `scc.RollUp` | `RollUpScreen` | `RollUpScreen` or `RollUpLines`. The text forms are `screen` and `lines` |

- **Read check**: `ReadOptions.Validate` rejects a channel other than 0, 1 and 2 with `The SCC data channel must be 1 or 2, got 3.` and an unknown roll-up mode. Both match `ErrInvalidArgument`.
- **Write check**: `WriteOptions.Validate` rejects an unknown drop-frame mode with `The SCC drop-frame mode 3 is unknown.`. It matches `ErrInvalidArgument`.
- **Reader**: it reads `Lenient` and `LastCueDuration` too.

## MCC

MCC (MacCaption) is the caption file of Telestream MacCaption and CaptionMaker. Package `format/mcc` reads and writes it. The format name is `mcc`, the extension `.mcc`. Each data line holds a time code and one SMPTE 291 ancillary data (ANC) packet in hex. The packet holds a caption distribution packet (CDP) with CEA-608 byte pairs and CEA-708 service data.

```text
File Format=MacCaption_MCC V2.0
Time Code Rate=30DF

00:00:01:00	T49S494F43Z0072F4FC9420ROO74Z0085AB
```

The line has the time code `00:00:01:00`, a tab, and the hex data. The letter `T` stands for the bytes `61 01`, the DID and SDID of a CDP packet. `S` stands for `96 69`, the CDP identifier. The line holds the captions of frame 30 at 29.97 fps, which starts at 1.001 s. The writer adds a header text of about 30 lines that the format asks every file to hold. The Go constant holds it byte for byte.

### Read

- **Header**: the first non-blank line must be `File Format=MacCaption_MCC V1.0` or `V2.0`. Another version gives `The MCC version 3.0 is unknown. The parser reads 1.0 and 2.0.` Both errors also stop a lenient read.
- **Header fields**: a line `Name=Value` before or between data lines is a header field. The reader stores the fields in file order, except `Time Code Rate`. A later line of the same name replaces the value. Lines that start with `//` and blank lines are skipped.
- **Data line**: `HH:MM:SS:FF`, white space and the hex data. The hour has 2 to 6 digits. Version 2.0 may add a field suffix and a line number, such as `01:00:00:00.1,9`. The reader ignores both. It reads no `;` in the label, because the header decides drop-frame.
- **Hex data**: pairs of hex digits in any case, and the alias letters `G` to `O` (1 to 9 times `FA 00 00`), `P`, `Q`, `R` (`FB 80 80`, `FC 80 80`, `FD 80 80`), `S` (`96 69`), `T` (`61 01`), `U` (`E1 00 00 00`) and `Z` (`00`).
- **Packets**: only ANC packets with DID `61h` and SDID `01h` count. The reader skips other packets with no warning, such as AFD data. It ignores both checksums, because some writers leave them wrong.
- **Frame rate**: the header field `Time Code Rate` is 24, 25, 30, 50 or 60, with `DF` after 30 or 60. The rate code of the first CDP tells 23.976 from 24, 29.97 from 30 and 59.94 from 60. A file with `Time Code Rate=25` and no packet stores the rate `25`. Without a valid `Time Code Rate`, a strict read fails and a lenient read warns `The MCC file has no Time Code Rate line. The parser read 30DF.`
- **Times**: the label is a frame count, as in SCC. Drop-frame skips 2 labels per minute at 30 labels and 4 at 60, except every tenth minute. Packets with the same time code join in file order.
- **Services**: the cues come from CEA-708 service 1, or from `ReadOptions.Service`. When that service has no captions, the cues come from the CEA-608 field 1 pairs, read as in SCC. `CEA608Only` skips CEA-708.
- **Format data**: the file holds `version`, `frameRate` (one of `23.976`, `24`, `25`, `29.97`, `30`, `50`, `59.94`, `60`), `dropFrame` and `header`. CEA-608 cues hold the `scc` cue data of SCC. CEA-708 cues hold none, and only the alignment of the window anchor.
- **Time limit**: a data line at 100000 hours or later fails with `The time "100001:00:00:00" is not below 100000 hours.` A cue that ends at 100000 hours or later ends at 99999:59:59.999, also a cue of CEA-708 data.
- **Lenient repairs**: see [lenient-parsing.md](lenient-parsing.md#mcc).

### Write

The writer writes CEA-608 field 1 and CEA-708 service 1 into each line. It writes one line for each frame that has caption data, and no line for a frame without data.

- **CEA-608**: pop-on captions on channel 1, scheduled as in the SCC writer, with `Fit` always on. The 29.97 fps frames of the scheduler map to the output rate, so 2 words can share a frame.
- **CEA-708**: pop-on windows of service 1 with the text, italics, underline, the 7 CEA-608 colors and the 9 anchor points of the alignment.
- **Frame rates**: 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60. Each has a CDP rate code and a number of triplets per packet.

| Name | Labels per second | CDP rate code | Triplets per CDP |
|:--- |---:|---:|---:|
| `23.976` | 24 | 1 | 25 |
| `24` | 24 | 2 | 25 |
| `25` | 25 | 3 | 24 |
| `29.97` | 30 | 4 | 20 |
| `30` | 30 | 5 | 20 |
| `50` | 50 | 6 | 12 |
| `59.94` | 60 | 7 | 10 |
| `60` | 60 | 8 | 10 |

- **Drop-frame**: only 29.97 and 59.94 have drop-frame labels. `DropFrameOn` and `DropFrameOff` win. With `DropFrameKeep`, a `FrameRate` with its `DropFrame` flag set counts as on. Else the stored `dropFrame` decides, else drop-frame. The labels always use `:`.
- **Version**: the stored `version`, else `2.0`. Drop-frame at 59.94 forces `2.0`, because version 1.0 lists no `60DF`.
- **Header fields**: `UUID`, `Creation Program=Subtitle Toolbox` and `Creation Date` of today in UTC, such as `Monday, October 12, 2026`. Stored fields replace these values in place and add their other fields in stored order. `Time Code Rate` comes last. A stored field whose name has other characters than letters and spaces is left out, because the reader would not read it. A line break in a stored value writes as a space. The UUID is a version 5 style hash of the data lines, so the same captions give the same file on the same day.
- **Triplets**: each line has the field 1 words (or one null pair), one null pair for field 2, the DTVCC packet of service 1, and empty triplets up to the count of the rate. The sequence counter starts at 0 and counts each line.
- **Hex**: upper case, with the aliases `G` to `O`, `P`, `Q`, `R`, `S`, `T` and `Z`. The writer never writes `U`, because files disagree on its length.
- **Empty subtitle**: the header only, ending with `Time Code Rate=30DF` and an empty line.
- **Line ending and BOM**: `LineEnding` and `BOM` apply. The default writes LF and no BOM.
- **Lost on write**: field 2 (CC3 and CC4), CEA-708 styles beyond italics, underline and the 7 CEA-608 colors, and the packets of an input file. The output is always encoded again.
- **Time limit**: the writer leaves out each frame at or after 100000 hours.

#### Fit report

The CEA-608 part follows the limits of SCC: 4 lines of 32 characters and characters that CEA-608 has. The writer fits the cues as SCC does with `Fit`. `WriteReport.Changes` lists each change in cue order, as in [SCC](#fit). `stbox.WriteReport.MCC` is non-nil after every MCC write, also with no change. The CEA-708 data still holds a cue that the fit leaves out of the CEA-608 data. A cue that the fit cannot repair gives an `*UnwritableError` that wraps `mcc.ErrWrapFirst` or `mcc.ErrSplitFirst`.

### Options

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `ReadOptions.MCC` | `Service` | `int` | 0, read as 1 | the CEA-708 service whose captions win when it has any, 1 to 63 |
| `ReadOptions.MCC` | `CEA608Only` | `bool` | false | read only the CEA-608 captions of field 1 |
| `ReadOptions.MCC` | `Channel` | `int` | 0, read as 1 | CEA-608 data channel 1 or 2, as in SCC |
| `ReadOptions.MCC` | `RollUp` | `mcc.RollUp` | `RollUpScreen` | `RollUpScreen` or `RollUpLines`, as in SCC |
| `WriteOptions.MCC` | `FrameRate` | `FrameRate` | zero: the stored rate, else 29.97 | within 0.01 of a rate of the table, so 2997/100 writes at 29.97 |
| `WriteOptions.MCC` | `DropFrame` | `DropFrameMode` | `DropFrameKeep` | `DropFrameOn`, `DropFrameOff`, or the rule above |

- **Read check**: `ReadOptions.Validate` rejects a service above 63 with `The CEA-708 service must be from 1 to 63, got 64.`, a channel other than 0, 1 and 2, and an unknown roll-up mode.
- **Write check**: `WriteOptions.Validate` rejects other frame rates with `The MCC formatter accepts the frame rates 23.976, 24, 25, 29.97, 30, 50, 59.94 and 60, got 48.` and an unknown drop-frame mode.
- **Reader**: it reads `Lenient` and `LastCueDuration` too.

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
| `FrameRate` | `FrameRate` | zero value: seconds and `FORMAT=TIME` | write frames at this whole rate and `FORMAT=<rate>` |

- **Check**: `Validate` rejects a frame rate that fails `FrameRate.Validate` or is not whole, for example `The MPSub frame rate must be a positive integer, got 29.97.`, with an error that matches `ErrInvalidArgument`. MPlayer and FFmpeg read `FORMAT=29.97` as 29 fps, so a fractional rate would not read back.

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

## QuickTime text

QuickTime text is the text track format of QuickTime Player 7 and older Apple tools, defined by [Apple's text descriptors](https://developer.apple.com/library/archive/documentation/QuickTime/Conceptual/QTScripting_HTML/QTScripting_HTML_Document/ScriptingHTML.html). Subtitle Edit writes it too. Package `format/qttext` reads and writes it. The format name is `qttext`, the extension `.txt`. `FormatFromPath` maps `.txt` to plain text, so `Load` finds QuickTime text by its `{QTtext}` line. No BOM by default.

```text
{QTtext} {timeScale:30}
[00:00:01.15]
The museum opens at ten.
[00:00:04.00]
```

A header of descriptors comes first, then samples that each start at a timestamp. The fraction counts units of the time scale, so `[00:00:01.15]` is 1.5 s. The sample ends at the next timestamp, so the cue runs from 1.5 s to 4 s.

### Read

- **Header**: all lines before the first line that is a timestamp line. The reader keeps them raw in the file data `header`. The first header line must start with `{QTtext}`, in any case, after optional white space. Strict mode fails otherwise with `The file does not start with {QTtext}.` on line 1. Lenient mode warns `The file does not start with {QTtext}. The parser read it as QuickTime text.`, `Repaired`, on line 1.
- **Timestamp line**: `[h:mm:ss]` or `[h:mm:ss.f]` after optional white space, with 1 or more hour digits and minutes and seconds up to 59. Text may follow the `]` on the same line. Later lines that are no timestamp line belong to the sample before them. A sample without text is a gap and gives no cue.
- **Time scale**: `{timeScale:N}` in the header, with N all digits and above 0. A fraction then counts units of 1/N s. Without a time scale, the fraction is a decimal fraction, so `[00:00:07.25]` is 7.25 s. A descriptor name matches in any case, and spaces around the colon are allowed.
- **Rounding**: the time is hours, minutes, seconds and fraction, rounded half up to whole milliseconds. The reader computes in integers. So `[00:00:01.13]` at scale 30 is 1433 ms, and a decimal fraction of `[00:00:02.7665]` is 2767 ms.
- **Fraction at or above the time scale**: `[00:00:01.45]` at scale 30 fails in strict mode with `The timestamp "[00:00:01.45]" counts 45 units, but the time scale is 30 per second.` Lenient mode warns the same text plus ` The parser added them to the seconds.`, `Repaired`, and reads 1 s and 45/30 s, which is 2.5 s.
- **Relative timestamps**: `{timeStamps:relative}`, in any case, adds each timestamp to the one before it. The sum must stay below 100000 hours too.
- **End times**: a cue ends at the next sample, also one without text. The last cue ends at its start plus `LastCueDuration`, as [reading-writing.md](reading-writing.md#last-cue-default) says. A sample that ends before it starts fails with `The cue ends at E s, before it starts at S s.` on the line of the sample. Lenient mode swaps the times when the cue then lasts 30 s or less.
- **Styles**: `{bold}`, `{italic}` and `{underline}` turn a style on for the rest of the sample and for all later samples. `{plain}` turns all off. The names match in any case. The styles of the header set the starting style. Other descriptors, such as `{font:Geneva}`, give no markup. A sample that is skipped because of its times does not move the style.
- **Text**: the white space around a styled run stays outside its tags. Each line is trimmed. A line without text drops out. `&`, `<` and `>` become entities.
- **Paired layout**: Subtitle Edit writes a start timestamp, the text, an end timestamp and an empty line. The reader sets the file data `pairedTimestamps` when an empty line follows a sample without text, after at least 2 samples, and another timestamp follows.
- **Time limit**: a timestamp or a relative sum of 100000 hours or more fails with `The time "99999:59:59.9995" is not below 100000 hours.`, after the rounding to milliseconds. The message quotes the timestamp without brackets. Lenient mode skips the line and the text lines after it, up to the next timestamp line.
- **Encoding**: invalid UTF-8 stays in the text. Lenient mode adds the decode warning of [encodings.md](encodings.md).

| Input | Result |
|:--- |:--- |
| `{timeScale:600}` and `[00:00:07.300]` | 7.5 s |
| `{timeScale:10}` and `[00:00:07.5]` | 7.5 s |
| `[00:00:01.00]`, `One`, `[00:00:04.00]`, `[00:00:05.00]`, `Two` | `One` from 1 to 4 s, `Two` from 5 s for `LastCueDuration` |
| `{timeScale:600}{timeStamps:relative}`, `[00:00:01.300]`, `One`, `[00:00:02.000]`, `[00:00:01.000]`, `Two`, `[00:00:02.000]` | `One` from 1.5 to 3.5 s, `Two` from 4.5 to 6.5 s |
| `{italic}Sung`, then `still sung {underline}loud`, then `{plain}Spoken` in 3 samples | `<i>Sung</i>`, `<i>still sung <u>loud</u></i>`, `Spoken` |

The file data `qttext` holds:

| Level | Field | Type | Meaning |
|:--- |:--- |:--- |:--- |
| file | `header` | list of strings | the raw lines before the first timestamp line |
| file | `pairedTimestamps` | bool | true for the layout of Subtitle Edit |
| cue | `source` | list of strings | the raw lines of a sample that holds a descriptor, for the round trip |

### Write

- **Header**: the stored `header` lines. A CR or LF in a stored line becomes a space, a line that reads as a timestamp is not written, and a first line without `{QTtext}` gets that line before it. So the output always reads back. A new file gets the header of Subtitle Edit, which reads only 2 fraction digits:

  ```text
  {QTtext} {font:Tahoma}
  {plain} {size:20}
  {timeScale:30}
  {width:160} {height:32}
  {timestamps:absolute} {language:0}
  ```

- **Layout**: the paired layout for a new file and when the file data has `pairedTimestamps`. It writes the start timestamp, the text lines, the end timestamp and an empty line for each cue. Else the QuickTime layout: an empty sample `[00:00:00.00]` first when the first cue starts after 0, then for each cue its start timestamp only when the start differs, in units, from the end of the cue before, the text lines and the end timestamp.
- **Timestamp**: `[HH:MM:SS.F]`. F counts units of the time scale, zero-padded to the digit count of the scale minus 1, at least 1 digit: scale 30 gives 2 digits, 600 gives 3 and 10 gives 1. Without a time scale, F is milliseconds with 3 digits. Up to 1000 units per second, the count is the seconds as a float times the scale, rounded half away from zero, as PHP 8.5 and the MicroDVD and MPSub writers do. So a time on an exact half unit can round down: 64.82 s at scale 25 writes 1620 units. Above 1000 units per second, floats lose milliseconds, so the writer rounds the exact milliseconds half away from zero. Every time scale keeps the time to the millisecond. A negative start writes as 0, and an end before the start writes as the start.
- **Relative timestamps**: the header `{timeStamps:relative}` makes each timestamp the difference to the time that a reader has after the timestamps written so far. So an overlap moves no later cue, and a rounding error stays within half a unit. A cue that starts before that time cannot go back, so its start timestamp is 0.
- **Time limit**: a timestamp stops at the largest count of units whose time is below 100000 hours, `99999:59:59.29` at scale 30. A relative timestamp shrinks until the sum that the reader adds stays below the limit.
- **Text**: when a cue has `source` and the reader would read `source` as the cue lines, the writer writes `source` unchanged. Else it writes ruby as text, carries open tags into the next line, decodes entities, strips all tags except `<b>`, `<i>` and `<u>`, and trims each line. Before a text run with visible text it writes the descriptors that change the running style to the open tags. Turning a style off writes `{plain}` and then the styles that stay on. Turning styles on writes `{bold}`, `{italic}` and `{underline}` in that order.
- **Running style**: the style carries across cues, as QuickTime reads it. With the header of Subtitle Edit, the cues `<i>Sung</i> and <b>loud</b>` and `<font color="red">red</font> &amp; plain`, then `<i>Sung</i>`, then `<i>Sung <u>loud</u></i>` write as:

  ```text
  [00:00:01.00]
  {italic}Sung{plain} and {bold}loud
  {plain}red & plain
  [00:00:02.00]

  [00:00:03.00]
  {italic}Sung
  [00:00:04.00]

  [00:00:05.00]
  Sung {underline}loud
  [00:00:06.00]
  ```

- **BOM**: none by default.
- **Lost on write**: colors, fonts, speakers, word timestamps and alignment. Descriptors other than bold, italic and underline survive only in unchanged cues and in the header.
- **Round trip**: an unchanged file comes out byte for byte, except for the empty lines between samples in the QuickTime layout.

### Options

`qttext.ReadOptions` and `qttext.WriteOptions` have no fields. The reader reads `Lenient`, `LastCueDuration` and `Encoding`. The writer reads `LineEnding` and `BOM`. `StripTags` changes nothing, because the writer always writes the descriptors of `<b>`, `<i>` and `<u>`. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#quicktime-text). The detection rule is in the doc comment of `qttext.Detect` and in [detection.md](detection.md).

## Plain text

Plain text is a transcript of the words, without times or markup. Package `format/plaintext` writes it. The format name is `txt`, the extension `.txt`. The format has no reader and no detection, so `Parse` with `ReadOptions.Format` set to `stbox.PlainText` returns `ErrUnsupportedFormat` with `The format txt can be written but not read.` Plain text owns `.txt` in `FormatFromPath`. `Load` still finds MPL2, TMPlayer and QuickTime text files named `.txt` by their content.

<!-- example: format/plaintext/ExampleWrite -->
```go
s := &model.Subtitle{}
s.AddCues(
	model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"Hello world."}},
	model.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}},
	model.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"<i>Home</i> &amp; bed."}},
)
if _, err := plaintext.Write(os.Stdout, s, model.CommonWriteOptions{}, plaintext.WriteOptions{}); err != nil {
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

## CSV and TSV

Translators edit subtitles in Excel or Google Sheets, and openai-whisper and whisper.cpp write tables. Package `format/csv` reads and writes such tables with RFC 4180 quoting. The format names are `csv` and `tsv`, the extensions `.csv` and `.tsv`. Both formats store the format data key `csv`. A BOM is written by default, because Excel needs it to read UTF-8.

```csv
start,end,speaker,text
00:00:01.000,00:00:04.000,Anna,Where are you going?
00:00:04.500,00:00:06.000,Ben,"Home.
Now."
```

This table reads as 2 cues: `<v Anna>Where are you going?` and `<v Ben>Home.` plus `Now.`. A table read and written unchanged comes out byte for byte, with its columns, header, delimiter, time format and quoting.

- **Detection**: none. A table has no signature, so `Load` takes CSV and TSV from the extension, and `Parse` needs `ReadOptions.Format`.
- **TSV and CSV**: the reader detects the delimiter, so it reads both. Without `WriteOptions.CSV.Delimiter`, TSV output writes tabs, and CSV output writes `,` when the stored delimiter is a tab.

### Columns

| Role | Header name | Meaning |
|:--- |:--- |:--- |
| `identifier` | `identifier` | `Cue.ID`, trimmed. An empty cell gives no ID |
| `start` | `start` | the start time. Required |
| `end` | `end` | the end time |
| `duration` | `duration` | the cue length. An end cell wins |
| `speaker` | `speaker` | the speaker, trimmed. It becomes a `<v Name>` tag before the first line |
| `text` | `text` | the cue text, with `&`, `<` and `>` escaped and split at LF. Required |

- **Header names** compare without case and after a trim.
- **Synonyms**: when `Columns` is zero and the header lacks `start` or `text`, a role without a header of its name takes the first synonym in the table below. The names compare in lower case with spaces, `_` and `-` removed, so `Start Time` and `start_time` both match `starttime`. A table with both `start` and `text` headers uses no synonyms, so in `start,end,text,name` the `name` column gives no speaker.
- **Synonym warning**: a role found under another name warns in both modes, for example `The parser reads the column "Begin" as start.`, action `Repaired`.
- **Column layout**: `ReadOptions.CSV.Columns` maps a role to a header name with `csv.ColumnNamed` or to a 0-based index with `csv.ColumnAt`. A layout turns the synonyms off. Map one role, for example `Start: csv.ColumnNamed("start")`, to read without synonyms.
- **No header**: with `NoHeader`, `Start`, `Text` and every other mapped role need an index.
- **Missing column**: a missing `start` or `text`, or a mapped column that the table lacks, fails in both modes with `The table has no column "NAME" for ROLE.` at the line of the header row. An index without a header must be below 1000.
- **Other columns**: the cells of a column without a role go to cue data `csv.columns`, keyed by header name, or by column index without a header. A short row is padded with `""` up to the header width.

| Role | Synonyms, in order |
|:--- |:--- |
| start | `begin`, `in`, `starttime`, `starttc`, `timecode`, `tcin` |
| end | `out`, `stop`, `endtime`, `endtc`, `tcout` |
| duration | `length` |
| speaker | `name`, `character` |
| text | `subtitle`, `caption`, `dialogue`, `transcript` |

### Read

- **Delimiter**: `,`, `;` or tab, from the option or from the first line that has one. Outside quotes, the most frequent wins, and a tie goes to `,`, then `;`.
- **Quoting**: a `"` at the start of a cell starts a quoted cell. `""` inside is one quote, and a line break inside becomes LF. Text after the closing quote appends to the cell. A `"` inside an unquoted cell is text.
- **Unclosed quote**: strict mode fails with `A quoted CSV cell has no closing quote.` at the line where the row starts. Lenient mode ends the cell at the end of its line, with a `Repaired` warning.
- **Blank rows**: a row whose cells are all blank is dropped.
- **Width**: a row of more than 1000 cells fails in both modes: `The table has 1001 columns. The limit is 1000.`
- **Rows before the header**: lenient mode drops the rows before the first row that has the needed columns, with one `Skipped` warning such as `The table has 2 rows before the header row.`
- **Row errors**: a row with a bad time fails in strict mode. Lenient mode skips it.
- **End before start**: fails the row in both modes, because a comma in a time or a cut-off last row gives such an end. The reader never swaps.
- **End**: the end cell, else the start plus the duration cell, at most 99999:59:59.999. Without both, the cue ends at the first later start of all cues, else at its start plus `LastCueDuration`, as [reading-writing.md](reading-writing.md#last-cue-default) says.

| Time shape | Example | Rule |
|:--- |:--- |:--- |
| seconds | `62.5` | digits with an optional fraction. Milliseconds in a millisecond table |
| frames | `00:01:02:12` | whole seconds plus the frames divided by `ReadOptions.CSV.FrameRate`. The frame number is not checked against the rate |
| clock | `00:01:02.500`, `00:01:02,500`, `1:02:03` | `h:mm:ss` with an optional `.` or `,` fraction of any length |
| minutes and seconds | `01:02.500`, `61:02` | `m:ss` with an optional fraction. The minutes may be 60 or more |
| one-digit seconds | `00:00:1`, `0:00:5.5` | a single digit after the last colon gets a `0` first |

- **Rounding**: every time rounds to whole milliseconds, then must be below 100000 hours: `The time "T" is not below 100000 hours.`
- **Frames without a frame rate**: `The time "T" counts frames and needs a frame rate.` The error has `Option` `ReadOptions.CSV.FrameRate`, and the CLI adds `Pass --fps or --input-fps.`
- **Other shapes**: `The time "T" is not seconds, h:mm:ss.mmm, m:ss.mmm, hh:mm:ss,mmm or hh:mm:ss:ff.`
- **Millisecond table**: when every non-empty start, end and duration cell is digits only, and the median cue duration is 100 or more, every time counts milliseconds. A duration is the end minus the start, else the duration cell. The median of an even count is the mean of the 2 middle values. The reader warns in both modes, for example `The times are whole numbers, and the median cue lasts 2550 seconds. The parser reads the times as milliseconds.` So `0,2340,Hello` reads as 0 to 2.34 s, and `1,4,Hello` stays in seconds.

| File data `csv` | Meaning |
|:--- |:--- |
| `delimiter` | the delimiter of the read |
| `header` | the header cells, null without a header row |
| `roles` | role to column index, for the roles found |
| `width` | the larger of the header width and the widest data row |
| `timeFormat` | the shape of the first read start: `hh:mm:ss,mmm` with a `,`, `hh:mm:ss:ff` with 3 colons, `hh:mm:ss.mmm` with a colon, else `seconds`. `milliseconds` for a millisecond table |
| `frameRate` | `ReadOptions.CSV.FrameRate`, null when zero |
| `quotedColumns` | the columns quoted in every data row, when there are any |

### Write

- **Columns of a read table**: the stored columns, roles and header. The other columns write the stored `csv.columns` cells. A table without a header row writes none.
- **Columns of other input**: `identifier` when a cue has an ID, then `start`, `end`, `text`, then the keys of the stored `csv.columns` of the cues, in first-seen order.
- **Speaker**: a `speaker` column goes before `text` when the first line of a cue starts with a `<v>` tag and the layout has no speaker column. The cell holds the name of that tag. Later `<v>` tags are lost.
- **Text**: the plain text of the lines, ruby as text and entities decoded, joined with LF.
- **Second text**: `SecondText` adds a column after `text` with the header `SecondTextHeader`. Each row gets the plain text of the cue of `SecondText` with the largest positive overlap. A second cue fills only the first row that picks it.
- **Quoting**: a cell that holds `"`, CR, LF or the delimiter gets quotes, and so does every cell of a stored quoted column. Each `"` doubles.
- **Formulas**: `EscapeFormulas` puts `'` before a cell that starts with `=`, `+`, `-` or `@`, header cells too. It is off by default, because dialogue lines start with `-`.
- **Line endings**: the rows end with the line ending of the write options. A line break inside a cell stays LF, as Excel writes it.
- **Lost on write**: markup, alignment, speakers after the first `<v>` tag, and the leading space of whisper.cpp texts.

| Time format | 62.5 s writes |
|:--- |:--- |
| `seconds` | `62.5`. 4 s writes `4` |
| `hh:mm:ss.mmm` | `00:01:02.500` |
| `hh:mm:ss,mmm` | `00:01:02,500` |
| `hh:mm:ss:ff` | `00:01:02:13` at 25 fps. A frame at or above the rounded frame rate moves to the next second as frame 0. A time that would write 100000 hours writes the last frame below it, such as `99999:59:59:24` at 25 fps |
| `milliseconds` | `62500` |

### Options

The root `ReadOptions.CSV` holds a `csv.ReadOptions` and `WriteOptions.CSV` a `csv.WriteOptions`. They serve CSV and TSV. Each zero value takes the stored value, else the default. The lenient rules are in [lenient-parsing.md](lenient-parsing.md#csv-and-tsv).

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `ReadOptions.CSV` | `Columns` | `csv.Columns` | zero: the header names, with synonyms | the column layout |
| `ReadOptions.CSV` | `Delimiter` | `csv.Delimiter` | detect from the first line | `,`, `;` or tab |
| `ReadOptions.CSV` | `FrameRate` | `FrameRate` | zero value: none | frames per second of `hh:mm:ss:ff` times |
| `WriteOptions.CSV` | `Delimiter` | `csv.Delimiter` | the stored delimiter, else `,` | `,`, `;` or tab |
| `WriteOptions.CSV` | `TimeFormat` | `csv.TimeFormat` | the stored time format, else `hh:mm:ss.mmm` | one of the 5 time formats |
| `WriteOptions.CSV` | `FrameRate` | `FrameRate` | zero value: the stored frame rate | frames per second of `hh:mm:ss:ff` |
| `WriteOptions.CSV` | `SecondText` | `*model.Subtitle` | nil: no second column | the cues of the second text column |
| `WriteOptions.CSV` | `SecondTextHeader` | `string` | `text2` | the header of the second text column |
| `WriteOptions.CSV` | `EscapeFormulas` | `bool` | false | put `'` before a formula cell |

- **Checks**: `Validate` rejects a negative column index (`The column index of start must be 0 or more, got -1.`), a column name or a missing index with `NoHeader` (`A table without a header row needs a column index for start.`), another delimiter (`The CSV delimiter must be ",", ";" or a tab.`), another time format, and a frame rate that is negative, NaN or infinite. Each error matches `ErrInvalidArgument` and names the option path, such as `ReadOptions.CSV.Columns.Start`.
- **Frames without a frame rate**: `TimeFrames` without a frame rate in the options or the file data fails at write time with `The time format hh:mm:ss:ff needs a frame rate.` and `Option` `WriteOptions.CSV.FrameRate`.
- **Stored frame rate**: a stored `frameRate` that is not above 0 fails at write time with, for example, `The frame rate must be greater than 0, got 0.` and the same `Option`, also for another time format.

## EBU STL

EBU STL is the binary subtitle exchange format of European broadcasters, defined in [EBU Tech 3264](https://tech.ebu.ch/docs/tech/tech3264.pdf). Package `format/ebustl` reads and writes it. The format name is `stl`, the extension `.stl`. EBU STL comes before Spruce STL in the registry, so it owns `.stl` in `FormatFromPath`.

A file starts with one GSI block (General Subtitle Information) of 1024 bytes. Each subtitle then takes one or more TTI blocks (Text and Timing Information) of 128 bytes. Example: a TTI block has time code in `00 00 01 05`, time code out `00 00 02 0A` and the text field `80 43 61 66 65 81 8A`. At 25 fps it reads as a cue from 1.2 s to 2.4 s with the line `<i>Cafe</i>`.

**Binary format**: the reader gets the raw bytes of the input, and the writer writes bytes that no line ending or BOM rule changes. It does no BOM removal, no UTF-8 check and no code page detection. `ReadOptions.Encoding` has no effect, `ReadInfo.SourceEncoding` is empty and the read has no decode warning. `Load` and `Parse` find the format by its signature on the raw bytes. `Load` also finds it by the extension `.stl`, as [detection.md](detection.md) says.

### Read

**Read option**: `ReadOptions.EBUSTL.SubtractStartOfProgramme`, default false. It subtracts the GSI field TCP (start of programme), such as `10:00:00:00`, from every cue time. A time before it becomes 0. TCP is 8 ASCII digits `HHMMSSFF`, else the offset is 0.

**Blocks**: a file under 1024 bytes fails. A partial last TTI block fails a strict read, and a lenient read cuts it. The DFC field (disk format code) must be `STL25.01` or `STL30.01`, which sets 25 or 30 fps. The CCT field (character code table) must be `00` to `04`. A file without TTI blocks gives an empty subtitle.

**GSI**: the reader decodes each of the 31 fields with the code page of the first 3 bytes (CPN). The code pages are 437, 850, 860, 863 and 865. Any other value reads as 850. Trailing spaces go, NUL bytes stay. OPT without trailing spaces and NUL bytes is the metadata `title`. LC (language code) gives the metadata `language` through the table of EBU Tech 3264 appendix 3. Example: `0F` gives `fr`. An unknown LC sets nothing.

**Sets**: consecutive TTI blocks with the same 2 subtitle number bytes form one set. A block with extension block number `FFh` closes the set, and a change of subtitle number closes it too. The reader joins the text fields of the blocks with EBN `00h` to `EFh` and `FFh`. It skips user data blocks (`FEh`) and the reserved numbers `F0h` to `FDh`. Then it removes every `8Fh` (unused space). The join comes before decoding. So an ISO 6937 diacritic at the end of one block combines with the letter at the start of the next. A diacritic just before a control code is dropped.

**Character codes**: CCT `00` decodes with ISO 6937. CCT `01` to `04` decode with ISO 8859-5, -6, -7 (1987) and -8 (1988). Bytes `20h` to `7Fh` and `A0h` to `FFh` are characters. A byte without an entry in the table is dropped, without a warning. Every other byte is a control code. It ends the run of characters. A code below `20h` also adds a space, because a teletext control code takes a character cell.

| Code | Effect |
|:--- |:--- |
| `00h` to `07h` | alpha color: black `#000000`, red `#ff0000`, green `#00ff00`, yellow `#ffff00`, blue `#0000ff`, magenta `#ff00ff`, cyan `#00ffff`, white. White gives no tag |
| `80h`, `81h` | italics on, off |
| `82h`, `83h` | underline on, off |
| `8Ah` | new row. The color goes back to white. Italics and underline go on into the next row |
| other `08h` to `1Fh` | a space only: flash, steady, end box, start box, normal height, double height, double width, mosaic colors, black background, new background |
| `84h`, `85h` (boxing) and other `80h` to `9Fh` | ends the run, no space, no style |

The reader writes each row as core markup and escapes its text. Example: `01 red 8A white` gives the lines `<font color="#ff0000">red</font>` and `white`. The model drops blank rows. Double height, boxing, flash, backgrounds and the content of user data blocks are not modelled.

**Comments**: a set whose first block has CF 1 is a comment, not a cue. Its rows are trimmed, empty rows dropped and the rest joined with LF. Tags go and entities decode. The comment stays before the next cue of the file, also when the cues are out of time order.

**Times**: the 4 time code bytes are hours, minutes, seconds and frames, read in the clock model of [frame-rates.md](frame-rates.md). `00 00 01 05` is 1.2 s at 25 fps and 1.167 s at 30 fps. A strict read takes every time code as written, also hour 255 or frame 200. TCS (time code status) is not read.

**Cumulative sets**: each subtitle of a cumulative set becomes its own cue with its own times. The text of earlier members is not added.

**Alignment and position**: JC (justification code) 1 is left, 3 right, and 0 or 2 center. The maximum row is 23 when DSC (display standard code) is `1` or `2`, teletext level 1 or 2. Else it is MNR (maximum number of rows) when above 0, else 23. MNR and MNC are read like a PHP `(int)` cast, so `2x` is 2. VP (vertical position) gives the row band: top when `3*VP < max`, middle when `3*VP < 2*max`, else bottom. Examples: teletext VP 1 with JC 1 is `TopLeft`, VP 12 with JC 3 is `MiddleRight` and VP 22 with JC 2 is `BottomCenter`.

`Position.Row` and `Position.Grid` follow EBU Tech 3264. Teletext rows count from 1, so `Row` is VP and `Grid.Rows` is 23. Open subtitles (DSC `0`) count VP from 0. So `Row` is VP plus 1 and `Grid.Rows` is the maximum row plus 1. `Grid.Columns` is MNC when above 0, else 40. A VP outside the grid gives a cue with an alignment but no row, no column and a zero grid. Examples: teletext VP 0, and open VP 30 with MNR 23.

**Known edge**: teletext VP 8 gets `MiddleCenter` with row 8 of 23. `Position.RowOn` puts row 8 in the top band, so a writer that calls it drops the row of such a cue. No other teletext VP is affected.

**Strict and lenient**: a time code is bad when hours are above 23, minutes or seconds above 59, or frames at or above the frame rate. A strict read keeps the cue as it is, without a warning. A lenient read skips the whole set with a warning. So strict mode is more permissive than lenient mode here. [lenient-parsing.md](lenient-parsing.md#ebu-stl) lists every error and warning. Binary formats have no line numbers, so every warning has `NoLine`. `Block` is the index of the first TTI block of the set, counted without the GSI block.

**Format data**: the file key `stl` holds these fields.

- `gsi`: all 31 GSI fields by name, `spare` and `UDA` too.
- `startOfProgrammeSubtracted`: the read option.
- `firstSubtitleNumber`: SN of the first set, or null without sets.
- `comments`: a list of `{text, blocks}`.
- `counts`: `TNB` is the TTI block count after a cut. `TNS` is the cue count. `TNG` is the count of distinct group numbers of cues. `TCF` is the 8 TCI digits of the first cue before any subtraction, or `00000000`.

The cue key `stl` holds `subtitleGroupNumber`, `cumulativeStatus`, `verticalPosition` and `justificationCode` of the first block. It also holds `text`, the cue text at read time. And it holds `blocks`, the hex of every block of the set, 256 hex digits each.

### Write

`ebustl.Write` writes the GSI block, then the TTI blocks of each comment and cue. `WriteOptions.EBUSTL.FrameRate` is 25 or 30 fps. The zero value takes the stored DFC, else 25. Another rate fails `Validate` with `The EBU STL formatter writes 25 or 30 fps, got 24.`. The writer reads `StripTags` and ignores `LineEnding` and `BOM`.

**Byte for byte**: a file that the reader read comes back with the same bytes, also after a retiming. A cue whose text did not change keeps its stored blocks, and the writer patches only the changed header fields (SGN, CS, TCI, TCO, VP, JC). When the text changed, the writer keeps the stored user data blocks and encodes the text again. A subtitle from another format gets a new GSI block. The writer writes the sets in cue order, with the comments before the cue they sit in front of. So a file with sets out of time order comes back reordered and renumbered.

**New GSI values**: CPN `850`, DSC `1`, RN `00`, MNC `40`, MNR `23`, TCS `1`, TCP `00000000`, TND `1` and DSN `1` fill a missing field. CD and RD hold today in UTC as `yymmdd`. OPT is the metadata `title`, LC is the code of the metadata `language` (`en-GB` gives `09` through the primary subtag `en`, no language gives `00`), TNB, TNS, TNG and TCF are the counts of the written blocks. A stored count stays when it still equals the count that the reader stored, so a count padded with spaces keeps its bytes. MNC grows to the longest row in displayable bytes, up to 99. Rows never wrap. A field is encoded with the code page of CPN, padded with spaces and cut to its length.

**Character code table**: the stored CCT stays. Without one, the writer picks the table that writes the fewest `?` for the cue and comment text. It tries 00, 01, 02, 03 and 04 in this order, and table 00 wins a tie. The count is the number of `?` bytes in the encoded text, so a `?` in the text counts too. A stored CCT outside `00` to `04` fails with `The character code table "05" is not 00, 01, 02, 03 or 04.`.

**Text**: each cue line is one row, and rows join with `8Ah`. `<i>` and `<u>` write `80h` to `83h`, and only the first open and the last close of nested tags write. A `<font>` or WebVTT color class with one of the 8 teletext colors writes `00h` to `07h`, and its end tag writes the color below it, white when none. Another color writes nothing. A color code replaces one space next to it. A color code at the row end goes. Italics or underline still open at the row end get `81h` or `83h`. A style open at a row end starts again on the next row. Bold, strike, voice, ruby annotation tags and positions of WebVTT write nothing, and ruby writes as `base (annotation)`. Double height, boxing and backgrounds that the reader dropped are lost when a cue text changes. They survive only through reused blocks. A text field has 112 bytes padded with `8Fh`. The last field always holds one `8Fh`, so 112 bytes of text give a second, empty block. In table 00 a field that would end with a diacritic ends one byte early. A cue that needs more than 241 blocks fails with `The cue text needs more than 241 TTI blocks.`.

**Subtitle numbers**: the first set takes `firstSubtitleNumber`, else 1, and each later set, comment sets too, takes the next number. A number above 65535 fails with `EBU STL allows subtitle numbers up to 65535.`.

**Times**: the writer takes the frame nearest to the time, at 25 or 30 fps, and caps hours at 255. With `startOfProgrammeSubtracted` in the file data, the stored TCP is added back. The writer reads TCP at the rate of the stored DFC. PHP reads it at the output rate, so a file read at 25 fps and written at 30 fps differs.

**Position**: the stored VP and JC stay when they still give the alignment and the row of the cue. Else the writer calls `Position.RowOn` on the grid of the file, 23 rows for teletext, else the maximum row plus 1, and writes that row as VP when it falls in the band of the alignment. Else the first row goes at the top, in the middle or at the bottom of the rows. Example: a one-line `BottomCenter` cue without a row writes VP 23 and JC 2. `RowOn` drops the row of 2 cues that the reader gives a row: teletext VP 8 read as `MiddleCenter`, and SCC row 5 with `BottomCenter`. After a conversion from SCC, or after a change of `Position.Row`, these 2 fall back to the alignment. A rewritten EBU STL file keeps VP 8, because the stored VP and JC stay.

**Report**: `WriteReport.EBUSTL` is non-nil after every write. `CharacterCodeTable` is the table, `TableChosen` is true when the writer picked it, and `Replaced` lists each character written as `?` in output order. A GSI field has its name in `Field`, a cue its index in `CueIndex`, and a comment or a GSI field has `CueIndex` -1. Example: an em dash and the guillemets in cue 0 give 3 entries.
## Cavena 890

Cavena 890 is the binary subtitle file of the Cavena systems, which Scandinavian broadcasters use. Package `format/cavena890` reads and writes it. The format name is `cavena890`, the extension `.890`. The format has no public spec and no signature, so `Detect` never finds it. `Load` finds it by the extension `.890`. `Parse` needs `ReadOptions.Format`, and without it returns `ErrUnknownFormat`.

A file holds a header of 388 bytes, then blocks of 128 bytes. Each block has 2 text lines of 51 bytes. Example: a block that starts with `00 10 00 00 19 00 00 64 50` is subtitle 16 from 1 s to 4 s, aligned bottom left. The frames 25 and 100 at 25 fps give the times. Its lower line `88 47 72 86 75 DF 65 98 20 E5 78 E6` reads as `<i>Gr` U+00FC U+00DF `e</i> [x]`, which is the italic word "Gruesse" with its umlaut and sharp s, then `[x]`.

**Binary format**: the reader gets the raw bytes of the input. `ReadOptions.Encoding`, `LineEnding` and `BOM` have no effect, and `ReadInfo.SourceEncoding` is empty. Because `.890` is an extension of binary formats only, `Load` picks the format before it decodes the bytes. The options of the format are empty structs.

```text
file   = header(388) block(128)*. The last block may lack its last 4 bytes (short end)
header : 146 language code of line 1, 147 language code of line 2,
         179, 208, 210 block count (uint16 little-endian, written 3 times),
         187 font name 1, 246 font name 2 (the reader reads 6 bytes of each)
block  : 0   subtitle number (uint16 big-endian)
         2   start frame (3 bytes big-endian, 25 fps)
         5   end frame (3 bytes big-endian)
         8   alignment code (50h left, 52h right, 54h center written)
         9   7 x 00h
         16  line 1 (51 bytes, filler 7Fh, Chinese filler 00h)
         67  6 x 00h
         73  line 2 (51 bytes)
         124 4 x 00h (block end)
```

### Variants

The language codes and the font names of the header pick the text table, called the variant. The first row that matches wins.

| Variant | Header | Text |
|:--- |:--- |:--- |
| `hebrew` | language 1 is `8Fh`, or a font starts with `HEBNOA` | Hebrew letters on the ASCII letter bytes, Latin letters on `91h` to `CAh`. Runs and line-end punctuation are in display order |
| `greek` | language 1 is `70h`, or a font starts with `GREEK` | Greek letters on the ASCII letter bytes, Latin letters on `91h` to `CAh`. `D9h` is the 2 characters U+03B4 U+03B9 |
| `arabic` | language 2 is `80h`, or font 1 is exactly `ARABIC` | Arabic letters on the ASCII letter bytes |
| `russian` | language 1 is `56h`, or a font starts with `KYRIL` | Cyrillic letters on the ASCII letter bytes |
| `chinese` | language 1 is `90h` or `91h`, or a font starts with `CCKM44` | UTF-16BE |
| `scandinavian` | language 1 is `07h` (Danish), `1Eh` (Norwegian) or `28h` (Swedish) | as `latin`, but `E5h` and `E6h` stay U+00E5 and U+00E6 |
| `latin` | anything else | Windows-1252 with 2-byte accent codes |

The Latin table maps single bytes. `5Bh` is U+00C6, `5Ch` is U+00D8 and `5Dh` is U+00C5. `E2h` is `@`, `E5h` is `[` and `E6h` is `]`. `1Bh` is U+00E6, `1Ch` is U+00F8 and `1Dh` is U+00E5. `1Eh` is U+00C6, `1Fh` is U+00D8 and `EBh` is U+266A. An accent code has 2 bytes, the accent first and the base letter second. The accents are `81` grave, `82` acute, `83` circumflex, `84` caron, `85` tilde, `86` diaeresis, `87` cedilla, `89` breve, `8C` ring, `8E` dot and `8F` ogonek. So `86 75` is U+00FC. The tables of the 5 variants come from Subtitle Edit, see [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

### Read

**Errors**: a file under 388 bytes fails with `A Cavena 890 file starts with a header of 388 bytes.` A body whose length leaves 124 bytes after the last whole block has a short end. The reader adds 4 zero bytes and stores `shortEnd` true. Any other body length that is not a multiple of 128 fails a strict read with `The blocks of a Cavena 890 file have 128 bytes each.` A lenient read cuts the rest and warns, see [lenient-parsing.md](lenient-parsing.md#cavena-890).

**Cues**: consecutive blocks with the same 2 number bytes form one cue, with 2 lines per block. A line that is blank after decoding and trimming is dropped, so a cue with text in the lower line only has 1 line.

**Times**: the start and end frame of the first block, divided by 25. The reader checks nothing and never warns about a time.

**Alignment**: `50h` is `BottomLeft`, `52h` is `BottomRight`, and every other code is `BottomCenter`. The format has no row, so the reader sets no `Position.Row`.

**Text**: the reader decodes each line in this order.

1. Chinese drops trailing `00h` and `7Fh` bytes, takes the next byte for an odd length, and reads UTF-16BE code units. So a character with the low byte `7Fh` at the line end stays. A valid surrogate pair is one character, and a lone surrogate gives nothing. The other variants read single bytes.
2. `88h` starts and `98h` ends italics, in `latin`, `scandinavian` and `chinese` only. They split style runs.
3. A 2-byte code, from the byte and the next byte, is tried first. Then the 1-byte code.
4. A byte without a table entry is `-` for `BEh`, and nothing for a code below `20h`, `7Fh` and the color codes `F1h` to `F8h`. Chinese turns the code unit into its character. Other variants read the byte as Windows-1252, and the bytes that Windows-1252 leaves undefined (`81h`, `8Dh`, `8Fh`, `90h`, `9Dh`) give nothing.
5. Hebrew reverses each run of characters up to U+00FF. Then it moves the punctuation from the line start to the end and back, with mirrored brackets, because Subtitle Edit stores Hebrew lines in display order.
6. The reader writes the runs as core markup with the white space outside the tags, and removes trailing white space.

Example: the bytes `BE 20 Red 20 F1 fox F7 20 00 1D` read as `- Red fox` and U+00E5, because `1Dh` is U+00E5 in the Latin table.

**Format data**: the file key `cavena890` holds `variant`, `header` (hex of the 388 bytes) and `shortEnd`. The cue key `cavena890` holds `text`, the cue text at read time, and `blocks`, the hex of each block of the cue. Neither key has a schema, so `SetFormatData` accepts any value, and the writer checks each value it reads.

### Write

**Variant and header**: the writer takes the stored `variant` when it names a known value, else `latin`. It takes the stored `header` when it decodes to 388 bytes, else the header of Subtitle Edit for a new file: English language codes and the font `HLV23N.V`. A value that does not fit counts as missing.

**Times**: `t * 25` frames rounded half away from zero, with a negative time as 0 and at most `FFFFFFh` (186 hours). 3 bytes, big-endian.

**Alignment**: left column `50h`, right column `52h`, else `54h`. The row is not written, so top and middle alignments become bottom.

**Stored blocks**: a cue that has `text` and `blocks` in its format data, with `StripTags` off and its text unchanged, writes its stored blocks. Bytes 2 to 8 hold the new times and alignment. The stored alignment byte stays when it means the same alignment. So a file that Read returned writes back byte for byte.

**New blocks**: the writer takes the lines after `RubyAsText` and `CarryOpenTags`, 2 per block. A last single line goes into the lower line. A cue without lines writes one empty block.

**Line encoding**: entities are decoded. `<i>` and `</i>` write `88h` and `98h` in `latin`, `scandinavian` and `chinese`, and Chinese writes `00h` before the code. `StripTags` drops them. Every other tag is dropped. Chinese text is UTF-16BE, with a surrogate pair for a character above U+FFFF. Hebrew text gets the punctuation move, then the run reversal. For other text, each character takes the first rule that matches.

1. A table code for 2 characters.
2. Plain ASCII `20h` to `7Eh`, when the table does not use that byte.
3. The 1-byte or 2-byte table code. The first code of a character wins.
4. The Windows-1252 byte, when the variant reads that byte back as the same character.
5. `?`.

The writer pads a line to 51 bytes with `7Fh`, or `00h` for Chinese.

**Numbers**: a stored number stays when it is greater than the number before it. Otherwise the cue gets the number before it plus 16, as Subtitle Edit writes them, or plus 1 when plus 16 passes 65535. All blocks of a cue get the same number. A new file starts at 16. When 65535 is used up, the write fails with a `*UnwritableError` for the cue and the message `Cavena 890 allows subtitle numbers up to 65535.` So a new file holds up to 4110 cues: 4095 from 16 to 65520, then 15 from 65521 to 65535.

**Counts and short end**: the writer sets the block count, at most 65535, at offsets 179, 208 and 210. When the file has blocks and the stored `shortEnd` is true or missing, the last 4 bytes are cut. A new file is short, as Subtitle Edit writes it.

**Line limit**: a line of more than 51 bytes fails with a `*UnwritableError` for the cue. Example: `Cavena 890 holds 51 bytes per line, got 52 for "LINE".` LINE is the line in core markup. There is no wrap and no fit option. Wrap long lines to 40 characters first with `WrapLines`, see [editing.md](editing.md#line-wrapping).

**Lost on write**: top and middle rows, colors, bold and underline, and italics in `hebrew`, `russian`, `greek` and `arabic`. A character outside the table becomes `?`. Example: a Russian file with cue 0 set to the Russian word "Da", a comma and `Ivan.` reads back with `????` in place of `Ivan`. The Russian table has no Latin letters.

### Options

None. `ReadOptions` and `WriteOptions` of the package are empty structs, and the root options have no field for the format. The writer reads `CommonWriteOptions.StripTags`. `LineEnding` and `BOM` do not apply to a binary format.

## PAC

PAC is the binary subtitle file of the Screen Electronics systems. PAC Unicode (`.fpc`) is its form with UTF-8 text. Package `format/pac` reads and writes both. The format name is `pac`, the extensions are `.pac` and `.fpc`. PAC has no public spec. The code follows `Pac.cs` of Subtitle Edit, see [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md).

```text
file   = header(20) [story record] record* footer
header = 01h + 19 x 00h
record = 00h | SN (uint16 LE) | mark 60h..67h | TCI (4) | TCO (4) | LEN (uint16 LE) | VP (1) | text (LEN - 1 bytes)
TC     = uint16 LE (hours*100 + minutes) | uint16 LE (seconds*100 + frames)
text   = junk* line*
line   = FEh | alignment byte | one more byte (03h when written) | content
footer = FFh ... (default: FFh, 11 x 00h, 11h, 00h, "dummy end of file")
```

Example: a record has the subtitle number 0, the time codes `00 00 64 00` and `00 00 38 01`, vertical position 10 and the line bytes `FE 02 03 Fresh rolls are<warm>today.`. At 25 fps it reads as a bottom-center cue from 1 s to 3.48 s whose first line is `Fresh rolls are <i>warm</i> today.`.

**Binary format**: the reader gets the raw bytes of the input. `ReadOptions.Encoding` does not decode the file, `ReadInfo.SourceEncoding` is empty, and the writer ignores `LineEnding` and `BOM`. `Load` and `Parse` find PAC by its signature on the raw bytes, see [detection.md](detection.md).

### Code pages

The file does not store its code page. `pac.CodePage` is a string type with these values. The reader stores the value in the file data `pac.codePage`.

| Value | Single-byte table | Double-byte text | Encoding names that select it |
|:--- |:--- |:--- |:--- |
| `latin` | Latin | | ISO-8859-1, ISO8859-1, LATIN1, Windows-1252, CP1252 |
| `latin-czech` | Latin | | ISO-8859-2, ISO8859-2, Windows-1250, CP1250 |
| `latin-turkish` | Turkish over Latin | | ISO-8859-9, ISO8859-9, Windows-1254, CP1254 |
| `greek` | Greek | | ISO-8859-7, ISO8859-7, Windows-1253, CP1253 |
| `cyrillic` | Cyrillic | | ISO-8859-5, ISO8859-5, Windows-1251, CP1251 |
| `arabic` | Arabic | | ISO-8859-6, ISO8859-6, Windows-1256, CP1256 |
| `hebrew` | Hebrew | | ISO-8859-8, ISO8859-8, Windows-1255, CP1255 |
| `thai` | Thai | | Windows-874, CP874, ISO-8859-11, ISO8859-11 |
| `chinese-traditional` | Latin | Big5 | Big5, CP950 |
| `chinese-simplified` | Latin | GBK | GBK, CP936, GB18030 |
| `korean` | Latin | EUC-KR table of CP949 | CP949, UHC, EUC-KR |
| `japanese` | Latin | Shift_JIS | Shift_JIS, SJIS, CP932 |
| `unicode` | Latin | | UTF-8, UTF8 |

- **Choice**: the first rule that holds decides. 1. `ReadOptions.PAC.CodePage`. 2. The `LANG:XXX` code of the story record, in any case: DAR, OAR, ARA and ARB give `arabic`. ISR and HEB give `hebrew`. GRC, GRE and ELL give `greek`. DRU, RUS, BGR, MKD, MNG and UKR give `cyrillic`. THA gives `thai`. TRD, OCH and CHT give `chinese-traditional`. SIM and CHS give `chinese-simplified`. DJA and JPN give `japanese`. DKO and KOR give `korean`. TUR gives `latin-turkish`. CSK and CZE give `latin-czech`. 3. The detector.
- **Encoding option**: `stbox.Parse` and `stbox.Load` set `PAC.CodePage` from `ReadOptions.Encoding` when `PAC.CodePage` is empty. `pac.CodePageFromEncoding` maps the canonical name of [encodings.md](encodings.md), so the names in the last column work and `Encoding` accepts them in any case. A name that no PAC code page uses, such as `KOI8-R`, fails with an `*InvalidArgumentError` for `ReadOptions.Encoding`. `PAC.CodePage` wins when both are set, and `pac.Read` never sees `Encoding`. The spellings `TIS-620`, `BIG-5`, `GB2312`, `LATIN2` and `LATIN5` are no encoding names in this library, so `Encoding` rejects them before PAC sees them.
- **Detector**: the detector decodes the text of all cues with each candidate and checks that the result looks like the script of the code page. Any PAC Unicode run gives `unicode`. Double-byte runs give the candidates `korean`, `japanese`, `chinese-simplified` and `chinese-traditional`. Else the candidates are `latin`, `latin-czech`, `latin-turkish`, `greek`, `cyrillic`, `hebrew`, `arabic` and `thai`. A candidate that leaves more than 1 % of the characters undefined gets no fit. The other checks are in the godoc of the package. When the best fit is a tie between Latin pages, `textenc.ScoreText` decides if the top score leads by 0.5.
- **Unclear code page**: a strict read fails with `The code page of the PAC file is not clear. REASON Set the code page of the file, for example ISO-8859-7 for Greek.`. A lenient read warns with the same text and ` The parser read it as VALUE.` as a `Repaired` warning without a line or block. Examples of the reason: `No code page gives text in its script.` for the bytes `C3 C4`, and `These code pages fit: greek, cyrillic.` for the bytes `Lf`.
- **Double-byte tables**: the decoders of the 4 double-byte pages are the `golang.org/x/text` decoders. They follow the WHATWG indexes, which differ from iconv in a few codes. The Big5 and Shift_JIS writers do not use the x/text encoders. x/text writes Hong Kong extension codes such as `FABDh` for U+5305, which CP950 readers cannot read, and the NEC selected IBM extension where CP932 writes the IBM extension. These 2 writers use a table of the lowest CP950 or CP932 code of each character, built from the x/text decoder, plus the 7 one-way CP932 codes such as U+301C to `8160h`. The GBK and CP949 writers use x/text. A character that has no 2-byte code, or whose code starts with `FEh`, is written as `00h ?`. Each of the 13 fixtures reads as in the PHP library and writes back byte for byte.

### Read

| Input | Goes to |
|:--- |:--- |
| a record whose text holds `STORY:`, in any case, as the first record | no cue. Its bytes join the stored header |
| `FEh` | starts a line. The next byte is the alignment byte. The 3 bytes from `FEh` on are skipped, and the mode goes back to single-byte. An `FEh` that starts a character starts a line, also in double-byte text. A trail byte `FEh` does not, so `A5FEh` in Big5 is one character |
| `1Fh W16.` | double-byte mode |
| `1Fh EFh BBh BFh` | Unicode mode, which ends at the next `.`. `FFh` in it reads as `.` |
| `1Fh C` and a digit | a 3-byte code, skipped |
| `1Fh C` and another byte | a comment up to the end of the text. The split stops |
| bytes before the first `FEh` | no text |
| `<` and `>` | italics on and off. The end of a line ends italics |

- **Single-byte decoding**: a table code has 1, 2 or 3 bytes, high byte first, so `E2 65` is `e` with an acute accent. A Latin code page looks up a 2-byte code first, unless a code also starts at the second byte, so `L` before an accented letter stays `L`. Other code pages try 3, 2, then 1 byte. `FFh` is a space. `20h` to `7Eh` without a table entry is ASCII. `A0h` and up in a Latin code page goes through ISO-8859-1, -2 or -9. Anything else, such as the colors `01h` to `07h`, gives no text.
- **Latin table**: it moves some ASCII bytes. `23h` is the pound sign, `5Ch` the capital AE ligature, `7Ch` the small ae ligature and `80h` is `#`. The Greek and Cyrillic tables put their letters on the ASCII letter bytes. A byte from `80h` without an entry in a code page that is not Latin counts as undefined for the detector.
- **Arabic marks**: a vowel mark byte (`E0h` to `EAh`) right before a letter byte from `80h` gives the letter, then the mark.
- **Double-byte runs**: a byte below `80h` is one ASCII character, and `00h` escapes the next byte. Only `20h` to `7Eh` give text. Other bytes go in pairs through the decoder of the code page, or Big5 for a code page without one. A pair that the decoder does not define gives no text.
- **Unicode runs**: UTF-8 with `FFh` as a period. Each invalid sequence becomes `?`, as PHP `mb_scrub` writes it.
- **Right to left**: for Arabic and Hebrew lines the reader swaps `(` with `)` and `[` with `]` after decoding, then reverses each run of Latin characters. For Arabic a run is digits and ASCII letters of both cases. For Hebrew it is digits and lower-case ASCII letters only, as in Subtitle Edit.
- **Italics**: a space goes in before `<` after a letter or digit, and after `>` before a letter or digit. Then the runs become `<i>` tags. `Go<now>, he said<quietly` gives `Go <i>now</i>, he said <i>quietly</i>`.
- **Not read**: colors, and a second code page that Screen marks for some lines.
- **Frame rate**: 30 fps when any TCI or TCO of any record has a frame number from 25 to 29, else 25 fps.
- **Times**: the clock model of [frame-rates.md](frame-rates.md). Hours = first number / 100, minutes = first number % 100, seconds = second number / 100, frames = second number % 100. Hours have no upper check, up to 655. A cue with minutes or seconds above 59, or frames at or above the frame rate, is an error, see [lenient-parsing.md](lenient-parsing.md#pac).
- **Alignment and position**: VP 0 to 4 is the top row, 5 to 8 the middle and 9 and up the bottom. The alignment byte of the first line, masked with `03h`, gives the column: 1 left, 0 right, 2 and 3 center. A record without lines is centered. `Position.Row` is VP plus 1 on a grid of 12 rows. A VP above 11 gives an alignment but no row, no column and a zero grid. `Column` stays 0.
- **Format data**: the file key `pac` holds `codePage`, `frameRate` (25 or 30), `firstNumber` (SN of the first record after the story, 0 without records), `header` (hex of the 20 header bytes and the story record) and `footer` (hex of the bytes after the last record). The cue key `pac` holds `verticalPosition`, `alignment` (the raw alignment byte of the first line), `text` (the cue text at read time) and `bytes` (hex of the text bytes after VP).

### Write

- **Code page**: the stored `codePage` when it names a known value. Else `latin` when the Latin encoder writes no `?` for any cue, else `unicode`. A new file with text that Latin cannot hold always becomes PAC Unicode, never Greek, Cyrillic or a CJK code page.
- **Frame rate**: the stored `frameRate` when it is 25 or 30, else 25. No option changes it.
- **Numbers**: the stored `firstNumber` plus the cue index, else from 0. A number above 65535 gives an `*UnwritableError` with `PAC allows subtitle numbers up to 65535.`. Text of 65535 bytes or more gives one with `PAC allows cue text of up to 65534 bytes.`.
- **Time code**: the frame of the time at the frame rate, with hours capped at 654, written as `hours*100 + minutes` and `seconds*100 + frames`. A negative time is 0.
- **Reuse**: the writer keeps the stored vertical position and bytes of a cue when all of these hold. The cue has stored `verticalPosition`, `alignment`, `text` and `bytes`. `StripTags` is off. The text is unchanged. The cue still has the alignment and the `Position.Row` that the reader gave it. So an unchanged file writes back byte for byte.
- **New text, vertical position**: `Position.RowOn` on a grid of 12 rows, minus 1, when the band of that row matches the alignment row. Else the top is 1, the middle 6 and the bottom `max(9, 12 - lines)`, and the stored VP stays when the line count is unchanged and the stored VP is in the same band.
- **New text, lines**: each line is `FEh`, the alignment byte (left 1, right 0, center 2), `03h` and the encoded line. PAC Unicode starts the text with `80h 80h 80h`, as Subtitle Edit writes it.
- **Line text**: ruby becomes text and open tags carry to the next line. Entities decode. A literal `<` or `>` becomes `?`. `<i>` writes `<` and `</i>` writes `>`, with nesting counted. Other tags drop, all tags with `StripTags`. One space before `<` after a letter or digit, and one space after `>` before a letter or digit, goes, and a `>` at the line end goes. `U+2026` becomes `...` except in PAC Unicode. Arabic and Hebrew lines get the right-to-left order.
- **Encoding a line**: PAC Unicode writes `1Fh EFh BBh BFh`, the UTF-8 text with `.` as `FFh`, then `.`. A double-byte code page and a line with a character outside `20h` to `7Eh` writes `1Fh W16.`, then per character the 2-byte code, or `00h` and the ASCII byte, or `00h ?`. Else the line is single-byte: the longest table match of 3, 2 or 1 characters, where the first code of a character wins. In a Latin code page a character outside the table falls back to its ISO-8859-1, -2 or -9 byte from `A0h`, else `?`. In Arabic, a vowel mark goes before its letter.
- **Header and footer**: the stored hex when it decodes. A stored header must be the 20 header bytes, optionally followed by one complete record. A stored footer must be empty or start with `FFh`. Else the default header and footer.
- **Bad stored values**: an unknown stored code page, a frame rate other than 25 or 30, a negative number, a header that is not the 20 header bytes and a complete story record, a footer that does not start with `FFh`, and hex that does not decode count as missing.
- **Lossy**: bold, underline, colors, voice and fonts are lost. A literal `<` or `>` becomes `?`. A character outside the stored code page becomes `?`. A Greek file with the word Kalimera in Greek letters and 2 Han characters reads back with the Greek word and `??`.

| Alignment, lines | VP | Alignment byte |
|:--- |---:|---:|
| `BottomCenter`, 2 | 10 | 2 |
| `BottomLeft`, 1 | 11 | 1 |
| `MiddleRight`, 1 | 6 | 0 |
| `TopCenter`, 3 | 1 | 2 |

### Options

`pac.ReadOptions` has one field. The root `ReadOptions.PAC` holds it. `pac.WriteOptions` has no field.

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `CodePage` | `pac.CodePage` | `CodePageAuto` | the code page of the text. `CodePageAuto` takes the story record language, else detects it |

- **Shared fields**: `Lenient` applies. `LineEnding` and `BOM` have no effect.

## PGS

PGS (Presentation Graphic Stream) is the bitmap subtitle format of Blu-ray discs, stored as `.sup` files. Package `format/pgs` reads and writes it. The format name is `pgs`, the extension is `.sup`. The code follows the sources in the package comment of `format/pgs`. It copies no code of FFmpeg or libbluray.

A **display set** is the group of segments from one presentation composition segment (PCS) to its end segment. It changes the screen at one time. Each display set that shows objects gives one image cue. Example: `shapes_1080p.sup` starts with a display set at PTS 90000. It shows a 640x90 object at x 640, y 940 on a 1920x1080 screen. A display set at PTS 315000 clears it. The reader returns one image cue from 1 s to 3.5 s with that bitmap as a PNG. The cue has no lines, so [OCR](ocr.md) can fill them.

**Binary format**: the reader gets the raw bytes of the input. `ReadOptions.Encoding` does not decode the file and `ReadInfo.SourceEncoding` is empty. `Load` and `Parse` find PGS by its signature on the raw bytes, see [detection.md](detection.md).

### Segments

Each segment has a 13-byte header, then `size` bytes of data. All integers are big endian. The header holds the magic `PG`, the PTS (ticks of a 90 kHz clock) at byte 2, the DTS (read and ignored) at byte 6, the segment type at byte 10 and the data size at byte 11.

| Type | Name | Fields read |
|:--- |:--- |:--- |
| `0x16` | PCS | screen width and height, frame rate (ignored), composition number (ignored), composition state, palette update flag (ignored), palette id, object count. Then per object: object id, window id, flags (`0x80` cropped, `0x40` forced), x, y. A cropped object adds crop x, crop y, crop width and crop height |
| `0x17` | WDS | window count, then per window: id, x, y, width, height. Only complete 9-byte entries count |
| `0x14` | PDS | palette id, palette version (ignored), then 5-byte entries: entry id, Y, Cr, Cb, alpha. An incomplete last entry is ignored |
| `0x15` | ODS | object id, version (ignored), sequence flags. The first fragment (`0x80`) adds a 3-byte data length (ignored), width, height and run-length data. A later fragment appends its run-length data to the object of the same id. A later fragment for an unknown id is dropped. An ODS with less than 4 bytes of data is ignored |
| `0x80` | END | ends the display set |
| other | | skipped |

### Display sets

- **Epoch start**: a PCS with the composition state bit `0x80` clears the palettes, objects and windows. The acquisition point state (`0x40`) and the normal state (`0x00`) keep them. So a later display set can update only the palette, or show an object of an earlier display set again.
- **End**: a display set ends at its END segment, at the next PCS and at the end of the input. It shows nothing when no palette has the PCS palette id or when no object is left to draw.
- **Objects**: a reference to an unknown object, or with an empty visible area, is skipped. Several objects go on one transparent RGBA image. An object that overlaps an earlier one is drawn over it.
- **Visible area**: the crop rectangle when the reference is cropped, else the whole object, cut to the object size and to the window of the reference. A cropped object keeps the screen position of the PCS, not that position plus the crop offset. This is the libbluray behaviour.
- **Forced**: `Cue.Forced` is true when at least one drawn reference has the flag `0x40`.
- **Image**: one object gives a palette PNG with the used colours only. Two or more objects give an RGBA PNG. The image is at the top of the screen (`TopCenter`) when its centre is in the top third, else `BottomCenter`.
- **Same image**: when a display set draws the image of the shown cue again, the cue goes on. A palette update of an entry that no pixel uses keeps the cue.
- **Times**: the cue starts at the PTS of the PCS that shows it and ends at the PTS of the PCS that replaces or clears it. A PTS becomes whole milliseconds, half up. A cue never ends before it starts. The last cue ends after `CommonReadOptions.LastCueDuration`, 5 s by default. 0 gives a cue of length 0.
- **Run-length data**: a byte other than `00h` is one pixel. `00h` and a flags byte F give a run of `F & 0x3F` pixels. With `F & 0x40` the length is `(F & 0x3F) * 256` plus the next byte. With `F & 0x80` the colour is the next byte, else 0. Rows do not align on the end-of-line code `00h 00h`. The decoder stops at `width * height` pixels and cuts the extra ones. A missing byte at the end reads as 0.
- **Colours**: the palette holds limited range Y, Cr, Cb codes. The reader converts them to RGB with BT.601 up to a screen height of 576 lines and with BT.709 above. An entry that the palette does not define is `0x00000000`. The alpha byte is the alpha of the pixel.
- **Limits**: an object over 7680 pixels per side or 8294400 pixels in total is an error. So is a display set whose joined image is over the same limits.

Example: `shapes_576p.sup` cut to 1400 bytes reads 1 cue from 0.5 s to 3 s in lenient mode and warns `The segment header at byte 1398 is cut off.` with block 8.

### Options

`pgs.ReadOptions` has no field, and the root `ReadOptions` has none for PGS. `Lenient` and `LastCueDuration` apply. `LineEnding` and `BOM` have no effect.

### Lenient mode

Damage in the segment framing stops the read. The cues before it stay, and the display set that was open shows nothing. Damage inside a complete segment skips only its display set. The shown cue ends at the PCS time of that display set and reading goes on, so one bad object does not lose the other cues. [lenient-parsing.md](lenient-parsing.md#pgs) lists the messages. Strict mode fails on the first damage, as PHP does.

### Writing

`pgs.Write` writes image cues as a `.sup` stream, the inverse of the reader. Every cue must hold an image. The writer ignores the lines of an image cue. A cue without an image gives an `*UnwritableError`.

Example: cue A from 1.5 s to 4 s and cue B from 4 s to 5 s give 3 display sets. The first shows A at 1.5 s. The second shows B at 4 s, with no clear for A. The third clears the screen at 5 s.

- **Display set plan**: the writer keeps a list of shown cues. Before a cue starts, each shown cue that ends earlier gets a display set at its end time. That display set shows the cues that remain, or clears the screen when none remain. A cue that ends at the start of the next cue leaves without a display set, because the next display set replaces it. At most 2 cues show at once. A third cue drops the cue that started first. A cue with the same start as the next cue waits, so both share one display set, unless it ends at its own start.
- **Show display set**: it is an epoch start (state `0x80`). Its segments all have the same PTS and a DTS of 0, in this order: PCS, WDS, PDS, the ODS segments of each object, END. The PCS has the screen size of the first shown image, the frame rate code `0x10` and no cropping. The composition number counts from 0 per `Write` call, and wraps after `0xFFFF`. `Cue.Forced` sets the object flag `0x40`.
- **Windows**: one window per image. Two images that overlap on the screen share one window around both.
- **Clear display set**: state `0x00`, no objects, the windows of the images shown before. It has no PDS and no ODS.
- **Palette**: all shown images share one palette. When every image is a palette PNG and the images together hold at most 256 colours, the palette keeps those colours in order of first use, with the colours of alpha 0 first. Otherwise the writer decodes every image to RGBA and reduces the pixels of all shown images together to 256 colours. Above 256 colours, a median cut gives 255 opaque entries and entry 0 transparent. An indexed PNG and an RGBA PNG with the same pixels give the same bytes.
- **Colour codes**: each colour becomes a limited range Y, Cr, Cb code with the matrix of the screen height of the first shown image, BT.601 up to 576 lines and BT.709 above. The writer searches the codes within 2 steps of the rounded code for the one that reads back as the same RGB. Some RGB values have no such code, so a colour can read back with one channel 1 off. The alpha is kept.
- **Objects**: the data is the width, the height and the run-length data of each row, with the codes of the reader. The first ODS segment holds at most 65,528 bytes of data and the 3-byte total length. Each later one holds at most 65,531 bytes.
- **Times**: a PTS is the cue time in milliseconds times 90. It must be from 0 to 4294967295. A cue that ends before it starts ends at its start, as the reader does.
- **Errors**: x, y, width, height and the screen size must be from 0 to 65535. The PNG must have the size that the image data says. A PNG that does not decode is an `*UnwritableError` with the decode error as `Err`. The writer checks every cue before it writes a byte, so a failed write writes nothing.
- **Round trip**: reading the output gives the same times, positions, sizes, forced flags, alignments and pixels. Two overlapping cues read back as one cue with one image per display set. A file with one object per display set in this segment order reads and writes back byte for byte.

`pgs.WriteOptions` and `pgs.WriteReport` have no field. `LineEnding`, `BOM` and `StripTags` have no effect, and neither has `SkipImageCues`.

## SubViewer

SubViewer 1 and 2 are the `.sub` formats of old DivX releases and DVD rippers. Package `format/subviewer` reads and writes both versions. The format name is `subviewer`, the extension `.sub`. MicroDVD owns `.sub` in `FormatFromPath`, so `Load` finds SubViewer files by their content. No BOM by default.

SubViewer 2 has a header of `[TAG]value` lines and cue blocks with centisecond times:

```text
[INFORMATION]
[TITLE]Morning at the bakery
[END INFORMATION]
[SUBTITLE]
00:00:01.50,00:00:04.00
The bakery opens at six.[br]Come early.
```

SubViewer 1 has a header, then the line `******** START SCRIPT ********`, then time lines in whole seconds:

```text
[TITLE]
Morning at the bakery
[DELAY]
2
******** START SCRIPT ********
[00:00:01]
The bakery opens at six.|Come early.
[00:00:04]
```

### Read

Each line is trimmed first. A line that equals `******** START SCRIPT ********` makes the file SubViewer 1. The lines before it are the header and the lines after it the script. Every other file is SubViewer 2.

**SubViewer 1 header**

- **Tags**: each non-empty line must be `[TAG]value`. The tag is trimmed and upper-cased, the value trimmed. A line that is no tag is an error.
- **Value on the next line**: when the value is empty and the next line is not empty and does not start with `[`, the next line is the value. So `[TITLE]` and `Market` on the next line give the title `Market`.
- **TITLE and AUTHOR**: they set the metadata `title` and `author`. An empty value deletes the key.
- **DELAY**: the reader adds the delay in whole seconds to every time, as FFmpeg does. It reads the number at the start of the value as PHP `(int)` does: `2` from `2.5`, `12` from `12abc`, `1000` from `1e3`, `0` from `abc`, and `0` from a number beyond the float range such as `1e999`. A negative delay is allowed. The stored header value becomes `0`, because the times already hold the delay.
- **Other tags**: they go to `header` under their upper-case tag.

**SubViewer 1 script**

- **Start**: a time line such as `[00:00:01]` followed by a text line that is no time line starts a cue at that time. Only that one line is read. `|` splits it into lines.
- **End**: a time line followed by an empty line or by another time line ends the last open cue at that time. The end of the file does the same for a pending time.
- **No end line**: a cue ends at the start of the next cue in the file that does not start before it. The last such cue lasts `LastCueDuration`, and its end stops at 99999:59:59.999.
- **Order**: a cue that a time line ended must not end before it starts.
- **Example**: with `[DELAY]` 2, the script `[00:00:01]` `Sunny|cloudy` `[00:00:04]`, an empty line, `[00:00:06]` `Light wind` `[00:00:09]` `Snow tonight` gives `Sunny` and `cloudy` from 3 to 6 s, `Light wind` from 8 to 11 s, and `Snow tonight` from 11 s for `LastCueDuration`.
- **Time limit**: the time plus the delay must be below 100000 hours and above -100000 hours. The message quotes the time line and the delay, such as `The time "[00:00:12] with DELAY 359999990" is not below 100000 hours.` Without a delay it quotes the time line alone.

**SubViewer 2**

- **Timing line**: `hh:mm:ss.f,hh:mm:ss.f`. The hours have up to 5 digits after leading zeros. Minutes and seconds are 00 to 59. The fraction has 1 to 3 digits and is a decimal fraction, as FFmpeg reads it: `.5` is 0.5 s and `.05` is 0.05 s.
- **Text**: the lines after a timing line. Each splits at `[br]` in lower case, and each part is trimmed. A timing line without text gives no cue, as in FFmpeg.
- **Style line**: a line that starts with `[` and holds `[COLF]`, `[SIZE]`, `[FONT]` or `[STYLE]` is never text. The reader stores only the first style line before the first cue, as `style`.
- **Header**: the lines before the first cue are `[TAG]value`. `[INFORMATION]`, `[END INFORMATION]` and `[SUBTITLE]` are skipped. `[TITLE]` and `[AUTHOR]` set the metadata, as in SubViewer 1. Other tags go to `header`. `[DELAY]` is kept as text and not applied.
- **A line that is no tag**: before the first cue, it is an error.
- **Text that looks like a timing line**: a line of 2 fields split at a comma where a field is a time such as `0:0:1` must be a valid timing line. Else it is an error. A line such as `We meet at 10:30, at the gate.` is text, because no field is a whole time.
- **Block index**: the timing lines counted from 0.

**Both versions**

- **Format data**: `{version: 1, header: {TAG: value}}` or `{version: 2, header: {TAG: value}, style: "..."}`. A file without header tags stores `header` as an empty list. The tags `0`, `1` and so on in this order also store a list, because a PHP array with these keys is a list.
- **Order**: the cues sort by start.
- **Encoding**: invalid UTF-8 stays in the text. Lenient mode adds the decode warning of [encodings.md](encodings.md).
- **Errors**: strict mode returns a `*ParseError` with the line of the error. Lenient mode records a `Skipped` warning. The rules are in [lenient-parsing.md](lenient-parsing.md#subviewer).

### Write

`WriteOptions.SubViewer.Version` sets the version. The zero value writes the version that the reader stored in the file format data, else SubViewer 2. So a SubViewer 1 file reads and writes as SubViewer 1, and `V2` converts it. PHP always writes SubViewer 2 and ignores the stored version.

**Header tags**: `TITLE` and `AUTHOR` from the metadata, empty when missing. Then the stored `header` in stored order, or these tags when no header is stored. The tag names are the ones that Subtitle Edit writes. The values are empty except where shown.

| Version | Default tags, in order |
|:--- |:--- |
| SubViewer 1 | `SOURCE`, `PRG`, `FILEPATH`, `DELAY` = `0`, `CD TRACK` = `0`, `BEGIN` |
| SubViewer 2 | `SOURCE`, `PRG`, `FILEPATH`, `DELAY` = `0`, `CD TRACK` = `0`, `COMMENT` |

- **SubViewer 1 layout**: each tag as `[TAG]`, then its value on the next line when it has one. A `DELAY` tag writes `0`, because the reader added the delay to the times. Then `******** START SCRIPT ********`. Each cue with text writes its start time, the lines joined with `|`, its end time and an empty line. At the end come `[end]` and `******** END SCRIPT ********`.
- **SubViewer 2 layout**: `[INFORMATION]`, each tag as `[TAG]value`, `[END INFORMATION]`, `[SUBTITLE]` and the stored style line. Then the cue blocks, each a timing line and the lines joined with `[br]`. One empty line separates the blocks.
- **Times**: SubViewer 2 rounds to whole centiseconds and SubViewer 1 to whole seconds, half up on whole milliseconds. A time within half a unit below 100000 hours stops at 99999:59:59.99 or 99999:59:59. The hours have 2 digits, or 3 from 100 hours. A negative time is 0.
- **Text**: the visible text of each line, with tags removed and entities decoded. A cue without visible text is not written, because an empty line ends a cue.
- **Header lines**: a CR or LF in a value becomes a space. A stored tag that the reader would read as another tag is not written. That is a tag that is empty, has `]`, has white space at its ends or has a lower-case ASCII letter. SubViewer 2 also skips `INFORMATION`, `END INFORMATION`, `SUBTITLE`, `COLF`, `SIZE`, `FONT` and `STYLE`. A SubViewer 2 value that makes the line a style line or a timing line is written empty. A SubViewer 1 value that starts with `[` or equals the script line is written on the tag line.
- **Style line**: the stored style is written when it is still a style line after CR and LF become spaces and the ends are trimmed. A text that is no style line, and a style line that looks like a timing line such as `[COLF]a,1:2:3`, is not written.
- **Lost on write**: the markup, the speakers, the precision below a centisecond in SubViewer 2 and below a second in SubViewer 1, the delay of SubViewer 1, style lines after the first cue, and the white space at the ends of a header value. A `|` in SubViewer 1 text reads back as U+00A6. A cue line that would read as structure gains U+2060. [Cue text that reads as structure](reading-writing.md#cue-text-that-reads-as-structure) lists both rules.

### Options

`subviewer.ReadOptions` has no fields. The reader reads `Lenient`, `Encoding` and `LastCueDuration`, which SubViewer 1 uses for a last cue without an end line. The root `WriteOptions.SubViewer` holds a `subviewer.WriteOptions`. The writer reads `LineEnding` and `BOM`. The detection rule is in the doc comment of `subviewer.Detect` and in [detection.md](detection.md).

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `Version` | `Version` | `VersionDefault`: the stored version, else SubViewer 2 | `V1` writes SubViewer 1, `V2` writes SubViewer 2 |

- **Check**: `Validate` rejects a version above `V2`, for example `The SubViewer version must be 1 or 2, got 3.`, with an error that matches `ErrInvalidArgument`.

## VobSub

VobSub is the bitmap subtitle format of DVD rips. It has 2 files: a text index `movie.idx` and a binary MPEG-2 program stream `movie.sub`. Package `format/vobsub` reads it. The format name is `vobsub`, the extension `.idx`. The format has no writer and no detection rule, so `Load` finds it only by the `.idx` extension, or when `ReadOptions.Format` is `vobsub`.

A **subpicture unit** (SPU) is one DVD subtitle bitmap with its control commands. Example: `two-tracks-pal.idx` has the lines `size: 720x576`, a 16-colour `palette:` line, `id: en, index: 0` and `timestamp: 00:00:01:000, filepos: 000000000`. The reader seeks to byte 0 of `two-tracks-pal.sub` and joins the private stream 1 packets of the sub-stream `0x20` into one unit. It decodes the 2-bit interlaced bitmap into an image cue at 1 s. [ocr.md](ocr.md) turns such cues into text.

### Loading the pair

- **Pair rule**: `Load` and `LoadFS` read both files. An `.idx` path looks for the `.sub` file next to it, first `.sub`, then `.SUB`. Any other path looks for `.idx`, then `.IDX`. A path without an extension gets a dot first. The rule applies only when the format is VobSub, from `ReadOptions.Format` or from an `.idx` path. A MicroDVD `.sub` file never looks for an `.idx` file.
- **Missing file**: the error matches `ErrNotExist` of package `io/fs` and names both files.
- **Encoding**: the `.idx` file decodes as text with `ReadOptions.Encoding`. The `.sub` file stays binary, so `ReadInfo.SourceEncoding` is empty. `Load` and `LoadFS` replace `ReadOptions.VobSub.IDX` with the `.idx` file.
- **Parse**: `Parse` has no second file. It needs the `.idx` content in `ReadOptions.VobSub.IDX` and the `.sub` content as input, and `ReadOptions.Format` set to `vobsub`. Without `IDX`, the error is an `*InvalidArgumentError` with `Option` `ReadOptions.VobSub.IDX`.

### Read

- **Tracks**: each `id: en, index: 0` line starts a track. The sub-stream of a track is `0x20` plus its `index:` value, at most 31. The `delay:` lines add up within a track, and each `id:` line resets the sum.
- **Cue times**: the start is the time of the `timestamp:` line plus the start delay of the unit. The end is the time of the `timestamp:` line plus the stop delay of the unit. A unit without a stop command ends at the start of the next unit, or after `LastCueDuration`, whichever is first. A cue never ends before it starts, so its end becomes its start. The time of a `timestamp:` line with its `delay:` lines must be above -100000 hours and below 100000 hours. The PES time stamps are not read.
- **Colours**: the `palette:` colours and the contrast of the unit, or the 4 colours of an enabled `custom colors:` line. The image has the size of the display area of the unit, on the screen size of the `size:` line.
- **Position**: every cue keeps the alignment bottom center. `Cue.Forced` is true for a unit that starts with the forced start command.
- **No picture**: a unit with no display area adds no cue. It still ends the cue before it.
- **Animation**: colour or area changes in a control sequence after the start command are ignored, because one image cannot hold them.
- **Ignored `.idx` settings**: `org`, `scale`, `alpha`, `smooth`, `fadein/out`, `align`, `time offset`, `forced subs` and `langidx`.
- **Language**: the `language` metadata is the `id:` value of the track, also when it is empty.
- **Errors**: an error in the `.idx` content fails both modes with a `*ParseError` of line 0. An error in the `.sub` content or in one unit fails a strict read. A lenient read adds a `Skipped` warning with the position of the timestamp line in its track as `Block`, and goes on with the next one.

### Options

| Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |
| `IDX` | `string` | `""` | the `.idx` content as UTF-8. `Load` and `LoadFS` fill it |
| `Index` | `*int` | nil: no filter | the track whose `id:` line has this `index:` value. A pointer, because 0 is a valid index |
| `Language` | `string` | `""`: no filter | the track whose `id:` value equals this, without case |

- **Selection**: the first track that matches every set filter. With no filter, the first track. `Index` names the `index:` field of the `.idx` file. It is not `ReadOptions.Track`.
- **Check**: `Validate` rejects an `Index` below 0, for example `The track index must be 0 or more, got -1.`, and a `Language` of only white space, with an error that matches `ErrInvalidArgument`.
- **CLI**: the input must be the path of the `.idx` file or of the `.sub` file with `--from vobsub`. Standard input with `--from vobsub` is a usage error. A directory input reads the `.idx` file and skips a `.sub` file that has an `.idx` file of the same name. No output may overwrite either file of the pair.
