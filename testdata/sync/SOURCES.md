# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/sync, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_reference_en.srt` | Written for this repository in the shape of a SubRip file from a subtitle editor: BOM, CR LF, 60 cues over 13 minutes, two-line cues. It is the reference that is in sync. | MIT |
| `own_target_de_25fps.srt` | Written for this repository. A German version of `own_reference_en.srt` for a 25 fps release: times divided by 25/23.976 and 2.3 s later, boundaries moved by up to 40 ms, cues 12, 30 and 49 missing, two extra cues. No BOM, LF. | MIT |
| `own_target_de_synced.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_target_de_25fps.srt` after the sync to `own_reference_en.srt`. | MIT |
| `own_reference_en_tv_break.srt` | Written for this repository. `own_reference_en.srt` as a TV recording with a 150 s ad break at 6:30: cues from 7:09 on are 150 s later. | MIT |
| `own_target_de_split_synced.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_target_de_25fps.srt` after the sync to `own_reference_en_tv_break.srt` with `maxSplits: 2`. | MIT |
| `own_target_de_23976fps.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_target_de_25fps.srt` after `convertFrameRate(25, 23.976)`. | MIT |
| `own_target_de_split_penalty_synced.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_target_de_25fps.srt` after the sync to `own_reference_en_tv_break.srt` with `maxSplits: 2` and `splitPenalty: 5`. The penalty keeps one part. | MIT |
| `cli_sync_de_to_srt.out.srt` | Written by `TestSyncGolden` of `internal/cli` with -update, reviewed by hand: the standard output of `stbox sync own_target_de_25fps.srt --reference own_reference_en.srt --to srt`. It equals `own_target_de_synced.srt`. | MIT |
