# subtitle-toolbox-go

A Go library and command line tool that reads, edits and writes subtitle files.

## Install

```sh
go get github.com/yama6a/subtitle-toolbox-go
go install github.com/yama6a/subtitle-toolbox-go/cmd/stbox@latest
```

[docs/cli.md](docs/cli.md#install) shows how to install the command from a release archive or from the container image.

## Usage

A German SubRip file starts 2.5 s too late. This code reads it, moves every cue 2.5 s earlier and writes WebVTT. `stbox.Load` reads a file from a path in the same way.

<!-- example: ExampleParse_shiftToWebVTT -->
```go
german := "1\n00:00:03,500 --> 00:00:06,000\nGuten Morgen.\n\n2\n00:00:07,000 --> 00:00:09,000\nWie geht es Ihnen?\n"

s, info, err := stbox.Parse(strings.NewReader(german), stbox.ReadOptions{})
if err != nil {
	fmt.Println(err)
	return
}
fmt.Println(info.Format, s.Len()) // srt 2

if err := s.Shift(-2500 * time.Millisecond); err != nil {
	fmt.Println(err)
	return
}
if err := stbox.Write(os.Stdout, s, stbox.WebVTT, stbox.WriteOptions{BOM: stbox.BOMRemove}); err != nil {
	fmt.Println(err)
}
// Output:
// srt 2
// WEBVTT
//
// 1
// 00:00:01.000 --> 00:00:03.500
// Guten Morgen.
//
// 2
// 00:00:04.500 --> 00:00:06.500
// Wie geht es Ihnen?
```

WebVTT output starts with a UTF-8 BOM by default. `BOMRemove` leaves it out. The same change from the shell:

```sh
stbox convert film.de.srt --to vtt --shift -2.5 -o film.de.vtt
```

## Requirements

- Go of the `go` line in [go.mod](go.mod) or later.
- No cgo.
- The library imports `golang.org/x/text`, `github.com/rivo/uniseg` and `github.com/dlclark/regexp2/v2`.

## Formats

| Format | Go constant | Name | Extensions | Read | Write | Notes |
|:--- |:--- |:--- |:--- |:--- |:--- |:--- |
| [ASS and SSA](https://en.wikipedia.org/wiki/SubStation_Alpha) | `stbox.ASS` | `ass` | `.ass`, `.ssa` | yes | yes | |
| [JSON of this library](docs/json.md) | `stbox.JSON` | `json` | `.json` | yes | yes | |
| [SAMI](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/dnacc/understanding-sami-1.0) | `stbox.SAMI` | `sami` | `.smi`, `.sami` | yes | yes | one language class per read |
| [SubRip](https://en.wikipedia.org/wiki/SubRip) | `stbox.SubRip` | `srt` | `.srt` | yes | yes | |
| [TTML, IMSC, DFXP](https://www.w3.org/TR/ttml2/) | `stbox.TTML` | `ttml` | `.ttml`, `.dfxp`, `.xml` | yes | no | also iTT |
| [WebVTT](https://www.w3.org/TR/webvtt1/) | `stbox.WebVTT` | `vtt` | `.vtt` | yes | yes | also chapters |

The name is the value of `--from` and `--to`. [docs/formats.md](docs/formats.md) has one section for each format.

## Command line tool

```sh
stbox convert film.srt --to vtt -o film.vtt        # one file
stbox convert season1 --to vtt --output-dir out    # every subtitle file of a folder
stbox convert film.srt --to vtt --shift -2.5       # WebVTT with all cues 2.5 s earlier, on stdout
stbox info film.srt --json                         # format, encoding, statistics and warnings
stbox formats                                      # names and extensions for --from and --to
```

- Without `-o`, the output goes to standard output.
- Several inputs need `--output-dir`.
- No command overwrites a file.
- When detection fails, pass `--from`.

[docs/cli.md](docs/cli.md) lists the commands, the options and the exit codes.

## API

```text
func Load(path string, opts ReadOptions) (*Subtitle, ReadInfo, error)
func Parse(r io.Reader, opts ReadOptions) (*Subtitle, ReadInfo, error)
func Write(w io.Writer, s *Subtitle, f Format, opts WriteOptions) error
func Detect(content []byte) (Format, bool)
```

| Option | Field | Type | Default | Meaning |
|:--- |:--- |:--- |:--- |:--- |
| `ReadOptions` | `Format` | `Format` | detect the format | the format of the input |
| `ReadOptions` | `Encoding` | `string` | detect the encoding | the source encoding, such as `Windows-1252` |
| `ReadOptions` | `Lenient` | `bool` | false: stop at the first broken block | skip or repair broken blocks and report each in `ReadInfo.Warnings` |
| `ReadOptions` | `LastCueDuration` | `*time.Duration` | nil: 5 s | the length of a last cue that has no end |
| `WriteOptions` | `LineEnding` | `LineEnding` | `LF` | the line ending of the output, `LF` or `CRLF` |
| `WriteOptions` | `BOM` | `BOM` | `BOMDefault` | `BOMAdd` or `BOMRemove` sets the UTF-8 BOM. `BOMDefault` keeps the default of the format |
| `WriteOptions` | `StripTags` | `bool` | false: keep the markup | write the text without tags |

Each format has its own field in `ReadOptions` and `WriteOptions`. See [docs/formats.md](docs/formats.md).

## Design

- **Time**: every time is a `time.Duration`, rounded to whole milliseconds. PHP shifts in binary seconds, so it rounds some shifts differently, see [docs/php-differences.md](docs/php-differences.md).
- **Options**: the zero value of every option is its default, so `stbox.ReadOptions{}` reads with the defaults. A struct literal then needs no constructor.
- **Logging**: the library never logs. A read returns its warnings in `ReadInfo`. A logger in the API would put zap into the module graph of every importer.
- **Errors**: each cause is a sentinel error such as `ErrParse`, matched with `errors.Is`. No sentinel matches every error, so errors of the caller's own reader stay apart.
- **Packages**: no format package registers itself in `init()`. A missing format is then a compile error, and not an "unknown format" at run time.
- **Tests**: the PHP read goldens are the conformance oracle, and a Go bug never rewrites them. [docs/php-differences.md](docs/php-differences.md) lists each deliberate difference.

[docs/design.md](docs/design.md) links each decision to its issue.

## Compatibility

Every 1.x release keeps the API, the option defaults and the conversion output. See [docs/compatibility.md](docs/compatibility.md).

## Documentation

[docs/README.md](docs/README.md) lists every page. These pages come first:

- [docs/reading-writing.md](docs/reading-writing.md): `Load`, `Parse`, `Write` and the options
- [docs/formats.md](docs/formats.md): the formats and their options
- [docs/subtitle.md](docs/subtitle.md): `Subtitle` and `Cue`
- [docs/editing.md](docs/editing.md): retiming, timing fixes, cue structure and line wrapping
- [docs/errors.md](docs/errors.md): the errors and the exit codes
- [docs/cli.md](docs/cli.md): the `stbox` command

## Development

```sh
make ci      # tidy-check, generate-check, fmt-check, lint, vet, test, vuln
make test    # go test -race ./...
make fmt     # rewrite formatting in place
make golden  # rewrite the Go write goldens, then review each diff by hand
make fuzz    # run each fuzz target for 30 s
make bench   # run the benchmarks
make build   # write bin/stbox
```

## Releases

Every merge to `main` publishes a release. The label of the pull request sets the version bump.

| Label | On merge |
|:--- |:--- |
| `patch` | `v1.2.3` to `v1.2.4` |
| `minor` | `v1.2.3` to `v1.3.0` |
| `major` | `v1.2.3` to `v2.0.0`, and the module path moves to `.../v2` in the same PR |
| `skip-release` | nothing built or tagged |

- Renovate labels its PRs `patch`.
- The bump comes from every PR merged since the last release. The strongest label wins.
- A release builds archives for linux, darwin and windows on amd64 and arm64, and a checksum file.
- It builds the image `ghcr.io/yama6a/subtitle-toolbox-go` for linux/amd64 and linux/arm64, with the tags `1.2.3`, `1.2`, `1` and `latest`.
- It creates the git tag last, so every tag has its archives and its image.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the credit and licence rules.

## License

MIT, see [LICENSE](LICENSE). Material from third parties keeps its own licence, see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Credits

This module is a Go port of [yama6a/subtitle-toolbox-php](https://github.com/yama6a/subtitle-toolbox-php) 5.17.

The PHP libraries [mantas-done/subtitles](https://github.com/mantas-done/subtitles) and [captioning/captioning](https://github.com/captioning/captioning) shaped that library. [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) lists every file copied from other projects.
