# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/spruce-stl/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `bakery.stl` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/stl.stl: LF line ends, no final line end, pipe characters as line breaks, an hour of 01 in the second cue | MIT |
| `bakery_with_header.stl` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/stl_with_comments.stl: a `//` comment, `$` formatting lines and a blank line before the cues | MIT |
| `dvdsp_full_header_ntsc.stl` | Written for this repository in the shape of the STL files of DVD Studio Pro 4, as its user manual describes them on pages 456 to 460: CR LF line ends, the full `$` command header with `//` section comments, `^B` and `^I` toggles, and `$Italic` and `$VertAlign` commands between the subtitles. Frame numbers up to 29 mark an NTSC file | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each `.out.stl` and `.to.*` file equals the PHP write golden of the same name at the pinned commit | MIT |
