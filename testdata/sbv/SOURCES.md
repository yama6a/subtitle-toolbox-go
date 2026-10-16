# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/sbv, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `valid.sbv` | Written for this repository. Valid cues with times from 0 to over 1 hour | MIT |
| `from_srt.sbv` | Written for this repository. Expected `SbvFormatter` output for a SubRip input | MIT |
| `special_characters.sbv` | Written for this repository. `<`, `&` and tag-like text that SBV keeps as text | MIT |
| `two_digit_hours.sbv` | Written for this repository. Times with a 2-digit hour field | MIT |
| `three_digit_hours.sbv` | Written for this repository. Times with a 3-digit hour field | MIT |
| `srt_timestamps.sbv` | Written for this repository. A SubRip time line in an SBV file | MIT |
| `exceeded_minutes.sbv` | Written for this repository. A minute field of 60 | MIT |
| `exceeded_seconds.sbv` | Written for this repository. A second field of 60 | MIT |
| `missing_milli_digits.sbv` | Written for this repository. A 1-digit millisecond field | MIT |
| `missing_text.sbv` | Written for this repository. A cue without text lines | MIT |
| `missing_timestamps.sbv` | Written for this repository. A cue without its time line | MIT |
