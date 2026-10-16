# Backward compatibility

The module follows semantic versioning. A 1.x minor or patch release does not break code that uses the parts below as this page describes. A break waits for 2.0, which has the module path `github.com/yama6a/subtitle-toolbox-go/v2`.

## Covered

| Part | What stays stable in 1.x |
|:--- |:--- |
| Go API | every exported identifier outside `internal/`: packages, types, functions, methods, constants, variables and struct fields. The compiler keeps callers out of `internal/`, so the module has no exported names for internal use |
| Behaviour | the behaviour that these pages and the godoc describe, the default of each option, and the output of a conversion. Only a major release changes them, except for the changes that the sections below list |
| Errors | the sentinel errors that `errors.Is` matches and the error types that `errors.As` matches. [errors.md](errors.md) lists them. The library has no numeric error codes |
| CLI | the commands, the flags, the meaning of each exit code and the `--json` shapes of `stbox`. The CLI never overwrites a file. See [cli.md](cli.md) |
| Library JSON | version 1 of the library JSON, its keys and its value types. See [json.md](json.md) |
| Format data | the keys and the value types of the format data that `Subtitle.FormatData` and `Cue.FormatData` return. See [format-data.md](format-data.md) |
| Module path | `github.com/yama6a/subtitle-toolbox-go`, without a major suffix |

## Changes a minor or patch release can make

- **Bug fixes and new formats**: a bug fix or a new format can change the written bytes, the parsed cues, the parse warnings and the detection result of a file. The release notes list each change.
- **Strict mode**: strict mode is the default, without `Lenient: true` in `ReadOptions`. It can start to reject bad input that it read before. That is a bug fix, not a break. See [lenient-parsing.md](lenient-parsing.md).
- **Go version**: a release can raise the `go` line of `go.mod`. Renovate does it in a pull request labelled `patch`. The release notes show the title of that pull request.

## Changes a minor release can make

- **Enum constants**: an enum such as `Format`, `WarningAction` or `Alignment` can get a new constant. Give a `switch` on a library enum a `default` case.
- **String values in JSON**: a string field of the library JSON or of the `--json` output can get a new value. Examples are `format` and the `action` of a warning.
- **New fields**: options, reports, results, `ReadInfo` and format data types can get new fields. The library JSON and the `--json` objects can get new keys. Ignore fields and keys that you do not know.
- **New API**: new packages, functions, methods, types and constants.
- **Errors**: the sentinel or the error type that a given cause returns can change. Test errors with `errors.Is` and `errors.As`.
- **Messages**: the text of an error, of a parse warning and of the CLI text output can change. Parse `--json` output, not the text output.
- **Format data checks**: `SetFormatData` can check the type of a new field. Format data that passed before can then return an error that matches `ErrInvalidArgument`.
- **Option defaults**: a minor release never changes the default of an option. New behaviour arrives behind a new option, and the zero value of that option keeps the old behaviour. A minor release also never starts to use a model field for output that it already writes.

## Text fixes and presets

The hearing-impaired remover, the common error fixer and the profanity filter arrive in 1.x minor releases. From the release that adds each of them, these rules apply:

- **Existing rules**: a patch release can change a rule so that it stops changing text that it should keep.
- **New rules**: a new rule is off by default. Turn it on with its option.

Validation presets arrive in 2.x. From then on, a preset never gets a new rule inside its major line.

## Interfaces

1.x exports no interface that callers implement. Later lines add 3: the OCR engine in 3.x, and the translation engine and the HTTP client in 4.x. From the line that adds an interface, it stays the same for the whole line.

## `==` on structs

- **Covered**: `==` on named scalar types, such as `Format`, `Alignment` and the other enums. Also `==` on a struct whose doc comment says "Values are comparable with ==".
- **Not covered**: `==` on any other struct. A release can add a slice, map or func field to it, and `==` then fails to compile.

## Major releases

A major release can change anything above. It moves the module path to `/vN`, for example `github.com/yama6a/subtitle-toolbox-go/v2`. Its upgrade guide `docs/upgrade-N.0.md` lists each change with the old call and the new call.

## Not covered

- **Internal code**: every package under `internal/` and every unexported name.
- **Unkeyed literals**: struct literals without field names. A new field breaks them. The `composites` check of `go vet` reports them.
- **Results built by callers**: the library builds reports, results and `ParseWarning` values. Read their fields. A caller can build them by hand, but the library makes no promise about the meaning of fields that it adds later.
- **Messages**: the text of errors, warnings and CLI output.
- **Test hooks**: environment variables that exist only for the tests, such as `STBOX_GROWTH`.
- **The Go code of `cmd/stbox`**: the command line tool is API only as a binary. Run `stbox` instead of its Go code.
