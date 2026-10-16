# Backward compatibility

The module follows semantic versioning. The rules below hold for every 1.x release, from v1.0.0 on. A break waits for 2.0, which has the module path `github.com/yama6a/subtitle-toolbox-go/v2`. The 2.x, 3.x and 4.x lines follow the same rules under their own module path.

## Rules

| # | Part | Rule |
|---:|:--- |:--- |
| C1 | API surface | Every exported identifier outside `internal/` is API. The compiler enforces `internal/`, so the module has no "public but internal" names |
| C2 | Behaviour | Documented behaviour is API. A change waits for the next major release, except under C3 and C4 |
| C3 | Bytes, cues, detection | A patch release may change written bytes, parsed cues, parse warnings and the detection result of a file. It does so only for a bug fix or a new format. The release notes list each change. Golden files make each change visible in review |
| C4 | Strict mode | A patch release may reject bad input that strict mode read before. That counts as a bug fix |
| C5 | Defaults and output | Option defaults and conversion output stay the same for the whole major line. A minor release never changes a default. It never starts to use a model field for output that it already writes. New behaviour arrives behind a new option. Bug fixes still go out in patch releases under C3 and C4 |
| C6 | Enumerations | A minor release may add a constant to `Format`, `WarningAction` and every other enum. Give each `switch` over a library enum a `default` case |
| C7 | Struct fields | A minor release may add fields to options, reports, results, `ReadInfo` and format data types. This is safe for keyed composite literals only. Unkeyed literals of library structs are not covered. The `composites` check of `go vet` flags them |
| C8 | `==` on structs | `==` is covered only for named scalar types, such as `Format`, `Alignment` and the other enums. It is also covered for structs whose doc comment says "Values are comparable with ==". A new slice, map or func field may break `==` on any other struct |
| C9 | Interfaces | Only the interfaces that users implement are API: the OCR engine, the translation engine and the HTTP client. A major line never changes them. 1.x exports none of them. Each applies from the line that adds it |
| C10 | Errors | The sentinel errors and error types that `errors.Is` and `errors.As` match are API. A minor release may change which one a given cause returns. Message text is not API. The library has no numeric error codes |
| C11 | Library JSON and format data | Version 1 of the library JSON, its keys and its value types are API. The keys and value types of format data are API too. A minor release may add fields and new string values. Readers ignore unknown fields. A minor release may add a type check on a format data field |
| C12 | Go version | A patch or minor release may raise the `go` line of `go.mod`. Renovate does it in a `patch` PR. The release notes show it as the title of that PR |
| C13 | Rule sets | A validation preset never gets a new rule inside a major line. A new text-fix rule is off by default. A patch release may change a text-fix rule so that it stops changing text that it should keep |
| C14 | CLI | The commands, the flags, the meaning of each exit code and the `--json` shapes are API. A minor release may add keys to the `--json` objects, as C11 says for the library JSON. Readers ignore unknown keys. The CLI never overwrites a file. Text output and messages may change in a minor release |

## Not covered

- **Internal code**: everything under `internal/` and every unexported name.
- **Unkeyed literals**: struct literals without field names.
- **Results built by callers**: callers read reports, results and `ParseWarning` values. A caller can build them by hand. The library then makes no promise about the meaning of fields that it adds later.
- **Messages**: the text of errors, warnings and CLI output.
- **Test hooks**: environment variables that exist only for tests, for example `STBOX_GROWTH`.

## Major releases

A major release may change anything above. It moves the module path to `/vN`. Its upgrade guide `docs/upgrade-N.0.md` shows the old call and the new call for each break.
