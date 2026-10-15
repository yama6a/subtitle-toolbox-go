# Contributing

## Workflow

- Run `make ci` before you push.
- Each pull request carries one release label: `major`, `minor`, `patch` or `skip-release`. Before v1.0.0 every pull request carries `skip-release`. Only the 1.0.0 pull request (#43) carries `major`.
- The pull request title becomes the squash commit subject. Write it as a release note line: imperative, for users, one line. Example: "Add the SBV format, read and write".
- The pull request body ends with a line `Release note: ...` in the same style.

## Credit for ported work

- **Copied file**: add it to the section of its project in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md), or add the section. Add an Apache-2.0 `NOTICE` file too when the project has one.
- **Ported or adapted code**: add a credit line to the comment of the type, function or table, even when the Go code looks different. Pin the link to a commit. For an exported identifier, the doc sentence comes first, then an empty `//` line, then the credit.

  ```go
  // ReadOptions holds the settings of the Spruce STL reader.
  //
  // Adapted from mantas-done/subtitles, src/Code/Converters/StlConverter.php
  // (https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/src/Code/Converters/StlConverter.php), MIT.
  type ReadOptions struct {
  ```

  For an unexported identifier, the credit can stand alone. When a whole package ports one upstream file, the credit goes in the package doc comment.
- **Wording**: write "Adapted from" for the same logic written again. Write "Copied from" for verbatim code or data. Use exactly these two forms, so `grep -rn "Adapted from\|Copied from"` finds every credit.
- **Apache-2.0**: add a line `// Changed: ...` that states what changed.
- **Upstream test data**: a test case with data from another project names the project in its case name, for example `name: "mantas-done styles"`. A ported test case cites the upstream test method in a comment, in the same credit form.
- **Fixtures**: every file under a `testdata` folder has a row in a `SOURCES.md` of its folder or of a parent folder, up to the `testdata` folder. A row is `| File | Source | License |`. The first cell holds a file name, a glob or a comma list of both, relative to the folder of the `SOURCES.md`, each in backticks. `TestEveryFixtureHasASourcesRow` and `TestEverySourcesRowNamesAFile` in `repo_test.go` check this.
- **Fuzz corpus**: files under `testdata/fuzz/` are exempt. `go test -fuzz` writes them, and they are this repository's own data under MIT.
- **Re-hosted data**: a project that re-hosts the data of another project gets a credit for the original source too. Example: the IRT EBU STL files that sandflow/ttconv took from irt-open-source/scf.
- **Files from the PHP repository**: a `SOURCES.md` copied from yama6a/subtitle-toolbox-php keeps its rows unchanged. It gets one line under its heading: `Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/<folder>, MIT. In the rows below, "this repository" means subtitle-toolbox-php.` Rows for files that are not copied are removed.
- **Code from the PHP repository**: Go code ported from yama6a/subtitle-toolbox-php carries no credit line. Both repositories have the same owner and the same MIT notice. The third-party credits inside a ported PHP file move to the Go code.
- **Linked modules**: the binary `stbox` contains every module it imports. A pull request that adds such a module to `go.mod` adds a section `## <module path>` with the `LICENSE` text of the module at that version. `TestEveryLinkedModuleHasANotice` checks this.

## Licence rules

- Permissive sources may be adapted, always with credit.
- Copyleft, proprietary and unlicensed sources are behaviour references only. Read the behaviour or the spec, write your own code and tests, and copy nothing.
- A project not in the table: check its licence at the pinned commit before you use anything. An unknown or missing licence means ideas only.

| Project | Licence | Use allowed |
|:--- |:--- |:--- |
| mantas-done/subtitles | MIT | adapt with credit |
| captioning/captioning | MIT | adapt with credit |
| podlove/webvtt-parser | MIT | adapt with credit |
| hartman/vtt-vivid | Apache-2.0 OR MIT. Its test data from mozilla/vtt.js is Apache-2.0 | adapt with credit. The vtt.js data needs its Apache-2.0 NOTICE |
| delphiki/SubRip-File-Parser | BSD | adapt with credit |
| Saeven/subtitles, ismailceylan/php-subtitle-toolkit, medienbaecker/kirby-vtt, Thijs-Riezebeek/subtitles | MIT | adapt with credit |
| benlipp/srt-parser | DBAD, permissive with attribution | adapt with credit |
| johnnoel/php-ass | CC-BY-SA-3.0 | behaviour only |
| iceman1010/srt-translation-validator | proprietary, no redistribution | ideas only, nothing copied |
| subtitle.js, subsrt, media-captions of vidstack, osk/node-webvtt | MIT | adapt with credit |
| shaka-project/shaka-player | Apache-2.0 | adapt with credit, keep the licence text, state changes |
| pysubs2, srt by cdown | MIT | adapt with credit |
| pycaption by PBS | Apache-2.0 | adapt with credit, keep the NOTICE |
| sandflow/ttconv | BSD-2-Clause. The STL files in `src/test/resources/stl/irt/` are Apache-2.0 with a NOTICE, checked at `1fed6ab` | adapt with credit. The IRT files need their NOTICE |
| go-astisub, martinlindhe/subtitles, libgosubs | MIT | adapt with credit |
| androidx/media (Media3) | Apache-2.0 | adapt with credit, keep the NOTICE, state changes |
| JDaren/subtitleConverter | MIT per its README | adapt with credit |
| noophq/subtitle | LGPL-3.0 | behaviour only |
| Subtitle Edit (libse, seconv) | MIT at every commit whose `LICENSE` file is MIT. The switch to MIT is commit `771236c5` of 2026-02-28. Older commits are GPL-3.0 | adapt with credit. Check the `LICENSE` file at the pinned commit first |
| AlexPoint/SubtitlesParser | MIT | adapt with credit |
| kitsumed/SubtitlesParserV2 | LGPL-3.0 | behaviour only |
| Aegisub | BSD-3 or ISC per file header. Some files differ | adapt with credit only if the file header is BSD or ISC. Otherwise behaviour only |
| KDE Subtitle Composer | GPL-2.0-or-later | behaviour only |
| MKVToolNix | GPL-2.0 | behaviour only |
| FFmpeg | LGPL-2.1 or later, some parts GPL | behaviour only |
| libass | ISC | adapt with credit |
| CCExtractor | GPL-2.0 | behaviour only |
| subparse, Rust | MPL-2.0, copyleft per file | behaviour only |
| rsubs-lib | MIT | adapt with credit |
| xqq/libaribcaption | MIT. The source files carry an ISC-style notice. Checked at `37dde0f` | adapt with credit |
| aribb24 | LGPL-3.0 | behaviour only |
| aspasia | 0BSD | adapt, credit appreciated |
| chardetng, used by aspasia | Apache-2.0 OR MIT, Mozilla Foundation, checked at `4a53a99` | adapt with credit under MIT |
| alass | GPL-3.0 | behaviour only |
| webvtt-ruby, srt for Ruby, subtitle_it | MIT | adapt with credit |
| SubTypo, submerger, subtitle_app_utils, Kotlin | GPL-3.0 | behaviour only |
| SwiftSubtitles, SwiftWebVTT | MIT | adapt with credit |
| swift-subtitle-kit | MIT, Dionysis Karatzas 2026, checked at `728d62d` | adapt with credit. Its `sample.*` fixtures are copies of the subsrt files, so credit subsrt. `embedded-bom-srt-fixture.srt` is its own |
| LibreTranslate, server | AGPL-3.0, checked at `54a66e8` | behaviour only. A client for its HTTP API is our own code. Never copy server code |
| KBlixt/subcleaner | no licence found | ideas only |
| BYVoid/OpenCC | Apache-2.0, no NOTICE file at `8cf737a` | adapt with credit, keep the licence text, state changes |
| Liberation Fonts | SIL OFL 1.1 | redistribute with the licence text |
| Podcastindex-org/podcast-namespace examples | CC0-1.0 | free to use. Credit is a courtesy |
| W3C specs and examples | W3C Software and Document License | examples may be used with the W3C notice |
| ISO/IEC, 3GPP, SMPTE, EBU, CEA specs, Adobe docs | specifications | implement from the spec. No code exists to copy |
| yama6a/subtitle-toolbox-php, yama6a/php-glyph-ocr | MIT, same owner | port freely. Third-party credits in a ported file move with the code |
| The Go standard library | BSD-3-Clause | adapt with credit |
| `golang.org/x/text`, `github.com/rivo/uniseg`, `github.com/Eyevinn/mp4ff`, `github.com/asticode/go-astits`, `github.com/dlclark/regexp2` | the `LICENSE` file of each module at the version in `go.mod` | import only. The binary links them, so each has a section in `THIRD_PARTY_NOTICES.md` |
