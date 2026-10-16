# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/sbv/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `subsrt_shape.sbv` | Written for this repository in the shape of https://github.com/papnkukn/subsrt/blob/46deebd95703be9b6a76eb3c1aaf915713f30394/test/fixtures/sample.sbv | MIT |
| `youtube_studio_lf.sbv` | Written for this repository in the shape of a YouTube Studio SBV export with LF line endings and a final newline | MIT |
| `youtube_studio_crlf.sbv` | Written for this repository in the shape of a YouTube Studio SBV export with CR LF line endings and no final newline | MIT |
| `own_empty_cue.sbv` | Written for this repository. Cue 2 has a timing line and no text. | MIT |
| `own_angle_bracket_text_from_srt.sbv` | Generated for this repository. Expected `SbvFormatter` output for `srt/real/own_angle_bracket_text.srt`. | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.sbv` | Written by the Go write golden tests with -update, reviewed by hand. PHP has no golden for these files | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |
