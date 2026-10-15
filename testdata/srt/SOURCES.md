# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/srt, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `valid.srt` | Written for this repository. Valid cues with times from 0 to over 1 hour | MIT |
| `strip_xml.srt` | Written for this repository. Cues with HTML and XML tags to strip | MIT |
| `all_tags_stripped.srt` | Written for this repository. Expected `SubRipFormatter` output for `strip_xml.srt` after `stripTags()` | MIT |
| `exceeded_hours.srt` | Written for this repository. A 4-digit hour field | MIT |
| `exceeded_minutes.srt` | Written for this repository. A minute field of 99 | MIT |
| `exceeded_seconds.srt` | Written for this repository. A second field of 99 | MIT |
| `exceeded_milli_accuracy.srt` | Written for this repository. A 4-digit millisecond field | MIT |
| `missing_cue_number.srt` | Written for this repository. A cue without its number line | MIT |
| `missing_text.srt` | Written for this repository. Cues without text lines | MIT |
| `missing_timestamps.srt` | Written for this repository. A cue without its time line | MIT |
