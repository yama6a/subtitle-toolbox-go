# Reading and writing

`stbox.Load` reads a file, `stbox.Parse` reads an `io.Reader`, and `stbox.Write` writes one format. One `ReadOptions` and one `WriteOptions` value serve every format. Each reader and writer ignores the fields of other formats, so one value serves a batch of mixed files.

```go
s, info, err := stbox.Load("show.srt", stbox.ReadOptions{})
// info.Format is "srt", info.SourceEncoding is "UTF-8"

err = stbox.Write(f, s, stbox.WebVTT, stbox.WriteOptions{LineEnding: stbox.CRLF})
// WebVTT with CR LF line endings and the UTF-8 BOM that WebVTT writes by default
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

No reader of this release has cues without an end. Each reader that uses the default adds its row here.

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
- **BOM**: the line ending comes first, then the BOM. `BOMAdd` adds one UTF-8 BOM unless the output starts with one. `BOMRemove` removes every leading BOM. `BOMDefault` adds the BOM for ASS, CSV, TSV, LRC, MPSub, SubRip and WebVTT, and leaves the output of the other formats as the writer wrote it.
- **StripTags**: the ASS, MicroDVD, SAMI, SubRip, TTML and WebVTT writers read it. The other writers hold no markup or always write plain text. Library JSON keeps the markup.
- **Negative times**: every writer except library JSON writes a negative time as 0. Library JSON writes it as it is.
- **No save function**: a caller opens the file and calls `Write`. The format comes from the caller, never from the file extension.

## Format packages called directly

A format package such as `format/srt` runs the text pre-pass in its `Read` and the output rules in its `Write`. So on UTF-8 input, `srt.Read` and `srt.Write` give the same result as `stbox.Parse` and `stbox.Write`. Only the root package decodes other encodings and detects the format.
