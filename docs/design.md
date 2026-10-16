# Design decisions

Each record gives a decision, the reason and the rejected alternative that an issue states, and the issue.

## Four major lines

1.x reads and writes text subtitles. 2.x adds broadcast delivery and checks. 3.x adds image subtitles, OCR and video files. 4.x adds transcripts, speech and language. Each line opens with one `major` release that lands the API break the line needs. Its module path ends in `/v2`, `/v3` or `/v4`.

- **Rule**: a change of documented behaviour waits for the next major release. [compatibility.md](compatibility.md) lists what a minor or patch release may change.
- **Source**: [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1), [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7).

## Packages and the root facade

Package `model` holds the types. Each format has one package under `format/`, such as `format/srt`. The root package `stbox` reads and writes every format, and it re-exports the model types as type aliases.

- **No `init()` registration**: no format package registers itself. A missing format is then a compile error, and not an "unknown format" at run time. The detection order is an ordered list, with no priority field.
- **Not chosen**: registration in `init()`, which runs in import path order.
- **Source**: [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1), [#8](https://github.com/yama6a/subtitle-toolbox-go/issues/8).

## Typed options

`ReadOptions` and `WriteOptions` have one field for each common setting and one field for each format that has options. The zero value of every field is its default. `WriteOptions.JSON` holds the settings of the library JSON writer, and `WriteOptions.BOM` applies to every format.

- **Zero value**: a bool is named so that false is the default. A setting with 3 states is an enum, such as `BOM`. A pointer appears only where 0 is legal and differs from the default, as in `LastCueDuration`. A struct literal then needs no constructor.
- **Defaults**: option defaults and conversion output stay the same for a whole major line. A caller cannot pin a default that is a zero value.
- **Format packages**: each one declares its own options, empty when it has none. A first option is then a new field, never a signature change.
- **Not chosen**: a pointer for every nullable field, and a generic `model.Optional[T]`. Also not chosen: a minor release that changes a default, as PHP does. Also not chosen: option types only for formats that have options.
- **Source**: [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7) D1 and D2, [#8](https://github.com/yama6a/subtitle-toolbox-go/issues/8) D3.

## Warnings in ReadInfo

A read returns its warnings in `ReadInfo.Warnings`. The `Subtitle` holds none.

- **Why**: PHP stores the warnings on the subtitle, so they travel with every copy and edit.
- **Line and block**: `ParseWarning.Line` 0 means no line, because lines start at 1. `Block` -1 (`NoBlock`) means no block. Plain ints print without a nil check.
- **Not chosen**: `Line, Block *int` with nil for none.
- **Source**: [#17](https://github.com/yama6a/subtitle-toolbox-go/issues/17), [#14](https://github.com/yama6a/subtitle-toolbox-go/issues/14).

## Errors

Each cause is a sentinel error such as `ErrParse`. Some causes also have a type with fields, such as `*ParseError`. The library has no numeric error codes. See [errors.md](errors.md).

- **Messages**: a message keeps the PHP sentence. `Error()` adds a lower-case prefix, as in `srt: line 6: Block #1 has no timing line on its second line. The line is "...".` People read these messages in the CLI, and the PHP sentences exist already.
- **No identifiers**: a message never names a PHP or Go identifier or a CLI flag. The `Option` field names the option.
- **No catch-all**: no sentinel matches every library error. A caller matches the causes it handles, so errors of its own reader, such as a closed network stream, stay apart.
- **Not chosen**: Go style messages everywhere, and an `ErrStbox` that every sentinel wraps and every error type matches.
- **Source**: [#13](https://github.com/yama6a/subtitle-toolbox-go/issues/13) D1 and D2.

## Times are whole milliseconds

Every time is a `time.Duration`. Each parser and setter rounds it to whole milliseconds, half away from zero. Example: 1.0005 s becomes 1.001 s.

- **PHP difference**: PHP shifts in binary seconds, so 1.001 + 0.0005 is 1.0014999999999998 there. [php-differences.md](php-differences.md) lists the cases.
- **Source**: [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1), [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7), [#10](https://github.com/yama6a/subtitle-toolbox-go/issues/10), [#34](https://github.com/yama6a/subtitle-toolbox-go/issues/34).

## No logging in the library

The library never writes to stdout, stderr or a logger. It returns warnings and reports. Only the `stbox` command prints, to stderr.

- **Exception**: the owner's Go repos log with zap. A logger in the API would put zap into the module graph of every importer.
- **Source**: [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1), [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7).

## Third-party modules

The library imports 3 modules, and the tests also use testify. The `go` line and the versions are in `go.mod`. Everything else is written in this repository.

| Module | Used for |
|:--- |:--- |
| `golang.org/x/text` | the code pages of [encodings.md](encodings.md), and case mapping |
| `github.com/rivo/uniseg` | grapheme clusters for character counts |
| `github.com/dlclark/regexp2/v2` | patterns with lookaround or back references, and patterns that users supply |

- **Why x/text and uniseg**: `golang.org/x/text` holds the code page tables of 37 encodings. `github.com/rivo/uniseg` holds the Unicode grapheme break tables. Both follow new Unicode versions. Writing and updating these tables in this repository would cost more than the import.
- **Source**: [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1), [#2](https://github.com/yama6a/subtitle-toolbox-go/issues/2), [#38](https://github.com/yama6a/subtitle-toolbox-go/issues/38), owner decision.

## regexp by default

Code uses `regexp`. It uses `regexp2` only where a pattern needs lookaround or back references, and for patterns that a user supplies. Go `regexp` has no lookaround. Every `regexp2` match has a timeout of 1 s.

- **Source**: [#2](https://github.com/yama6a/subtitle-toolbox-go/issues/2), [#18](https://github.com/yama6a/subtitle-toolbox-go/issues/18), [#38](https://github.com/yama6a/subtitle-toolbox-go/issues/38).

## PHP read goldens as the conformance suite

The repository copies the PHP input files and the PHP read goldens. Each golden holds the parse warnings and the [library JSON](json.md) of a parse result. Go must produce the same JSON for the same file.

- **Why**: the PHP goldens are the conformance oracle. A Go bug must never rewrite them.
- **Differences**: a deliberate difference is a row in [php-differences.md](php-differences.md).
- **Write output**: the write goldens are Go's own, and they do not have to match the PHP bytes.
- **Source**: [#9](https://github.com/yama6a/subtitle-toolbox-go/issues/9), [#1](https://github.com/yama6a/subtitle-toolbox-go/issues/1).

## Word times and speakers are tags

In 1.x a word time such as `<00:00:01.500>` and a speaker such as `<v Anna>` stay as tags in the cue text. See [markup.md](markup.md).

- **Source**: [#10](https://github.com/yama6a/subtitle-toolbox-go/issues/10).

## API naming rules

Every exported name follows the naming rules N1 to N21 of [CONTRIBUTING.md](../CONTRIBUTING.md#api-rules). A test checks part of them.

- **Source**: [#7](https://github.com/yama6a/subtitle-toolbox-go/issues/7).
