# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/diff, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_original.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: BOM, CR LF, 13 cues, two-line cues. It is the old version. | MIT |
| `own_edited.srt` | Written for this repository. An edited copy of `own_original.srt` without BOM and with LF: cue 2 later, typo fixes in cues 4 and 9, cue 6 split in two, cue 8 removed, italics removed from cue 10, a line break moved in cue 11, and one added cue. | MIT |
| `own_report.txt` | Written for this repository. Expected `SubtitleDiff::toText()` output for `own_original.srt` against `own_edited.srt`. | MIT |
| `own_report.json`, `own_report_time_tolerance.json`, `own_report_ignore_formatting.json`, `own_report_text_only.json`, `own_report_ignore_whitespace.json` | Generated for this repository. Output of `subtitle-toolbox diff own_original.srt own_edited.srt --json` without an option and with one of `--time-tolerance 00:00:00.5`, `--ignore-formatting`, `--text-only` and `--ignore-whitespace`, checked by hand | MIT |
