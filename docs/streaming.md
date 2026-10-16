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

Read `show.srt` cue by cue, add 2 s to each cue, and write `show.vtt`. The program never holds more than one cue.

```go
func shift(in io.Reader, out io.Writer) error {
	w, err := stream.NewWebVTTWriter(out, model.CommonWriteOptions{}, webvtt.WriteOptions{}, nil)
	if err != nil {
		return err
	}
	for c, err := range stream.NewSubRipReader(in, stream.ReadOptions{}).Cues() {
		if err != nil {
			return err
		}
		c.Start += 2 * time.Second
		c.End += 2 * time.Second
		if err := w.Write(c); err != nil {
			return err
		}
	}
	return w.Close()
}
```

- **Header copy**: to keep the header text, `STYLE` and `REGION` blocks of a WebVTT input, pass `rd.Header()` to `NewWebVTTWriter`. The header is complete once the reader yielded its first cue.
- **Caller owns the files**: the package takes an `io.Reader` and an `io.Writer`. The caller opens and closes the files. `Close` on a writer marks it closed and leaves the `io.Writer` open.

## UTF-16 and other encodings

The readers take UTF-8, with or without a BOM. They keep the bytes of other 8-bit encodings as they are. For UTF-16, wrap the input with `golang.org/x/text`:

```go
r := transform.NewReader(f, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
rd := stream.NewWebVTTReader(r, stream.ReadOptions{})
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
