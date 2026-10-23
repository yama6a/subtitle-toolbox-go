# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/hearing-impaired, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_sdh.srt` | Written for this repository in the shape of SubRip files with hearing-impaired annotations: UTF-8 BOM, CR LF line endings, sound descriptions in `[ ]`, `( )` and `{ }`, upper and mixed case speaker labels, two-speaker dialogue dashes, music notes and `#` lyrics, italic tags and `&amp;`. | MIT |
| `own_sdh_removed.srt` | Written for this repository. Expected `SubRipFormatter` output with BOM and CR LF for `own_sdh.srt` after `HearingImpairedRemover::apply()`. | MIT |
| `own_sdh_removed_all_options.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_sdh.srt` after `HearingImpairedRemover::apply()` with mixed case labels, `{ }` brackets and lyrics on. | MIT |
| `own_sdh.vtt` | Written for this repository in the shape of WebVTT captions with hearing-impaired annotations: CR LF line endings, `Kind` and `Language` headers, `NOTE` comments, cue settings, `<v>`, `<c.yellow>` and `<i>` tags, dialogue dashes, music notes and an unescaped `&`. | MIT |
| `own_sdh_removed.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_sdh.vtt` after `HearingImpairedRemover::apply()`. | MIT |
| `own_sdh_removed_keep_square_brackets.srt`, `own_sdh_removed_keep_music_lines.srt`, `own_sdh_removed_any_case_labels.srt`, `own_sdh_removed_keep_parentheses.srt`, `own_sdh_removed_keep_speaker_labels.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_sdh.srt --to srt --sdh` with one of `--sdh-keep-square-brackets`, `--sdh-keep-music-lines`, `--sdh-any-case-labels`, `--sdh-keep-parentheses` and `--sdh-keep-speaker-labels`, checked by hand against `own_sdh_removed.srt` | MIT |
