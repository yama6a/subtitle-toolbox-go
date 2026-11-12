# Upgrade to 2.0

Version 2.0 has 4 breaks: the module path, the cue position, the frame rate type and the return value of every `Write`. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v2@latest
```

Library JSON stays version 1. The CLI keeps its commands, flags, exit codes and `--json` shapes. 2.0 removes one name, `webvtt.WriteWithReport`.

## The 1.x call and the 2.0 call

| 1.x | 2.0 |
|:--- |:--- |
| `import "github.com/yama6a/subtitle-toolbox-go"` | `import stbox "github.com/yama6a/subtitle-toolbox-go/v2"`. The package name stays `stbox`, so the `stbox` name in the import line keeps the code readable |
| `import "github.com/yama6a/subtitle-toolbox-go/format/microdvd"` | `import "github.com/yama6a/subtitle-toolbox-go/v2/format/microdvd"` |
| `go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@latest` | `go install github.com/yama6a/subtitle-toolbox-go/v2/cmd/stbox@latest` |
| `a := cue.Alignment` | `a := cue.Position.Alignment` |
| `cue.Alignment = stbox.TopCenter` | `cue.Position.Alignment = stbox.TopCenter` |
| `stbox.Cue{Alignment: stbox.TopLeft}` | `stbox.Cue{Position: stbox.Position{Alignment: stbox.TopLeft}}` |
| `microdvd.ReadOptions{FrameRate: 25}` | `microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}` |
| `microdvd.WriteOptions{FrameRate: 23.976}` | `fr, err := stbox.FrameRateFromFloat(23.976)`, then `microdvd.WriteOptions{FrameRate: fr}` |
| `csv.ReadOptions{FrameRate: 25}` and `csv.WriteOptions{FrameRate: 25}` | the same with `FrameRate: stbox.FrameRate{Num: 25, Den: 1}` |
| `mpsub.WriteOptions{FrameRate: 25}` | `mpsub.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}`. The rate must be whole |
| `err := s.ConvertFrameRate(25, 23.976)` | `err := s.ConvertFrameRate(stbox.FrameRate{Num: 25, Den: 1}, fr)`. The error also covers a rate that fails `Validate` |
| `err := stbox.Write(w, s, f, opts)` | `report, err := stbox.Write(w, s, f, opts)`. Use `_, err :=` to ignore the report |
| `err := srt.Write(w, s, common, opts)`, and the same call in every format package | `_, err := srt.Write(w, s, common, opts)`. Each format package declares its own `WriteReport`, empty where the format reports nothing |
| `report, err := webvtt.WriteWithReport(w, s, common, opts)` | `report, err := webvtt.Write(w, s, common, opts)` |

## Module path

The module path is `github.com/yama6a/subtitle-toolbox-go/v2`. Every import of the root package or of a sub-package gets `/v2` after the repository name. Links to the repository, `https://github.com/yama6a/subtitle-toolbox-go`, stay the same. Go treats `.../subtitle-toolbox-go` and `.../subtitle-toolbox-go/v2` as 2 modules, so a program can import both while it moves.

## Cue position

`Cue.Alignment` moves to `Cue.Position.Alignment`. The zero value is still bottom center. `Position` holds more than the alignment.

| Field | Meaning |
|:--- |:--- |
| `Alignment` | the 9-cell alignment of 1.x |
| `Row`, `Column` | the 1-based row of the first line and column of the first character on `Grid`. 0 means none. CEA-608 column 0 is `Column` 1 |
| `Grid` | the caption grid of `Row` and `Column`, such as `Grid{Rows: 15, Columns: 32}` for CEA-608. Zero when both are 0 |
| `Offset` | a point in percent of the video, `X` from the left edge and `Y` from the top edge. The `Alignment` says which point of the cue sits there. Nil means none. 0 is a legal value, so the field is a pointer |

Before:

```text
cue.Alignment = stbox.TopCenter
```

After:

```text
cue.Position.Alignment = stbox.BottomCenter
cue.Position.Offset = &stbox.Offset{X: 50, Y: 95}
```

`Validate` returns an error that matches `ErrInvalidArgument` unless `Row` and `Column` lie on `Grid` and `Offset` is from 0 to 100. `SetCue` and the library JSON reader call it. Library JSON stays version 1. A cue gets the optional key `position`, written only when `Row`, `Column` or `Offset` is set, and a 1.x reader ignores it. The 1.x formats fill and use only `Alignment`, except WebVTT and TTML, which read and write `Offset`. A WebVTT or TTML writer ignores an `Offset` outside the screen cell of the alignment. The cell is the third of the screen that holds the alignment. See [formats.md](formats.md) and [format-data.md](format-data.md#an-explicit-alignment-wins-over-a-stored-position).

### Band rule

A writer uses `Row` and `Column` only while the band rule holds. `Position.RowOn(g, lines)` applies it and moves the row to another grid. It splits the rows of the cue grid into 3 bands by the first row. The top band holds rows with `3*(Row-1) < Rows-1`, and the bottom band holds rows with `3*(Row-1) >= 2*(Rows-1)`. `RowOn` returns false when the row conflicts with the alignment. The writer then computes the row from the alignment, as in 1.x.

- A top alignment conflicts with a row in the bottom band.
- A bottom alignment conflicts with a row in the top band, but not with a row in the middle band. A CEA-608 reader gives every cue below row 4 a bottom alignment.
- A middle alignment conflicts with any row outside the middle band.

Example: an SCC cue on row 2 has `TopCenter`, so row 2 stays. A caller sets `BottomCenter` on it. Row 2 lies in the top band, so the EBU STL writer ignores it and writes the bottom row. SCC row 8 with `BottomCenter` keeps its row. A row that holds with the alignment moves to the grid of the writer. SCC rows 2, 15 and 8 with 1 line give rows 2, 23 and 12 on the 23-row teletext grid.

### Default-anchor rule

A WebVTT or TTML reader stores no offset for the default anchor of an alignment. The default anchor is 10, 50 or 90 percent on each axis, such as (50, 90) for `BottomCenter`. It says no more than the alignment. So a SubRip file that goes through TTML to WebVTT keeps its output.

## Frame rates

`model.FrameRate` replaces the `float64` frame rate. It is an exact fraction of `Num` frames in `Den` seconds with a `DropFrame` flag. The zero value means not set, as 0 did in 1.x. The root package re-exports it as `stbox.FrameRate`. These 1.x fields and parameters change:

- `microdvd.ReadOptions.FrameRate` and `microdvd.WriteOptions.FrameRate`
- `csv.ReadOptions.FrameRate` and `csv.WriteOptions.FrameRate`
- `mpsub.WriteOptions.FrameRate`, which must be whole
- `Subtitle.ConvertFrameRate(from, to)`, which takes 2 `FrameRate` values and keeps its error result

`FrameRateFromFloat(23.976)` gives 2997/125, not 24000/1001, so a 1.x frame rate keeps its result. It returns an error for NaN, infinity, 0, a negative value and a number of more than 18 digits, such as `1e-300`. 1.x accepted such a rate. The CLI options `--fps`, `--input-fps`, `--output-fps`, `--from-fps` and `--to-fps` keep their syntax and fail with exit code 2 for it. `DropFrame` is valid for 4 rates only: 30000/1001, 2997/100, 60000/1001 and 2997/50. `Validate` returns an error for any other rate with the flag. See [frame-rates.md](frame-rates.md).

## Write reports

`stbox.Write` returns `(WriteReport, error)`. The root `WriteReport` has one pointer field per format that reports changes. The field is non-nil after a write in that format. 2.0 has `WebVTT`, which holds one `DroppedColor` for each text color that WebVTT cannot hold. A later release adds fields for other formats, which breaks no caller.

Before:

```text
report, err := webvtt.WriteWithReport(w, s, common, webvtt.WriteOptions{})
```

After:

```text
report, err := stbox.Write(w, s, stbox.WebVTT, opts)
for _, d := range report.WebVTT.DroppedColors {
	fmt.Println(d.Message)
}
```

`webvtt.Write` returns the same report as `webvtt.WriteWithReport` did. The `Write` of every other format package returns its own `WriteReport`, which is an empty struct where the format reports nothing. The `stream` writers stay as they are.

## Conversions that change output

WebVTT and TTML readers fill `Cue.Position.Offset`, and their writers use it for a cue with no stored position of the output format. The default-anchor rule keeps a plain SubRip to TTML to WebVTT chain the same. The `.out` round trips of WebVTT and TTML stay the same. No conversion golden of the repository changes. To get the 1.x result, set `cue.Position.Offset = nil` before the write.

| Input | 1.x output | 2.0 output | Keep the 1.x output |
|:--- |:--- |:--- |:--- |
| TTML with a bottom center cue in a region such as `tts:origin="10% 10%" tts:extent="80% 85%" tts:displayAlign="after" tts:textAlign="center"`, converted to WebVTT | no cue settings | `line:95%,end`, plus `position:X%,center` when X is not 50 | `cue.Position.Offset = nil` |
| TTML with a bottom left or right cue in a region such as `tts:origin="10% 10%" tts:extent="80% 85%" tts:displayAlign="after" tts:textAlign="left"`, converted to WebVTT | `align:left` | `position:10%,line-left align:left`. The anchor X is not at the screen edge | `cue.Position.Offset = nil` |
| TTML with a top or middle cue, converted to WebVTT | `line:0` or `line:50%,center` | the same. The writer writes the height of `BottomCenter` only, and adds `position:X%,...` when X is not at the screen edge | `cue.Position.Offset = nil` |
| WebVTT with a percent `line` or `position` that names the same anchor as its alignment, converted to TTML, such as `line:95%,end` or `line:0 position:30% align:left` | the region of the alignment, such as `bottomCenter` or `topLeft` | a region with the offset, such as `bottomCenter-50-95` or `topLeft-30-0`. The first puts the cue at 95% from the top | `cue.Position.Offset = nil` |
| WebVTT or TTML with such an offset, converted to library JSON | no `position` key | the cue gets `"position":{"offset":{"x":50.0,"y":95.0}}` for `line:95%,end` | `cue.Position.Offset = nil` |
