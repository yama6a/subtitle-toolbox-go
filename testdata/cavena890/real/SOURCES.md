# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/cavena890/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `ferry_danish.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. The Danish language code with the Scandinavian letters | MIT |
| `harbour_english.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. Accent codes, italics, the 3 alignments and a cue of 3 lines in 2 blocks | MIT |
| `library_hebrew.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. The Hebrew table, with digits and punctuation in display order | MIT |
| `market_arabic.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. The Arabic table | MIT |
| `noodle_chinese.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. UTF-16BE text with italics | MIT |
| `train_russian.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. The Russian table | MIT |
| `weather_greek.890` | Written for this repository by `tests/files/cavena890/generate-fixtures.php`. The Greek table | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. The files `harbour_english.890.to.*` equal the PHP write goldens of the same name at the pinned commit | MIT |

No permissive real Cavena 890 files are known. The script writes each file in the shape that the Cavena 890 code of Subtitle Edit reads and writes, with the bytes of its text tables, see THIRD_PARTY_NOTICES.md.

The self-written files come from `tests/files/cavena890/generate-fixtures.php` in subtitle-toolbox-php. The script writes every byte itself and does not use the library.
