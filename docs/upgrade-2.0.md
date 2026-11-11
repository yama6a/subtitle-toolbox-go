# Upgrade to 2.0

Version 2.0 moves the module path to `/v2`. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v2@latest
```

| 1.x | 2.0 |
|:--- |:--- |
| `import "github.com/yama6a/subtitle-toolbox-go"` | `import "github.com/yama6a/subtitle-toolbox-go/v2"` |
| `go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@latest` | `go install github.com/yama6a/subtitle-toolbox-go/v2/cmd/stbox@latest` |
| `a := cue.Alignment` | `a := cue.Position.Alignment` |
| `cue.Alignment = stbox.TopCenter` | `cue.Position.Alignment = stbox.TopCenter` |
| `stbox.Cue{Alignment: stbox.TopLeft}` | `stbox.Cue{Position: stbox.Position{Alignment: stbox.TopLeft}}` |
| `microdvd.ReadOptions{FrameRate: 25}` | `microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}` |
| `microdvd.WriteOptions{FrameRate: 23.976}` | `fr, err := stbox.FrameRateFromFloat(23.976)`, then `microdvd.WriteOptions{FrameRate: fr}` |
| `csv.ReadOptions{FrameRate: 25}` and `csv.WriteOptions{FrameRate: 25}` | the same with `FrameRate: stbox.FrameRate{Num: 25, Den: 1}` |
| `mpsub.WriteOptions{FrameRate: 25}` | `mpsub.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}`. The rate must be whole |
| `s.ConvertFrameRate(25, 23.976)` | `s.ConvertFrameRate(stbox.FrameRate{Num: 25, Den: 1}, fr)`. It returns an error for a rate that fails `Validate` |
| `err := stbox.Write(w, s, f, opts)` | `report, err := stbox.Write(w, s, f, opts)`. `report.WebVTT` is non-nil after a WebVTT write and holds the dropped colors. Use `_, err :=` to ignore the report |
| `err := srt.Write(w, s, common, opts)`, and the same call in every format package | `_, err := srt.Write(w, s, common, opts)`. Each format package declares its own `WriteReport`, empty where the format reports nothing |
| `report, err := webvtt.WriteWithReport(w, s, common, opts)` | `report, err := webvtt.Write(w, s, common, opts)` |

`FrameRateFromFloat(23.976)` gives 2997/125, so a 1.x frame rate keeps its result. It returns an error for NaN, infinity, 0, a negative value and a number of more than 18 digits, such as `1e-300`. 1.x accepted such a rate. The CLI options `--fps`, `--input-fps`, `--output-fps`, `--from-fps` and `--to-fps` fail with exit code 2 for it.

`Cue.Position` also holds a row and a column on a `Grid`, and an `Offset` in percent of the video. 2.0 readers and writers of the 1.x formats fill and use only `Alignment`, so no output changes. `SetCue` returns an error that matches `ErrInvalidArgument` when `Position.Validate` fails. Library JSON stays version 1. A cue gets the optional key `position`, and a 1.x reader ignores it.

`Position.RowOn(g, lines)` moves the row to another grid. It returns false when the row lies outside its grid or when the third of the grid that holds the row conflicts with the alignment. A top alignment conflicts with a bottom third row. A bottom alignment conflicts with a top third row. A middle alignment conflicts with any other row. A bottom alignment on a middle third row keeps its row, because a CEA-608 reader gives every cue below row 4 a bottom alignment. Example: SCC row 8 with `BottomCenter` keeps its row. SCC row 14 set to `TopCenter` loses it, and so does SCC row 2 set to `BottomCenter`. A writer then uses the row of the alignment, as in 1.x.
