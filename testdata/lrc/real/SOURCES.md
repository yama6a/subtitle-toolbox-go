# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/lrc/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| justan-1.lrc | Written for this repository in the shape of https://github.com/justan/lrc/blob/6ff0d026206a8cdb69bae537203075b3c452d816/example/lrcs/1.lrc | MIT |
| justan-4.lrc | Written for this repository in the shape of https://github.com/justan/lrc/blob/6ff0d026206a8cdb69bae537203075b3c452d816/example/lrcs/4.lrc | MIT |
| lrc-maker-nami.lrc | Written for this repository in the shape of https://github.com/lrc-maker/lrc-parser/blob/7ceaca1692cd2a03793b22133fed7a696f2a510a/test/sample-nami.lrc | MIT |
| mantas-done-lrc.lrc | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/lrc.lrc | MIT |
| subsrt-sample.lrc | Written for this repository in the shape of https://github.com/papnkukn/subsrt/blob/46deebd95703be9b6a76eb3c1aaf915713f30394/test/fixtures/sample.lrc | MIT |
| handwritten-core.lrc | Written for this repository from the core format and ID tag description at https://en.wikipedia.org/wiki/LRC_(file_format) | MIT |
| handwritten-enhanced.lrc | Written for this repository from the enhanced LRC (A2 extension) description at https://en.wikipedia.org/wiki/LRC_(file_format) | MIT |
| own-comment-before-earlier-line.lrc | Written for this repository. Lines out of time order, and a `[#:...]` comment before a line that starts earlier than the line before it | MIT |
| `own_whisper_cpp.lrc` | Written for this repository in the shape of whisper.cpp `-olrc` output: `[by:whisper.cpp]`, `[mm:ss.xx]` times and the text with its leading space. Shape from `output_lrc` in https://github.com/ggml-org/whisper.cpp/blob/60c0be6ac8fa71b1a2ae2dd938a31a34a508e774/examples/cli/cli.cpp | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
