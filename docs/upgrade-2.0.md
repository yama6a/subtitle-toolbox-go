# Upgrade to 2.0

Version 2.0 moves the module path to `/v2`. Run this command to update.

```bash
go get github.com/yama6a/subtitle-toolbox-go/v2@latest
```

| 1.x | 2.0 |
|:--- |:--- |
| `import "github.com/yama6a/subtitle-toolbox-go"` | `import "github.com/yama6a/subtitle-toolbox-go/v2"` |
| `go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@latest` | `go install github.com/yama6a/subtitle-toolbox-go/v2/cmd/stbox@latest` |
| `err := stbox.Write(w, s, f, opts)` | `report, err := stbox.Write(w, s, f, opts)`. `report.WebVTT` is non-nil after a WebVTT write and holds the dropped colors. Use `_, err :=` to ignore the report |
| `err := srt.Write(w, s, common, opts)`, and the same call in every format package | `_, err := srt.Write(w, s, common, opts)`. Each format package declares its own `WriteReport`, empty where the format reports nothing |
| `report, err := webvtt.WriteWithReport(w, s, common, opts)` | `report, err := webvtt.Write(w, s, common, opts)` |
