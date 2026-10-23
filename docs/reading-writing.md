# Reading and writing

`stbox.Load` reads a file, `stbox.Parse` reads an `io.Reader`, and `stbox.Write` writes one format. One `ReadOptions` and one `WriteOptions` value serve every format. Each reader and writer ignores the fields of other formats, so one value serves a batch of mixed files.

<!-- example: ExampleWrite_lineEnding -->
```go
srt := "1\n00:00:01,000 --> 00:00:02,000\nHello\n"

s, _, err := stbox.Parse(strings.NewReader(srt), stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
var out bytes.Buffer
err = stbox.Write(&out, s, stbox.WebVTT, stbox.WriteOptions{LineEnding: stbox.CRLF})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Printf("%q\n", out.String())
// Output:
// "\ufeffWEBVTT\r\n\r\n1\r\n00:00:01.000 --> 00:00:02.000\r\nHello\r\n"
```

## Read options

| Field | Zero value | Check of `Validate` |
|:--- |:--- |:--- |
| `Format` | detect the format | none. A format without a reader returns an error that matches `ErrUnsupportedFormat` at read time |
| `Encoding` | detect the encoding | a name of [encodings.md](encodings.md), compared without case |
| `Lenient` | strict mode: the first broken block returns the error. See [lenient-parsing.md](lenient-parsing.md) | |
| `LastCueDuration` | 5 s | 0 or more. 0 gives a cue that ends where it starts |

- **Pointer**: `LastCueDuration` is a pointer, because 0 s is legal and differs from the default. `new(2 * time.Second)` sets it in one expression.
- **Validate first**: `Load`, `Parse` and `Write` call `Validate` before they read or write. A failed check is an `*InvalidArgumentError` whose `Option` is the Go path of the field, for example `ReadOptions.LastCueDuration`.

## Read steps

1. Read the whole input into memory. For a SubRip or WebVTT file too large for that, see [streaming.md](streaming.md).
2. Decode the text to UTF-8, as [encodings.md](encodings.md) says. `ReadInfo.SourceEncoding` holds the encoding.
3. Detect the format, unless `Format` is set. [detection.md](detection.md) holds the rules, also for empty content, video files and the extension fallback of `Load`.
4. Run the reader of the format.
5. Return the subtitle and a `ReadInfo` with the format, the source encoding and the warnings. On error, the subtitle is nil and `ReadInfo` is the zero value.

## Text pre-pass

Every text reader runs these steps in this order, before its own parsing. [internal/parsing/prepare.go](../internal/parsing/prepare.go) holds them.

| Step | Both modes | Strict | Lenient |
|---:|:--- |:--- |:--- |
| 1 | check UTF-8, as [Invalid UTF-8](encodings.md#invalid-utf-8) says | | |
| 2 | remove every UTF-8 BOM at the start. Joined files can start with 2 or more | | no warning |
| 3 | remove the last run of Ctrl-Z (0x1A) when only space, tab, CR and LF follow it. DOS editors end files with it | | no warning |
| 4 | WebVTT reads each NUL as U+FFFD, as its spec says | | |
| 5 | | a NUL returns `Line N has a NUL character. Check the encoding of the file.` | step 6 removes the NUL |
| 6 | remove the bytes 0x00 to 0x08, 0x0B, 0x0C, 0x0E to 0x1F and 0x7F. U+200E, U+200F and other format characters stay | no warning | one warning with the line of the first byte |
| 7 | content of only space, tab, CR and LF gives an empty subtitle | | the warnings of steps 1 and 6 stay |

- **Line numbers**: LF, a run of CRs before LF (CR LF, CR CR LF) and a lone CR each end one line.
- **Format given**: with `Format` set, empty content gives an empty subtitle with no warning.

## Last cue default

A cue without an end ends at the first later start of another cue. The last such cue ends at its start plus `LastCueDuration`, rounded to whole milliseconds. "Later" means strictly later, so 2 cues with the same start do not end each other.

- **Time limit**: the end stops at 99999:59:59.999, the largest time below 100000 hours. So a last cue that starts less than `LastCueDuration` before the limit is shorter. `LastCueEnd` in [internal/parsing/parsing.go](../internal/parsing/parsing.go) holds the rule.
- **Writer output**: every writer output reads back with its reader, except plain text, which has no reader. A writer that rounds a time to a unit coarser than a millisecond stops at the largest value below the limit in that unit, so ASS writes `99999:59:59.99` and MPL2 writes `[3599999999]`. `Units` in [internal/timecode/timecode.go](../internal/timecode/timecode.go) holds the rule for ASS, LRC, MPL2, SubViewer and TMPlayer, and `Frames` in the same file holds it for MicroDVD, MPSub and QuickTime text. The CSV writer counts frames inside each second, so `frameCell` in [format/csv/time.go](../format/csv/time.go) holds its own step back to the last frame below the limit. The SubRip writer stops at 999:59:59,999, because its reader takes at most 3 hour digits. `maxTime` in [internal/subrip/write.go](../internal/subrip/write.go) holds that limit. The LRC writer stops at 999:59.99, because its reader takes at most 3 minute digits. `maxTime` in [format/lrc/write.go](../format/lrc/write.go) holds that limit.

| Reader | Cue without an end |
|:--- |:--- |
| LRC | the last time tag with text. An LRC cue ends at the next time tag in time, also one without text or with the same time |
| QuickTime text | the last sample, when it has text. A QuickTime text cue ends at the next sample, also one without text |
| TMPlayer | the last entry with text. A TMPlayer cue ends at the next entry in time, also one with the same time |
| SubViewer 1 | the last cue of the script. A SubViewer 1 cue without an end line ends at the next cue in the file that does not start before it, also one with the same start. SubViewer 2 has an end in every timing line |
| SAMI | the last cue of the class. A SAMI cue ends at the next `SYNC` block of its class, also one with the same start |
| CSV and TSV | a row with an empty end cell and no duration cell |

Each reader that uses the default adds its row here.

## Write options

| Field | Zero value | Check of `Validate` |
|:--- |:--- |:--- |
| `LineEnding` | `LF` | `LF` or `CRLF` |
| `BOM` | `BOMDefault`: the default of the format | `BOMDefault`, `BOMAdd` or `BOMRemove` |
| `StripTags` | keep the markup that the format can hold | |

- **No lone CR**: readers accept a lone CR as a line end. Writers never write it.
- **`BOM` as an enum**: it has 3 states, and the zero value keeps the default of the format.

## Output rules

A writer builds the whole output with LF line endings in memory. [internal/writing/writing.go](../internal/writing/writing.go) then applies the line ending and the BOM rule and writes the result in one `Write` call. An error of the `io.Writer` returns wrapped and leaves incomplete output.

- **Line ending**: `CRLF` replaces each LF that has no CR before it with CR LF. A CR LF that a writer writes on purpose stays.
- **CSV and TSV line endings**: the CSV writer ends each row with the line ending itself and keeps a line break inside a cell as LF, as Excel writes it. Only the BOM rule runs after it.
- **CSV and TSV delimiter**: `stbox.Write` sets the delimiter when `WriteOptions.CSV.Delimiter` is empty. TSV output gets a tab. CSV output gets `,` when the stored delimiter is a tab. `csv.Write` called directly keeps the stored delimiter.
- **BOM**: the line ending comes first, then the BOM. `BOMAdd` adds one UTF-8 BOM unless the output starts with one. `BOMRemove` removes every leading BOM. `BOMDefault` adds the BOM for ASS, CSV, TSV, LRC, MPSub, SubRip and WebVTT, and leaves the output of the other formats as the writer wrote it.
- **StripTags**: the ASS, MicroDVD, SAMI, SubRip, TTML and WebVTT writers read it. The other writers hold no markup or always write plain text. Library JSON keeps the markup.
- **Negative times**: every writer except library JSON writes a negative time as 0. Library JSON writes it as it is.
- **No save function**: a caller opens the file and calls `Write`. The format comes from the caller, never from the file extension.

### Cue text that reads as structure

A writer changes cue text that its reader would read as structure, such as a line break, a new cue or a header line. A read of the output gives the same cues and the closest text that the format holds. Text without such a marker stays byte for byte.

| Text in a cue line | Writers | Written as |
|:--- |:--- |:--- |
| a line break | MicroDVD, MPL2, MPSub, SBV, SubRip, SubViewer, TMPlayer, WebVTT, CSV and TSV | a line break of the format, such as `\|` or `[br]` |
| a line break | SAMI | `<br>` |
| a line break | LRC | a space, because an LRC cue is one line |
| `\|`, such as `Rock \| Pop` | MicroDVD, MPL2, TMPlayer, SubViewer 1 | U+00A6 BROKEN BAR |
| `[br]` | SubViewer 2 | `[br]` with U+2060 WORD JOINER after the `[` |
| a line that the reader reads as a timing line, such as `00:00:03,500 --> 00:00:03,800` or `10:30 -> 11:00` | SubRip | U+2060 WORD JOINER, then the line |
| a line that the reader reads as a timing line, such as `0:00:03.500,0:00:03.800` or `0:00:03.5,x` | SBV | U+2060 WORD JOINER, then the line |
| a line that the reader reads as a time line, such as `[00:00:03]` | SubViewer 1 | U+2060 WORD JOINER, then the line |
| a line that the reader reads as a timing line, such as `x,00:00:03.80`, a style line such as `[COLF]&HFFFFFF`, or the script line `******** START SCRIPT ********` | SubViewer 2 | U+2060 WORD JOINER before each field in the shape of a time, else before the line |

- **Entities**: a writer without markup decodes the entities, so `&#10;` is a line break there too. SAMI and the other writers with markup write the entity as it is.
- **Why the word joiner**: these formats have no escape. U+2060 has no width, so the line shows the same, and no reader takes a line that starts with it as structure. A visible change such as `-->` to `->` would not help, because lenient SubRip reads `->` as an arrow too.
- **Why the broken bar**: `|` is a single character, so no zero-width character can split it. U+00A6 is the character that looks closest.
- **Markup look-alikes**: this rule does not change text that reads back as markup, such as `<i>` in SubRip, `{y:i}` in MicroDVD, `/` at the start of an MPL2 line and `{\i1}` in ASS.

## Format packages called directly

A format package such as `format/srt` runs the text pre-pass in its `Read` and the output rules in its `Write`. So on UTF-8 input, `srt.Read` and `srt.Write` give the same result as `stbox.Parse` and `stbox.Write`. Only the root package decodes other encodings and detects the format.
