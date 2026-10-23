# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/mpl2/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

Every file is written for this repository. No permissively licensed real MPL2 file was found.

| File | Source | License |
|:--- |:--- |:--- |
| `pysubs2_shape_bom.txt` | Written for this repository in the shape of https://github.com/tkarabela/pysubs2/blob/ae3c520a8aa13130f36d84df02ce15a843bc9bdb/tests/formats/test_mpl2.py: a space after the brackets and after `/`, spaces after the line break character, UTF-8 BOM, LF, a cue at 1 hour | MIT |
| `subtitle_edit_shape.txt` | Written for this repository in the shape of the output of https://github.com/SubtitleEdit/subtitleedit/blob/ffefc2f410ba584a9a34dd87e9d2ccb5a8939781/src/libse/SubtitleFormats/MPlayer2.cs: CR LF, no line break after the last cue | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.out.*`, `expected/*.to.*` | Written by the Go write golden tests with -update, reviewed by hand | MIT |
