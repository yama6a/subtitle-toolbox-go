# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/scc/fit, MIT, except `own_long_cjk.txt`. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_ferry.srt` | Written for this repository: lines longer than 32 characters, the characters `Š`, `…` and `–` that CEA-608 lacks, and cues too close together for their caption data | MIT |
| `own_ferry.scc` | `own_ferry.srt` written by the PHP library with `SccWriteOptions(fit: true)`. The Go writer gives the same bytes | MIT |
| `own_long_cjk.txt` | Written for subtitle-toolbox-go: the 341 characters of the cue of `testFitAnswersForALongCjkCueUnderATenthOfASecond` in `SccFormatterTest.php`, which is a Chinese sentence about shopping repeated 12 times and cut at 341 characters | MIT |
