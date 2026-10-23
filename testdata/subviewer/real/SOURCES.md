# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/subviewer/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `subviewer2_crlf.sub` | Written for this repository in the shape of the SubViewer 2 example in https://github.com/yama6a/subtitle-toolbox/issues/73, with CR LF line endings | MIT |
| `mantas_headers_shape.sub` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/sub_viewer_with_headers.sub | MIT |
| `mantas_plain_shape.sub` | Written for this repository in the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/sub_viewer.sub | MIT |
| `subtitle_edit_v1_shape.sub` | Written for this repository in the shape of the SubViewer 1.0 output of Subtitle Edit, https://github.com/SubtitleEdit/subtitleedit/blob/5b9ee8baf08c472c7a74fbc337446b656dedabb4/src/libse/SubtitleFormats/SubViewer10.cs | MIT |
| `subviewer1_delay.sub` | Written for this repository in the shape of the SubViewer 1 example in https://github.com/yama6a/subtitle-toolbox/issues/73, with a `[DELAY]` and cues without an end line | MIT |
| `own_max_delay.sub` | Written for this repository. SubViewer 1 with a `[DELAY]` of 359999990 seconds, so the last cue ends at 99999:59:59 | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
