# Errors

A caller reacts to a failed read or write by its cause. Each cause is a sentinel error, and some causes also have an error type with fields. Match the sentinel with `errors.Is`, and read the fields with `errors.As`. Both work through any number of `%w` wraps.

Example: block 1 of a SubRip file has `00:00:05,000 => 00:00:07,000` on line 6, where the timing line must be. `stbox.Parse` returns a `*ParseError` with `Format` `srt` and `Line` 6.

<!-- example: ExampleParse_error -->
```go
content := "1\n00:00:01,000 --> 00:00:04,000\nHello\n\n2\n00:00:05,000 => 00:00:07,000\nWorld\n"

_, _, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{Format: stbox.SubRip})
var perr *stbox.ParseError
if errors.As(err, &perr) {
	fmt.Println(perr.Format, perr.Line)
}
fmt.Println(errors.Is(err, stbox.ErrParse))
fmt.Println(err)
// Output:
// srt 6
// true
// srt: line 6: Block #1 has no timing line on its second line. The line is "00:00:05,000 => 00:00:07,000".
```

## Causes

| Go | Returned for | PHP class and code |
|:--- |:--- |:--- |
| `ErrParse`, `*ParseError` | content that a reader cannot read. A byte that is not valid in the source encoding, in both modes. Invalid UTF-8 in strict mode, for the formats of [Invalid UTF-8](encodings.md#invalid-utf-8). A library JSON document with a wrong shape | `ParsingException` 100 |
| `ErrUnsupportedFormat` | a read of a format without a reader, such as `txt`. A write of a format without a writer. A `Format` value that no registry row lists, such as `Format("foo")`. A read of a Matroska, MP4 or MPEG-TS video file | `UnsupportedFormatException` 102 |
| `ErrUnknownFormat` | detection found no format, see [detection.md](detection.md). Also matches `ErrUnsupportedFormat` | `UnknownFormatException` 106, a subclass of 102 |
| `ErrInvalidArgument`, `*InvalidArgumentError` | an invalid option or argument, for example frame rate 0 or NaN. An unknown `ReadOptions.Encoding`. A retime or `Append` that moves a cue time to 100000 h or more, for example `Scale(1e300)`. MicroDVD output without a frame rate. CSV output with frame times and no frame rate. Format data that fails its type check in a setter | `InvalidArgumentException` 104 |
| `ErrCueNotFound` | `RemoveCue`, `SplitCue` and `JoinCues` with an index that has no cue | `CueNotFoundException` 105 |
| `ErrUnwritable`, `*UnwritableError` | the output format cannot hold the subtitle. Text that is not valid UTF-8 in a JSON format, TTML or SAMI output, which also matches `ErrInvalidUTF8`. A stored TTML head that is not valid XML. An `ass.WriteOptions.Style` that the `Format:` line of the stored styles lacks | `UnwritableContentException` 108 |
| `ErrInvalidUTF8` | text that an output format cannot hold because it is not valid UTF-8. It is the `Err` of the `*UnwritableError`. Read the input again with its source encoding | none, part of `UnwritableContentException` 108 |
| the `*fs.PathError` of package `os`, wrapped | a file that does not exist or cannot be read or written. Test it with `errors.Is(err, fs.ErrNotExist)` | `InvalidArgumentException` 104 |

- **No codes**: the library has no numeric error codes. `errors.Is` and `errors.As` replace them.
- **No catch-all**: no sentinel matches every library error. A caller matches the causes it handles, so errors of its own reader, such as a closed network stream, stay apart.
- **No panics**: bad input and bad options return one of the errors above. A panic is a library bug.
- **Which cause**: the sentinels and the types are API. A minor release may change which one a given cause returns, see [compatibility.md](compatibility.md#changes-a-minor-release-can-make).
- **Wraps**: `ErrUnsupportedFormat` and `ErrCueNotFound` arrive wrapped with details, for example `stbox: cue not found: index 4, the subtitle has 2 cues`.

## Error types

All 3 types live in package `model`. Package `stbox` aliases them under the same names.

| Type | Fields | `Error()` |
|:--- |:--- |:--- |
| `*ParseError` | `Format`, `Line`, `Msg`, `Option`, `Err` | `srt: line 6: Msg`. No `line N: ` when `Line` is 0. `stbox` in place of the format when `Format` is "" |
| `*UnwritableError` | `Format`, `CueIndex`, `Reason`, `Err` | `sami: Reason` |
| `*InvalidArgumentError` | `Option`, `Msg` | `stbox: invalid argument: WriteOptions.MicroDVD.FrameRate: Msg`. No option part when `Option` is "" |

- **Cause**: `Unwrap` of `*ParseError` and `*UnwritableError` returns `Err`, for example a `*json.SyntaxError`. `Error()` shows `Err` only when the message field is empty, because the message quotes the cause where it matters.
- **Message text**: `Msg` and `Reason` hold a full English sentence, as the PHP library writes it. `Error()` adds a lower-case prefix in Go style. People read these messages in the CLI, and the PHP sentences exist already.
- **No identifiers in messages**: a message never names a Go identifier or a CLI flag. A missing or bad option goes in `Option` as its Go path, such as `ReadOptions.MicroDVD.FrameRate`. A sentinel in `Err` names an edit that the caller must make first.
- **Cue index**: `UnwritableError.CueIndex` is the 0-based index of the first cue that the format cannot hold. It is -1 when no single cue causes the error.

## Line numbers

`ParseError.Line` is the 1-based input line. 0 means that the reader does not know it.

| Reader | Line |
|:--- |:--- |
| SubRip, WebVTT | the line of the error. A message about a missing cue number or timing line quotes the line, cut to 60 characters with `...` |
| TTML | the line of the element, the line of the `>` that ends its start tag. 0 for a time expression that is not valid or that alone reaches 100000 hours, for a file that is not well-formed XML and for a root that is not `<tt>` |
| library JSON | 0 |
| ASS and SSA | the line of the error: the event line, or the `Format:` line for a field name that is a number. A file without `[Events]` and `[Script Info]` gives 0 |

Each format adds its row here when its reader ships.

[Lenient mode](lenient-parsing.md) turns a `*ParseError` into a `skipped` warning. The warning takes `Msg` as its message and `Line` as its line. Some errors return in both modes, because they affect the whole file. An example is invalid JSON.

## CLI exit codes

The CLI maps an error to an exit code by the phase in which it occurs, not by its type.

| Code | Meaning | When |
|---:|:--- |:--- |
| 0 | success | no input failed, and no difference was found |
| 1 | result | `diff` found a difference. Not an error |
| 2 | usage | a flag parse error, an unknown command, no input, every check before the first read, and every error before the input loop starts. Example: `Validate()` of the options that the CLI built returns an error that matches `ErrInvalidArgument` |
| 3 | file | any error while one input is read, edited or written, whatever its type. A side file that is missing or does not parse, such as the reference of `sync`. An output file that the CLI cannot create. Any other error outside the input loop |
| 130 | interrupt | Ctrl-C or SIGTERM stopped the run. The finished outputs stay |

- **Precedence**: 130 wins over 3, and 3 wins over 1. A run where one input failed and another gave a result exits 3.
- **Message**: for a failed input the CLI prints `LABEL: MESSAGE` on stderr. LABEL is the input path as given, or `stdin`.
- **Lenient hint**: after an input fails in strict mode with a `*ParseError`, the CLI prints `Pass --lenient to skip or repair broken cues.` on stderr. It skips the hint for a format that ignores lenient mode.
- **Advice**: the CLI maps the `Option` field of an error to its own flag text. Example: `WriteOptions.MicroDVD.FrameRate` becomes `Pass --fps or --output-fps.`. It maps a sentinel in `Err` the same way.
