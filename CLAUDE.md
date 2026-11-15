# CLAUDE.md - subtitle-toolbox-go

This file covers how the repo is built. The GitHub issues hold the behaviour of each feature.

## Tech stack

- **Language**: Go 1.27, one module, root package `stbox`, binary `stbox`. `CGO_ENABLED=0` must build.
- **Linting**: golangci-lint v2 with the shared config of yama6a/gha. `.golangci.local.yaml` merges over it.
- **Allowed modules**: `golang.org/x/text`, `github.com/rivo/uniseg`, `github.com/Eyevinn/mp4ff`, `github.com/asticode/go-astits`, `github.com/dlclark/regexp2/v2`, `github.com/yama6a/go-glyph-ocr`.
- **Test module**: `github.com/stretchr/testify`.
- **Written here**: everything else. EBML is written by hand. Tesseract runs through `os/exec`. PNG uses `image/png`.

## Project structure

The package map.

```
*.go           Root package stbox: the facade (Load, Parse, Write, Detect), aliases, format registry
model/         Subtitle, Cue, format facts table, common options, shared errors
markup/        Inline tag model and its helpers
format/<name>/ One package per format: srt, webvtt, ass, ttml, sami, sbv, microdvd, mpl2,
               tmplayer, mpsub, subviewer, qttext, sprucestl, dcp, lrc, csv, itt, stboxjson, plaintext, pgs, chapters, transcript, youtube,
               cavena890, ebustl, mcc, pac, scc, vobsub
stats/ stream/ hearingimpaired/ speakers/ fixing/ profanity/ refsync/ diff/ dual/ karaoke/
               The 1.x services. A service never imports the root package
container/<name>/ The 3.x readers of video containers, layer 4: mkv, mp4, mpegts. They call the format readers. Their helpers live in container/<name>/internal: ebml (mkv), arib and teletext (mpegts)
ocr/           The 3.x service that reads the text of image cues with an OCR engine, and ocr/ocrtest, a fake engine for tests.
               The engines are ocr/glyph and ocr/tesseract, the chooser is ocr/auto. Layer 5, see CONTRIBUTING.md
translation/   The service that translates cue text with a machine translation engine. Its HTTP helper is translation/internal/httppost, the engines live in translation/<engine>. Layer 5, see CONTRIBUTING.md
validation/    The 2.x service that checks a subtitle against rules. It follows the same layer rule
timing/        The 2.x service that moves cue times to shot changes and frames. Same layer rule
imaging/       The 3.x public helpers that decode and build the PNG bitmaps of image cues. Same layer rule
hls/           The service that cuts a subtitle into HLS WebVTT segments and joins segments. Same layer rule
resegment/     The 4.x service that splits long cues and builds cues from word timings. Same layer rule
internal/      Helpers that more than one top-level folder uses: caption (with caption/cea608 and caption/cea708),
               bitmap (with bitmap/displayset, bitmap/dvbsub and bitmap/dvdsub), markupcore, speakerlabel, linewrap,
               timecode, textenc, extfallback, grapheme, ssatag, intkey, orderedjson, textblock, subrip, vtt,
               position, parsing, writing, cli, readingspeed, the test helpers testfiles, formattest and translationtest, and the
               test-only packages repocheck and conformance
format/internal/ Helpers that only format packages use: codepage, formatdata, paragraphs, ttmldoc, wordgroup, xmlraw
cmd/stbox/     The CLI main. Wiring, no business logic
testdata/      All fixtures and goldens, one folder per topic. Packages read
               them through internal/testfiles. Only fuzz corpora stay in <package>/testdata/fuzz/
```

## Commands

```bash
make ci      # tidy-check, generate-check, fmt-check, lint, vet, test, vuln
make fmt     # rewrite formatting in place
make test    # go test -race ./...
make build   # write bin/stbox, VERSION=1.2.3 stamps the version
make golden  # rewrite the Go read and write goldens, then review each diff by hand
make cross   # build and vet for darwin and windows
make fuzz    # run each fuzz target for 30 s, FUZZTIME=10s shortens it, FUZZSHARD=0 FUZZSHARDS=7 runs a seventh
make bench   # run the benchmarks, BENCHTIME=1x runs each once
make image   # build the container image from .build/Dockerfile
make dist    # write the 6 release archives and the checksum file to an empty dist/
make smoke   # run the smoke test on this machine's archive in dist/
```

**Always run `make ci` after each work step and fix everything it reports before moving on.**

## Non-negotiables

- **No logging in the library.** It returns warnings and reports. Only `cmd/stbox` prints, to stderr, as plain messages. No zap.
- **Durations.** Every time is a `time.Duration`. Every parser and setter rounds it to whole milliseconds.
- **Cue order.** A `Subtitle` always holds its cues in start order.
- **Read goldens.** They are Go goldens. `go test ./internal/conformance -run TestReadGoldens -update` rewrites them. Review each diff by hand.
- **Test text.** Right-to-left and CJK text lives in testdata files, never in Go string literals. `bidichk` and `gosmopolitan` fail on it.
- **Regular expressions.** Use `regexp`. Use `github.com/dlclark/regexp2/v2` only where a pattern needs lookaround or back references, and for user-supplied patterns. Always set `regexp2.Regexp.MatchTimeout`, because `regexp` has no timeout and regexp2 matches can run without limit.
- **Files.** The CLI never overwrites a file.
- **PHP bugs.** A bug in yama6a/subtitle-toolbox-php that this repo finds gets an issue in that repo.
- **Credit.** Ported or copied code carries its credit line, see CONTRIBUTING.md.

## Code style

- Comments default to zero. Write one only when the reason is not in the code.
- Each exported identifier has a doc comment of one sentence that starts with its name.
- Errors wrap with `%w`. Sentinels are `ErrX` in `errors.go`. Error types are `XError` with an `Unwrap` method when the type has an `Err` field.
- Interfaces get compile-time checks: `var _ Reader = &reader{}`.
- Every `//nolint` names the linter and gives a reason.
- `exhaustive` switches list every case.
- A lookup table at package level needs `//nolint:gochecknoglobals` with a reason.

## Tests

- testify: `require` for errors and preconditions, `assert` for values.
- `t.Parallel()` in every test and subtest.
- Table-driven.
- Black-box `package x_test`. Only `*_internal_test.go` files use `package x`, for unexported helpers.
- Fixtures come from real or upstream data, never from invented data.
- **Read goldens**: `<folder>/real/expected/<file>.json`, Go's own. `internal/conformance/readgoldens_test.go` reads each real file and compares canonical JSON. A format issue adds its folder row and its per-file read options there. `go test ./internal/conformance -run TestReadGoldens -update` rewrites a golden that differs. Review each diff by hand.
- **Write goldens**: Go's own, in `internal/conformance/writegoldens_test.go` or the test of a format package. A round trip without a golden must equal its input after `testfiles.Normalize`. The package declares `var update = flag.Bool("update", false, "rewrite the write golden files")`. `make golden` rewrites them.
- **Fuzz targets**: `FuzzRead` in `fuzz_test.go` of each format package, `FuzzDetect` in the root package.
  - Seeds: every file of the format's testdata folders, plus the short inputs of the table tests.
  - Each input reads once strict and once lenient, with zero options. A format that cannot read without a setting sets it, for example the MicroDVD frame rate 25.
  - Checks: no panic. An error matches a sentinel or type of `errors.go` with `errors.Is` or `errors.As`. The cues are in start order. Every time is whole milliseconds and below 100000 h. Writing the result in the same format returns nil or an error that matches `ErrUnwritable`.
  - Commit each crash input under `<package>/testdata/fuzz/FuzzRead/` with its fix.
  - `formattest.CheckRead` holds the read checks and `formattest.AddSeeds` adds the testdata seeds.
- **Examples**: `example_test.go` in the black-box test package, one per exported entry point, each with `// Output:`. Input is an inline raw string or a file under testdata, ASCII only. Output never depends on map order, the clock or the machine.
- **Benchmarks**: `b.Loop()`, `b.ReportAllocs()`, input built before the loop, sub-benchmark names such as `cues=20000`. `BenchmarkParse/<format>` in `bench_test.go` of the root package gets a row for each format with a reader.
- **Growth tests**: run the operation at n and 10n, best of 3 runs each, and fail above a ratio of 30. The top-level test name holds `Growth`. The test skips unless `STBOX_GROWTH=1` is set.

## Don'ts

- Don't add a module outside the allowed list.
- Don't add a `.golangci.yaml`. go-ci fails a repo that has one.
- Don't use cgo.
- Don't add a package-level logger.
- Don't call `panic` for bad input or bad options.
