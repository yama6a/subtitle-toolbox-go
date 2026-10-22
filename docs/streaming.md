# Streaming

`stbox.Load` holds the whole file and every cue in memory. For an 80 MB SubRip file with 400,000 cues, that is the 80 MB of input plus 400,000 cue values. Package `stream` handles one cue block at a time instead. Memory stays constant, whatever the file size.

Only SubRip and WebVTT stream, because their cue blocks do not depend on each other. LRC end times and ASS headers need the whole file.

| Type | Constructor | Gives or takes |
|:--- |:--- |:--- |
| `SubRipReader` | `NewSubRipReader(r, ReadOptions)` | `Cues()`, `Warnings()` |
| `WebVTTReader` | `NewWebVTTReader(r, ReadOptions)` | `Cues()`, `Warnings()`, `Header()` |
| `SubRipWriter` | `NewSubRipWriter(w, common, srt.WriteOptions)` | `Write(cue)`, `Close()` |
| `WebVTTWriter` | `NewWebVTTWriter(w, common, webvtt.WriteOptions, header)` | `Write(cue)`, `DroppedColors()`, `Close()` |

## Shift a SubRip file into WebVTT

Read a SubRip input cue by cue, add 2 s to each cue, and write WebVTT. The program never holds more than one cue. For files, pass the `*os.File` of `show.srt` in place of the string reader. Pass the `*os.File` of `show.vtt` in place of `os.Stdout`.

<!-- example: ExampleSubRipReader -->
```go
input := "1\n00:00:01,000 --> 00:00:02,500\nThe ferry leaves.\n\n2\n00:00:04,000 --> 00:00:05,000\nIt is <i>late</i>.\n"

w, err := stream.NewWebVTTWriter(os.Stdout, model.CommonWriteOptions{BOM: model.BOMRemove}, webvtt.WriteOptions{}, nil)
if err != nil {
	fmt.Println(err)
	return
}
for c, err := range stream.NewSubRipReader(strings.NewReader(input), stream.ReadOptions{}).Cues() {
	if err != nil {
		fmt.Println(err)
		return
	}
	c.Start += 2 * time.Second
	c.End += 2 * time.Second
	if err := w.Write(c); err != nil {
		fmt.Println(err)
		return
	}
}
if err := w.Close(); err != nil {
	fmt.Println(err)
}
// Output:
// WEBVTT
//
// 1
// 00:00:03.000 --> 00:00:04.500
// The ferry leaves.
//
// 2
// 00:00:06.000 --> 00:00:07.000
// It is <i>late</i>.
```

- **Header copy**: to keep the header text, `STYLE` and `REGION` blocks of a WebVTT input, pass `rd.Header()` to `NewWebVTTWriter`. The header is complete once the reader yielded its first cue.
- **Caller owns the files**: the package takes an `io.Reader` and an `io.Writer`. The caller opens and closes the files. `Close` on a writer marks it closed and leaves the `io.Writer` open.

## UTF-16 and other encodings

The readers take UTF-8, with or without a BOM. They keep the bytes of other 8-bit encodings as they are. For UTF-16, wrap the input with `golang.org/x/text`:

<!-- example: ExampleWebVTTReader_utf16 -->
```go
utf16 := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
input, err := utf16.NewEncoder().String("WEBVTT\n\n00:01.000 --> 00:02.000\nThe ferry leaves.\n")
if err != nil {
	fmt.Println(err)
	return
}

r := transform.NewReader(strings.NewReader(input), utf16.NewDecoder())
for c, err := range stream.NewWebVTTReader(r, stream.ReadOptions{}).Cues() {
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c.Start, c.Text())
}
// Output:
// 1s The ferry leaves.
```

## Differences from the full reader

With the same options, a reader gives the cues and warnings of `srt.Read` or `webvtt.Read`, and a writer gives the bytes of `srt.Write` or `webvtt.Write`. These points differ:

| Topic | Stream | Full reader |
|:--- |:--- |:--- |
| cue order | file order | sorted by start |
| WebVTT `NOTE` blocks | skipped | kept as comments |
| WebVTT warnings | sorted by line, a warning without a line first | in read order |
| `Warnings()` | the warnings of the blocks read so far, also during the loop | all warnings, after the read |
| strict mode error | yields the cues before the broken block, then the error | returns the error and no cues |
| read error or NUL byte in the middle of a block | drops that block, yields the complete blocks before it, then the error | returns the error and no cues |
| NUL byte in strict mode | the error comes when the reader reaches the line | the error comes before any block |
| options | `Lenient` only | also `LastCueDuration` and the others |

- **Errors**: `Cues()` yields `(model.Cue{}, err)` once and stops. A read error of the `io.Reader` returns wrapped.
- **Single use**: `Cues()` reads the input once. A second call yields nothing. Breaking out of the loop stops the read.
- **Writer errors**: an error of the `io.Writer` returns wrapped, and the output is incomplete. A `Write` after `Close` returns an error that matches `ErrInvalidArgument`.

## Memory

A reader holds one block. The WebVTT reader holds up to 2 cues for the [rolling cue collapse](formats.md#webvtt). A WebVTT file without the `WEBVTT` line in lenient mode holds the lines before its first cue, until the reader knows how to repair it. A Ctrl-Z at the end of a SubRip or WebVTT file holds the blank lines after it. The WebVTT reader also holds each line of only spaces or tabs until a line with text follows.
