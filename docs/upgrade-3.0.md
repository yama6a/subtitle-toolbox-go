# Upgrade to 3.0

Version 3.0 moves the module path, adds image cues, takes an `io.ReaderAt` in `Parse` and returns a `Detection` from `Detect`. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v3@latest
```

## The 2.x call and the 3.0 call

| 2.x | 3.0 |
|:--- |:--- |
| `import stbox "github.com/yama6a/subtitle-toolbox-go/v2"` | `import stbox "github.com/yama6a/subtitle-toolbox-go/v3"` |
| `import "github.com/yama6a/subtitle-toolbox-go/v2/format/microdvd"` | `import "github.com/yama6a/subtitle-toolbox-go/v3/format/microdvd"` |
| `go install github.com/yama6a/subtitle-toolbox-go/v2/cmd/stbox@latest` | `go install github.com/yama6a/subtitle-toolbox-go/v3/cmd/stbox@latest` |
| the `image` format data of a cue, set with `SetFormatData("image", ...)` and `model.ImageKey` | `cue.Image`, a `*stbox.Image`. `SetFormatData("image", ...)` on a cue returns an `*InvalidArgumentError`, and `model.ImageKey` is gone |
| a write of a subtitle with image cues without text to a text format, which wrote cues without text | an `*UnwritableError`. Pass `stbox.WriteOptions{SkipImageCues: true}` to drop those cues, or run OCR first |
| `JoinCues`, `MergeSameTimeCues`, `MergeShortCues` and `RemoveDuplicateCues` joined cues with image data | they never join an image cue, and `JoinCues` returns an `*InvalidArgumentError` for a range that holds one |
| `stats.Of` and `validation.Check` counted cues with image data like other cues | they skip image cues without lines |
| `EditLines` and the text edits built on it removed a cue when all its lines went | a cue that holds an image stays, with no lines, because its removal would drop the image |
| `stbox.Parse(r, opts)` with an `io.Reader` | `b, err := io.ReadAll(r)`, then `stbox.Parse(bytes.NewReader(b), int64(len(b)), opts)` |
| `stbox.Parse(strings.NewReader(s), opts)` | `r := strings.NewReader(s)`, then `stbox.Parse(r, r.Size(), opts)`. A `*bytes.Reader` works the same way |
| `stbox.Parse(f, opts)` with an `*os.File` | `fi, err := f.Stat()`, then `stbox.Parse(f, fi.Size(), opts)`. Or `stbox.Load(path, opts)` |
| `f, ok := stbox.Detect(b)` | `d, ok := stbox.Detect(b)`, then `f := d.Format`. For a video file `d.Container` is set and `d.Format` is "" |

## Random-access input

`Parse` takes an `io.ReaderAt` and a size. A 4 GB Matroska file holds its subtitle track in blocks across the whole file, and the 2.x `Parse(io.Reader)` read all 4 GB into memory first. A container reader now jumps to the bytes it needs. Text and binary subtitle formats still read the whole input, because detection and code page scoring need all bytes.

`Load` keeps its signature. It opens the file and passes the handle with its size, so it reads only the head of a video file. A pipe, such as `/dev/stdin`, is read whole first, as in 2.x.

The size is the number of bytes to read. A negative size returns an `*InvalidArgumentError`. A size larger than the data never allocates that much, because `Parse` reads through a section reader.

## Containers and tracks

3.0 adds these names. They are additions, and no 2.x call changes because of them.

| Name | Meaning |
|:--- |:--- |
| `Container` | a video file format: `Matroska`, `MP4` or `MPEGTS`. `Container.Label()` gives the name for messages, such as `MKV or WebM` |
| `Track` | a subtitle track of a container: `Container`, `Number`, `CodecID`, `Format`, `Language`, `Name`, `Default`, `Forced` |
| `TrackError` | the error of a read without a track number, when the file has no subtitle track or 2 or more. It matches `ErrUnsupportedFormat` |
| `Tracks(r, size)` | the subtitle tracks of a video file, in file order |
| `ReadOptions.Track` | the number of the track to read. 0 reads the only subtitle track |
| `ReadInfo.Container` and `ReadInfo.Track` | the container and the track number of a container read. They are empty for a subtitle file |
| `ContainerHeadSize` | the 389 bytes at the start of a file that `Detect` needs to find a container |

v3.0.0 has no container reader yet. A read or `Tracks` call on a video file returns an error that matches `ErrUnsupportedFormat`. The 3.x minor releases add Matroska, MP4 and MPEG-TS.
