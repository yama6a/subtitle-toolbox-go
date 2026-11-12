# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/scc/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `popon_broadcast_df.scc` | Written for this repository in the shape of the pop-on example in http://www.theneitherworld.com/mcpoodle/SCC_TOOLS/DOCS/SCC_FORMAT.HTML: EDM and two `8080` words before each EOC, drop-frame time codes across 00:59:00 and 01:00:00, PAC styles, mid-row codes, special and extended characters, captions on rows 1 and 2 | MIT |
| `rollup_news_ndf.scc` | Written for this repository in the shape of a live roll-up file: RU3, CR and a PAC on each line, words that continue on the next line, a change to RU2, non-drop time codes, CR LF line endings | MIT |
| `painton_corrections.scc` | Written for this repository in the shape of a paint-on file: RDC, a backspace, a delete to the end of the row, tab offsets, a space after the time code | MIT |
| `interleaved_pac_tab.scc` | Written for this repository in the shape of the file in https://github.com/pbs/pycaption/issues/194: PAC, tab offset, PAC, tab offset | MIT |
| `out_of_order_lines.scc` | Written for this repository in the shape of the output in https://github.com/pbs/pycaption/issues/352: the line of a short caption comes first in the file, but the line of a long caption has an earlier time code | MIT |
| `raw_data_row.scc` | Written for this repository in the shape of the file in https://github.com/SubtitleEdit/subtitleedit/issues/11341: a row of random byte pairs without a PAC between two valid captions | MIT |
| `captionmaker_popon_df.scc` | Written for this repository in the shape of a CaptionMaker or MacCaption pop-on export: `94ae 94ae 9420 9420` (ENM, RCL) before each caption, every control code doubled, `942f 942f` (EOC) at the end of the line, `942c 942c` (EDM) on a line of its own, drop-frame time codes | MIT |
| `premiere_popon_df.scc` | Written for this repository in the shape of an Adobe Premiere Pro SCC export: one caption per line with RCL before ENM, `942f 942f` at the end of the line, no EDM between captions, drop-frame time codes with `;` | MIT |
| `live_rollup_ru2.scc` | Written for this repository in the shape of a live roll-up encoder: RU2, `94ad` (CR) before each line, all caps, a time code every 1 to 2 seconds, words that continue on the next line of the file | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand. The PHP library has no SCC write golden, so these are Go output. | MIT |
