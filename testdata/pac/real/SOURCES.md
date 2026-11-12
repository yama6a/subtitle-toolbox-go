# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/pac/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `bakery_latin.pac` | Written for this repository by `../generate-fixtures.php`. A story record, italics, the 3 alignments, 3 rows and Latin characters on moved bytes | MIT |
| `bakery_unicode.fpc` | Written for this repository by `../generate-fixtures.php`. PAC Unicode in the shape that Subtitle Edit writes, with Latin, Chinese and Greek text | MIT |
| `bazaar_turkish.pac` | Written for this repository by `../generate-fixtures.php`. The Turkish code page | MIT |
| `cafe_korean.pac` | Written for this repository by `../generate-fixtures.php`. Korean double-byte text, CP949 | MIT |
| `harbour_30fps.pac` | Written for this repository by `../generate-fixtures.php`. Frame numbers up to 29 | MIT |
| `library_hebrew.pac` | Written for this repository by `../generate-fixtures.php`. The Hebrew code page, a Latin word and a digit in display order | MIT |
| `market_arabic.pac` | Written for this repository by `../generate-fixtures.php`. The Arabic code page, digits in display order | MIT |
| `noodle_chinese_simplified.pac` | Written for this repository by `../generate-fixtures.php`. Simplified Chinese double-byte text, GBK, and a Latin line | MIT |
| `noodle_chinese_traditional.pac` | Written for this repository by `../generate-fixtures.php`. Traditional Chinese double-byte text, Big5 | MIT |
| `station_japanese.pac` | Written for this repository by `../generate-fixtures.php`. Japanese double-byte text, Shift_JIS | MIT |
| `temple_thai.pac` | Written for this repository by `../generate-fixtures.php`. The Thai code page | MIT |
| `train_cyrillic.pac` | Written for this repository by `../generate-fixtures.php`. The Cyrillic code page | MIT |
| `weather_greek.pac` | Written for this repository by `../generate-fixtures.php`. The Greek code page with accents | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit | MIT |

No permissive real PAC files are known. The script writes each file in the shape that the PAC code of Subtitle Edit reads and writes, with the character bytes of its code tables, see THIRD_PARTY_NOTICES.md.

Regenerate the self-written files with `php tests/files/pac/generate-fixtures.php`. The script writes every byte itself and does not use the library.
