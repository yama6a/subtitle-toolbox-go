# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/dcp/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `dcpomatic_interop_shape.xml` | Written for this repository in the shape of the Interop files that DCP-o-matic writes through libdcp: 2-space indent, `Font` attributes in alphabetical order, `Text` elements with `VAlign` and `VPosition` only, top line first, fade times as tick counts | MIT |
| `dcpomatic_smpte_shape.xml` | Written for this repository in the shape of the SMPTE ST 428-7 files that DCP-o-matic writes through libdcp: the 2010 namespace as the default namespace, `TimeCodeRate` 24, `StartTime` 00:00:00:00, `Valign` and `Vposition` | MIT |
| `monal_interop_shape.xml` | Written for this repository in the shape of the Monal 2008 Interop file in the test `testParsesDCSubtitles` of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TtmlTest.php: CR LF line ends, 3-space indent, comments before the root, a `Font` around the subtitles and a `Font` inside each `Text`. The lines of one subtitle come bottom line first | MIT |
| `subtitle_edit_smpte2014_shape.xml` | Written for this repository in the shape of the SMPTE ST 428-7 files that Subtitle Edit writes, and of the test `testConvertFromXml6b` of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TtmlTest.php: the `dcst:` prefix, the 2014 namespace, `TimeCodeRate` 25, a `Space` element and a top line | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.xml`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit, except for 2 changes. The writer adds `FadeUpTime` and `FadeDownTime` to a stored subtitle that has none, and the WebVTT writer writes `line:N%,end` for a cue with an offset | MIT |
