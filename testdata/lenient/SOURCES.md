# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/lenient, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Every file except `mantas_utf16.srt` and `mantas_multiple_new_lines.vtt` is written for this repository in the shape of broken subtitle downloads.

| File | Damage | License |
|:--- |:--- |:--- |
| `missing_cue_numbers.srt` | two cues without a cue number, UTF-8 BOM, CR LF | MIT |
| `bad_timestamp.srt` | a letter in an end time, and a `->` arrow, CR LF | MIT |
| `arrow_variants.srt` | no spaces, two spaces and a tab around the arrow, a `->` arrow and a `--->` arrow, LF | MIT |
| `full_width_delimiters.srt` | full-width `：` and `，` in cue 2, full-width `．` and `。` in cue 3, full-width punctuation in the cue text, LF | MIT |
| `no_cue_numbers.srt` | no cue numbers at all, so the file starts with a timing line, LF | MIT |
| `missing_empty_line.srt` | no empty line before cue 2 and before cue 3, cue 3 without a number, LF | MIT |
| `text_before_first_cue.srt` | a download site banner before cue 1, CR LF | MIT |
| `truncated_last_cue.srt` | the file ends inside the time line of cue 3, LF | MIT |
| `empty_line_in_cue.srt` | an empty line inside the text of cue 1, an empty line between the timing line and the text of cue 2, LF | MIT |
| `mantas_utf16.srt` | UTF-16 LE with a BOM, a start time with 1 seconds digit, CR LF. Copied from https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/utf16.srt | mantas-done/subtitles, MIT |
| `mixed_line_endings.srt` | CR LF, LF and CR CR LF in one file | MIT |
| `bad_timestamp.vtt` | a comma before the fraction of an end time, CR LF | MIT |
| `missing_empty_line.vtt` | no empty line after the header lines, nor between two cue pairs, LF | MIT |
| `text_before_first_cue.vtt` | a download site banner after the header, UTF-8 BOM, CR LF | MIT |
| `missing_signature.vtt` | no `WEBVTT` line, cues with identifiers, LF | MIT |
| `two_boms.vtt` | two UTF-8 BOMs before `WEBVTT`, CR LF | MIT |
| `text_before_signature.vtt` | converter text before the `WEBVTT` line, LF | MIT |
| `damaged_signature.vtt` | `WEBVTS` as the first line, a header line, cue settings, a NOTE block, LF | MIT |
| `truncated_last_cue.vtt` | the file ends inside the time line of the last cue, LF | MIT |
| `joined_files.vtt` | two files joined: the second part starts with a UTF-8 BOM, a `WEBVTT` line, a header line and a `STYLE` block, LF | MIT |
| `mantas_multiple_new_lines.vtt` | empty lines between the timing line and the text, and inside the text. Copied from https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/vtt_with_multiple_new_lines.vtt, LF | mantas-done/subtitles, MIT |
| `mixed_line_endings.vtt` | CR LF, LF and CR CR LF in one file | MIT |
| `bad_begin.ttml` | a letter in a `begin` time, a `p` without `end` or `dur`, LF | MIT |
| `mantas_multiple_divs.ttml` | a `p` with absolute times in a timed `div`, so it begins after the `div` ends. In the shape of https://github.com/mantas-done/subtitles/blob/959d1705b1ed12cbb9ddba517f8bcb814e1919a8/tests/files/ttml_with_multiple_divs.ttml, LF | MIT |
| `bare_seconds.ttml` | times as bare seconds with a fraction, such as `19.764`, and as `m:ss.fff`, in the shape of an Apple Music lyrics file with own text, LF | MIT |
| `credit_before_xml.ttml` | a credit line before the XML declaration, a letter in a `begin` time, CR LF | MIT |
| `html_entities.ttml` | the HTML entities `&eacute;`, `&nbsp;`, `&agrave;`, `&ndash;`, `&euro;`, `&laquo;` and `&raquo;`, which XML does not define, LF | MIT |
| `malformed.ttml` | a bare `&`, a `<br>` without `/`, the unknown entity `&cur;`, a `span` without its end tag, LF | MIT |
| `open_paragraphs.ttml` | three `p` elements without `end`, one of them with an empty `dur`, and a last `p` without `end`, LF | MIT |
| `missing_end.json` | a cue without `end`, a cue with a number as line, a comment after the first broken cue, LF | MIT |
| `start_after_end.json` | in the shape of `JsonFormatter` output, a cue that ends 2 s before it starts and a cue that ends 68 s before it starts, LF | MIT |
| `absurd_hours.vtt` | a cue with 20 hour digits between two good cues, LF | MIT |
| `absurd_hours.ttml` | a `p` with 20 hour digits and a `p` with an `end` of 20 digits in hours, LF | MIT |
| `absurd_seconds.json` | the library JSON. A cue with a start of `1e20` seconds between two good cues, LF | MIT |
| `absurd_word_timestamp.vtt` | a cue with a word timestamp of 20 hour digits between two good cues, LF | MIT |
| `broken_events.ass` | no `Format:` line in `[Events]`, an event with 4 fields, a letter in an end time, UTF-8 BOM, CR LF | MIT |
| `absurd_hours.ass` | a `Dialogue` line with 20 hour digits between two good lines, LF | MIT |
| `absurd_hours.csv` | a row with 20 hour digits between two good rows, LF | MIT |
| `loose_times.ass` | Start and End times with 4 fraction digits, without a fraction, with `,` before the fraction and with `:` before the fraction, LF | MIT |
| `numeric_field_name.ass` | the number `1` as a field name in the `Format:` line of `[Events]`, in the shape of an Aegisub file, UTF-8 BOM, CR LF | MIT |
| `sixty_minutes.ass` | an End time with 75 minutes and a Start time with 75 seconds, both with a 2-digit fraction, in the shape of an Aegisub file, LF | MIT |
| `bad_sync_start.smi` | a `SYNC` tag with an empty `Start` attribute, CR LF | MIT |
| `release_name.sub` | a release name before the `{1}{1}25` frame rate line, a cue with a letter as start frame, CR LF | MIT |
| `absurd_frames_microdvd.sub` | MicroDVD at 25 fps. A cue with frame numbers of 13 digits between two good cues, LF | MIT |
| `bad_timestamp.sbv` | a letter in an end time, LF | MIT |
| `loose_timing.sbv` | a dot between the start and end time, commas before the fractions, fractions with 1 and 2 digits, LF | MIT |
| `missing_empty_line.sbv` | no empty line between cue 1 and cue 2, LF | MIT |
| `text_before_first_cue.sbv` | a note line before cue 1, LF | MIT |
| `truncated_last_cue.sbv` | the file ends inside the time line of cue 3, LF | MIT |
| `empty_line_in_cue.sbv` | an empty line inside the text of cue 1, an empty line between the timing line and the text of cue 2, LF | MIT |
| `mixed_line_endings.sbv` | CR LF, LF and CR CR LF in one file | MIT |
| `absurd_hours.sbv` | a cue with 20 hour digits between two good cues, LF | MIT |
| `absurd_hours_tmplayer.txt` | TMPlayer. A line with 20 hour digits between good lines, LF | MIT |
| `bad_timing_line.mpsub` | MPSub. No `FORMAT=` line, a letter as duration, a last cue without text, LF | MIT |
| `broken_time_tag.lrc` | a letter in the seconds of a time tag, CR LF | MIT |
| `sixty_minutes.lrc` | a time tag with hours and 75 minutes, `[1:75:00.00]`, LF | MIT |
| `expected/*.json` | written for subtitle-toolbox-go. Each file holds the values of `damagedFiles()` in `tests/Parsers/LenientParsingTest.php` of this repository for the fixture of the same name, and the full message and line of the strict error that the PHP parser returns | MIT |
| `expected/release_name.sub.json` | written for subtitle-toolbox-go, as `expected/*.json`. Changed: the strict error is the Go message, which names no PHP option such as `MicroDvdReadOptions::$frameRate`, see [php-differences.md](../../docs/php-differences.md) | MIT |
| `expected/joined_files.vtt.json` | written for subtitle-toolbox-go. `damagedFiles()` has no row for this fixture, so the file holds the values of the 3 `joined_files` tests in `tests/Parsers/LenientWebVttTest.php` of this repository, and the strict error that the PHP parser returns | MIT |
| `expected/no_cue_numbers.srt.json`, `expected/mantas_utf16.srt.json` | written for subtitle-toolbox-go. `damagedFiles()` has no row for these fixtures, so the files hold the cue count of `FormatDetectorTest::subRipWithoutLeadingCueNumber` and the cue of `LooseTimestampsTest::testLenientModeReadsATimeWithOneSecondDigitInUtf16` of this repository, and the rest of the result and the strict error that the PHP parser returns | MIT |
| `expected/malformed.ttml.json`, `expected/bare_seconds.ttml.json` | written for subtitle-toolbox-go. `damagedFiles()` has no row for these fixtures, so the files hold the values of `testTtmlWithMalformedXmlIsRepaired` and `testLenientModeReadsBareSecondsWithAFractionOfAnAppleMusicFile` in `tests/Parsers/LenientTtmlTest.php` of this repository, and the rest of the result and the strict error that the PHP parser returns. The warning of `malformed.ttml` holds the start and the end of its message, because the middle is the message of the XML parser | MIT |
