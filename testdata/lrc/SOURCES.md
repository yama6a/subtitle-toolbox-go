# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/lrc, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| spaces_in_time_tags.lrc | Written for this repository. Spaces inside the time tag brackets, as in row 1 of issue #314 | MIT |
| hours_with_text_on_next_line.lrc | Written for this repository in the shape of the `testNotLrc` input in https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/formats/LrcTest.php. `hh:mm:ss` time tags with the text on the next line | MIT |
| one_digit_fraction.lrc | Written for this repository. A time tag with a 1-digit fraction, `[00:12.5]` | MIT |
| one_digit_minutes.lrc | Written for this repository. A time tag with a 1-digit minute field, `[0:01.00]` | MIT |
| `valid.lrc` | Written for this repository. Valid `[mm:ss.xx]` lines | MIT |
| `strip_xml.lrc` | Written for this repository. A line with HTML and XML tags to strip | MIT |
| `with_id_tags.lrc` | Written for this repository. ID tags such as `[ti:]` and `[by:]` between the lyric lines | MIT |
| `with_id_tags_formatted.lrc` | Written for this repository. Expected `LrcFormatter` output for `with_id_tags.lrc` | MIT |
| `exceeded_centi_accuracy.lrc` | Written for this repository. A 4-digit fraction field | MIT |
| `exceeded_minutes.lrc` | Written for this repository. A 4-digit minute field | MIT |
| `exceeded_seconds.lrc` | Written for this repository. A second field of 99 | MIT |
| `missing_text.lrc` | Written for this repository. A time tag without text | MIT |
| `missing_timestamps.lrc` | Written for this repository. A line without a time tag | MIT |
