# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/stl/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `astisub_opn.stl` | https://github.com/asticode/go-astisub/blob/7671e72e9d47287d6f8ded80a004560b63a2ead5/testdata/example-opn-in.stl | go-astisub, MIT |
| `bakery_teletext_25fps.stl` | Written for this repository by `../generate-fixtures.php`. Teletext codes, ISO 6937 accents, extension blocks, a comment block, a user data block and a start of programme of 10:00:00:00 | MIT |
| `harbour_open_30fps.stl` | Written for this repository by `../generate-fixtures.php`. Open subtitles at 30 fps, a code page 850 title, two subtitle groups and a cumulative set | MIT |
| `library_hebrew.stl` | Written for this repository by `../generate-fixtures.php`. Character code table 04, ISO 8859-8 | MIT |
| `market_arabic.stl` | Written for this repository by `../generate-fixtures.php`. Character code table 02, ISO 8859-6 | MIT |
| `shop_comments_out_of_order.stl` | Written for this repository by `../generate-fixtures.php`. Subtitles out of time order, and a comment block before a subtitle that starts earlier than the subtitle before it | MIT |
| `train_cyrillic.stl` | Written for this repository by `../generate-fixtures.php`. Character code table 01, ISO 8859-5 | MIT |
| `weather_greek.stl` | Written for this repository by `../generate-fixtures.php`. Character code table 03, ISO 8859-7 | MIT |
| `irt_requirement-0056-001_modified.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/irt/requirement-0056-001_modified.stl, unchanged. 4 subtitles in 3 subtitle groups | sandflow/ttconv after IRT SCF, Apache-2.0, see THIRD_PARTY_NOTICES.md |
| `irt_requirement-0061-001.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/irt/requirement-0061-001.stl, unchanged. Teletext boxing on row 1 | sandflow/ttconv after IRT SCF, Apache-2.0, see THIRD_PARTY_NOTICES.md |
| `irt_requirement-0074-001.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/irt/requirement-0074-001.stl, unchanged. Punctuation of the ISO 6937 character set | sandflow/ttconv after IRT SCF, Apache-2.0, see THIRD_PARTY_NOTICES.md |
| `irt_requirement-0076-001.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/irt/requirement-0076-001.stl, unchanged. Black text, double height, a full GSI block and a start of programme of 10:00:00:00 | sandflow/ttconv after IRT SCF, Apache-2.0, see THIRD_PARTY_NOTICES.md |
| `irt_requirement-0076-009.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/irt/requirement-0076-009.stl, unchanged. New background and black background codes | sandflow/ttconv after IRT SCF, Apache-2.0, see THIRD_PARTY_NOTICES.md |
| `sandflow_cumulative_set.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/sandflow/cumulative_set.stl, unchanged. A cumulative set of 4 subtitles after a single subtitle | sandflow/ttconv, BSD-2-Clause |
| `sandflow_multi_tti_subtitle.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/sandflow/multi_tti_subtitle.stl, unchanged. One subtitle in 3 TTI blocks with extension block numbers | sandflow/ttconv, BSD-2-Clause |
| `sandflow_overlapping_tti.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/sandflow/overlapping_tti.stl, unchanged. 2 subtitles that overlap in time, GSI counts padded with spaces | sandflow/ttconv, BSD-2-Clause |
| `sandflow_vp18_3_lines.stl` | https://github.com/sandflow/ttconv/blob/1fed6abf555ca1d5ae2c839ad3c1625f15c9a96c/src/test/resources/stl/sandflow/vp18_3_lines.stl, unchanged. 3 rows from vertical position 18 | sandflow/ttconv, BSD-2-Clause |
| `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. Each file equals the PHP write golden of the same name at the pinned commit. | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |

The go-astisub files `example-in.stl` and `example-in-nonzero-offset.stl` hold film dialogue, so this repository does not copy them.

The self-written files come from `tests/files/stl/generate-fixtures.php` in subtitle-toolbox-php. The script writes every byte itself and does not use the library.
