# Upgrade to 3.0

Version 3.0 moves the module path. Run this command to update.

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
