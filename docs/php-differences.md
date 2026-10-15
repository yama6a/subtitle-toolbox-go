# Differences from PHP

Go matches the PHP library byte for byte, except for the behaviour below. Each row is a change on purpose.

| Behaviour | PHP 5.x | Go | Issue |
|:--- |:--- |:--- |:--- |
| `rgba()` alpha beyond the int range, such as `rgba(1,2,3,9223372036854775808)` | the int cast overflows and gives `#0102038000000000000000` ([subtitle-toolbox-php#1054](https://github.com/yama6a/subtitle-toolbox-php/issues/1054)) | not a color: an alpha above 255 gives false | #11 |
| word timestamp of 2562047 hours or more, such as `<2562047:00:00.000>` | reads it as 9223369200 s | not a time, because `time.Duration` cannot hold it: `WordTimestamp` gives false and `MapWordTimestamps` keeps the tag as written | #11 |
