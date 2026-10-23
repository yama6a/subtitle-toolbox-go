# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/qttext/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `quicktime_export_styles.txt` | Written for this repository in the shape of a QuickTime text export, as Apple's QuickTime reference "Text Descriptors" shows it: one header line, `{timeScale:600}` with 3 fraction digits, a timestamp before each sample, empty samples for gaps, `{italic}`, `{bold}` and `{plain}` in the text | MIT |
| `subtitle_edit_shape.txt` | Written for this repository in the shape of the QuickTime text that Subtitle Edit writes and of the string `$qttxt` in https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/TxtQuickTimeTest.php: CR LF line ends, the 5 header lines, `{timeScale:30}`, a start and an end timestamp per cue and an empty line after each end | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
