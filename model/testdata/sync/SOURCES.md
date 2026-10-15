# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/sync, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_target_de_25fps.srt` | Written for this repository. A German version of `own_reference_en.srt` for a 25 fps release: times divided by 25/23.976 and 2.3 s later, boundaries moved by up to 40 ms, cues 12, 30 and 49 missing, two extra cues. No BOM, LF. | MIT |
| `own_target_de_23976fps.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_target_de_25fps.srt` after `convertFrameRate(25, 23.976)`. | MIT |
