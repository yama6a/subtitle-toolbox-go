# CLAUDE.md - subtitle-toolbox-go

This file covers how the repo is built. The GitHub issues hold the behaviour of each feature.
The Go port follows the PHP project yama6a/subtitle-toolbox-php.

## Tech stack

- **Language**: Go 1.27, one module, root package `stbox`, binary `stbox`. `CGO_ENABLED=0` must build.
- **Linting**: golangci-lint v2 with the shared config of yama6a/gha. `.golangci.local.yaml` merges over it.
- **Allowed modules**: `golang.org/x/text`, `github.com/rivo/uniseg`, `github.com/Eyevinn/mp4ff`, `github.com/asticode/go-astits`, `github.com/dlclark/regexp2`.
- **Test module**: `github.com/stretchr/testify`.
- **Written here**: everything else. EBML is written by hand. Tesseract runs through `os/exec`. PNG uses `image/png`.

## Project structure

The package map of #8. A package appears when its issue lands.

```
*.go           Root package stbox: the facade (Load, Parse, Write, Detect), aliases, format registry
model/         Subtitle, Cue, format facts table, common options, shared errors
markup/        Inline tag model and its helpers
format/<name>/ One package per format: srt, webvtt, ass, ttml, sami, sbv, microdvd, mpl2,
               tmplayer, mpsub, subviewer, qttext, lrc, csv, stboxjson, plaintext
stats/ stream/ hearingimpaired/ fixing/ profanity/ refsync/ diff/ dual/
               The 1.x services. A service never imports the root package
internal/      Helpers: timecode, textenc, grapheme, xmlraw, blocks, subrip, vtt, cli
cmd/stbox/     The CLI main. Wiring, no business logic
testdata/      Fixtures and goldens, next to the package that uses them
```

## Commands

```bash
make ci      # tidy-check, generate-check, fmt-check, lint, vet, test, vuln
make fmt     # rewrite formatting in place
make test    # go test -race ./...
make build   # write bin/stbox, VERSION=1.2.3 stamps the version
make golden  # regenerate Go write goldens, arrives with #9
make cross   # build and vet for darwin and windows
make fuzz    # run each fuzz target for 30 s, FUZZTIME=10s shortens it
make bench   # run the benchmarks, BENCHTIME=1x runs each once
make image   # build the container image, the Dockerfile arrives with #4
make dist    # write the 6 release archives and the checksum file to an empty dist/
make smoke   # run the smoke test on this machine's archive in dist/
```

**Always run `make ci` after each work step and fix everything it reports before moving on.**

## Non-negotiables

- **No logging in the library.** It returns warnings and reports. Only `cmd/stbox` prints, to stderr, as plain messages. No zap.
- **Durations.** Every time is a `time.Duration`. Every parser and setter rounds it to whole milliseconds.
- **Cue order.** A `Subtitle` always holds its cues in start order.
- **PHP read goldens.** A tool never rewrites them. A deliberate difference is a hand edit with a "Changed:" note in `SOURCES.md` (#9).
- **Test text.** Right-to-left and CJK text lives in testdata files, never in Go string literals. `bidichk` and `gosmopolitan` fail on it.
- **Regular expressions.** Use `regexp`. Use `github.com/dlclark/regexp2` only where a pattern needs lookaround or back references, and for user-supplied patterns. Always set `regexp2.Regexp.MatchTimeout`, because `regexp` has no timeout and regexp2 matches can run without limit.
- **Files.** The CLI never overwrites a file.
- **Credit.** Ported or copied code carries its credit line, see CONTRIBUTING.md.

## Code style

- Comments default to zero. Write one only when the reason is not in the code.
- Each exported identifier has a doc comment of one sentence that starts with its name.
- Errors wrap with `%w`. Sentinels are `ErrX` in `errors.go`. Error types are `XError` with an `Unwrap` method.
- Interfaces get compile-time checks: `var _ Reader = &reader{}`.
- Every `//nolint` names the linter and gives a reason.
- `exhaustive` switches list every case.
- A lookup table at package level needs `//nolint:gochecknoglobals` with a reason.

## Tests

- testify: `require` for errors and preconditions, `assert` for values.
- `t.Parallel()` in every test and subtest.
- Table-driven.
- Golden files live in testdata. A test rewrites them with the `-update` flag.
- Fixtures come from real or upstream data, never from invented data.

## Don'ts

- Don't add a module outside the allowed list.
- Don't add a `.golangci.yaml`. go-ci fails a repo that has one.
- Don't use cgo.
- Don't add a package-level logger.
- Don't call `panic` for bad input or bad options.
