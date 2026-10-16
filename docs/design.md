# Design decisions

Each record gives the decision, the main alternative, the reason and the cost.

## Four major lines

1.x reads and writes text subtitles. 2.x adds broadcast delivery and checks. 3.x adds image subtitles, OCR and video files. 4.x adds transcripts, speech and language.

- **Rule**: each line opens with one `major` release that lands the API break the line needs. A later major has the module path `github.com/yama6a/subtitle-toolbox-go/v2`, `/v3` or `/v4`.
- **Why**: a break has one date and one upgrade guide. [compatibility.md](compatibility.md) lists what a minor or patch release may change.
- **Cost**: a break waits for the next line, even when a fix is small.

## Packages and the root facade

Package `model` holds the types. Each format has one package under `format/`, such as `format/srt`. The root package `stbox` reads and writes every format, and it re-exports the model types as type aliases.

- **Why**: most callers import only `stbox`. A program that needs one format imports its package alone and pulls no other format into the binary.
- **Registry**: the format table is written by hand, and no package registers itself in `init()`. Import order then never changes the result.
- **Not chosen**: one package for everything, which puts every format into every importer.
- **Cost**: each format issue edits the registry by hand, in two tables that a test keeps in step.

## Typed options

`ReadOptions` and `WriteOptions` have one field for each common setting and one field for each format that has options. Example: `WriteOptions.JSON` holds the settings of the library JSON writer, and `WriteOptions.BOM` applies to every format.

- **Zero value**: the zero value of every field is its default. A bool is named so that false is the default. A setting that may get a third value is an enum. A pointer appears only where 0 is legal and differs from the default, as in `LastCueDuration`.
- **Why**: a struct literal needs no constructor. A caller cannot pin a default that is a zero value, so defaults stay the same for a whole major line.
- **Format packages**: each one declares its own options, empty when it has none. A first option is then a new field and never a signature change.
- **Not chosen**: a pointer for every optional field, and a generic `Optional[T]`.
- **Cost**: the root options structs grow by one field for each format with options.

## Warnings in ReadInfo

A read returns its warnings in `ReadInfo.Warnings`. The `Subtitle` holds none.

- **Why**: a warning describes one read, not the subtitle. The PHP library stores warnings on the subtitle, so they travel with every copy and edit.
- **Cost**: a caller who passes a subtitle on and wants its warnings passes the `ReadInfo` too.
- **Line and block**: `ParseWarning.Line` 0 and `Block` -1 (`NoBlock`) mean none. Plain ints print without a nil check.

## Errors

Each cause is a sentinel error such as `ErrParse`. Some causes also have a type with fields, such as `*ParseError`. Callers match with `errors.Is` and `errors.As`. The library has no numeric error codes.

- **Messages**: a message keeps the PHP sentence, and `Error()` adds a lower-case prefix: `srt: line 6: Block #1 has no timing line`. A message never names a Go identifier or a flag. The `Option` field names the option.
- **No catch-all**: no sentinel matches every library error. A caller matches the causes it handles, so errors of its own reader stay apart.
- **Not chosen**: Go style messages everywhere, and an `ErrStbox` that every error wraps.
- **Cost**: a caller who wants "any library error" lists the sentinels.

See [errors.md](errors.md).

## Times are whole milliseconds

Every time is a `time.Duration`. Each parser and setter rounds it to whole milliseconds, half away from zero. Example: 1.0005 s becomes 1.001 s.

- **Why**: integer arithmetic gives the same result on every machine. PHP computes in binary seconds, so a shift of 0.5 ms rounds the wrong way for some inputs. [php-differences.md](php-differences.md) lists the cases.
- **Cost**: no sub-millisecond times. A time of 100000 hours or more is an error, and a `time.Duration` stops at 2562047 hours.

## Word times and speakers are tags

A word time such as `<00:00:01.500>` and a speaker such as `<v Anna>` stay as tags in the cue text. `Cue` has no field for them.

- **Why**: the formats that hold them (WebVTT, ASS, TTML, LRC) write them as tags in the text, and every text transform already keeps tags.
- **Cost**: a caller reads word times with the helpers of package `markup`. See [markup.md](markup.md).

## No logging in the library

The library never writes to stdout, stderr or a logger. It returns warnings and reports. Only the `stbox` command prints, to stderr.

- **Exception**: the owner's Go repos log with zap. A logger in the API would put zap into the module graph of every importer.
- **Cost**: a caller who wants log lines turns `ReadInfo.Warnings` into log lines itself.

## Third-party modules

The library imports 3 modules. Tests also use testify. The `go` line and the versions are in `go.mod`.

| Module | Used for | Why not written here |
|:--- |:--- |:--- |
| `golang.org/x/text` | the code pages of [encodings.md](encodings.md), Unicode case mapping | it holds the code page tables, WHATWG tables that match the browsers, and the case mapping |
| `github.com/rivo/uniseg` | grapheme clusters for character counts | the standard library has no UAX #29 |
| `github.com/dlclark/regexp2/v2` | patterns with lookaround or back references, and patterns that users supply | `regexp` has no lookaround and no back references |

- **Rule**: a pull request that adds a module to `go.mod` adds its licence text to `THIRD_PARTY_NOTICES.md`.
- **Cost**: each module is part of the compatibility surface of the binary.

## regexp by default

Code uses `regexp`. It uses `regexp2` only where a pattern needs lookaround or back references, and for patterns that a user supplies. Every `regexp2` match has a timeout of 1 s.

- **Why**: `regexp` runs in linear time. `regexp2` can run without limit on a bad pattern, so the timeout is a rule and not an option.
- **Cost**: a user pattern that needs more than 1 s for one match fails.

## PHP read goldens as the conformance suite

The repository copies the PHP input files and the PHP read goldens. Each golden is the [library JSON](library-json.md) of a parse result. Go must produce the same JSON for the same file.

- **Rule**: a tool never rewrites a PHP read golden. A deliberate difference is a hand edit with a note, and a row in [php-differences.md](php-differences.md).
- **Write output**: the write goldens are Go's own. Each is reviewed on its own. They do not have to match the PHP bytes.
- **Why**: 1.x is a port, so the PHP suite is the cheapest proof that a read is right.
- **Cost**: Go inherits a PHP fault until a row of [php-differences.md](php-differences.md) fixes it.

## API naming rules

Every exported name follows the rules of [CONTRIBUTING.md](../CONTRIBUTING.md#api-rules). A test checks part of them.

- **Examples**: a lookup that can miss returns `(T, bool)`. A method that changes a `*Subtitle` is a verb and returns no `*Subtitle`. A closed set of values is a named type with a zero value that is the default.
- **Why**: a name that follows one rule is guessable, and a rule is easy to check in review.
- **Cost**: the names differ from PHP in places, for example `CueIndexAt` and `FrameRate`.
