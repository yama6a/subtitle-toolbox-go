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
