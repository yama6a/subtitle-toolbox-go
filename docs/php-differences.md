# Differences from PHP

Go matches the PHP library byte for byte, except for the behaviour below. Each row is a change on purpose.

| Behaviour | PHP 5.x | Go | Issue |
|:--- |:--- |:--- |:--- |
| `rgba()` alpha beyond the int range, such as `rgba(1,2,3,9223372036854775808)` | the int cast overflows and gives `#0102038000000000000000` ([subtitle-toolbox-php#1054](https://github.com/yama6a/subtitle-toolbox-php/issues/1054)) | not a color: an alpha above 255 gives false | #11 |
| word timestamp of 2562047 hours or more, such as `<2562047:00:00.000>` | reads it as 9223369200 s | not a time, because `time.Duration` cannot hold it: `WordTimestamp` gives false and `MapWordTimestamps` keeps the tag as written | #11 |
| `EmbedRightToLeft` of a line that ends with LF | the `$` of its pattern matches before the final LF, so the LF is lost | the LF stays inside the marks | #18 |
| a regional indicator followed by a combining mark or U+200D, such as U+1F1E9 U+0301 | the PCRE2 `\X` of PHP 8.5 counts 2 | counts 1, as rule GB9 of UAX #29 says | #18 |
