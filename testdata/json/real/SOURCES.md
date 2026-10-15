# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/json/real, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `own_aegisub.json` | Written for this repository. `JsonFormatter` output with pretty print for `tests/files/ass/real/own_aegisub.ass`. | MIT |
| `own_alignment_and_coordinates.json` | Written for this repository. `JsonFormatter` output with pretty print for `tests/files/srt/real/own_alignment_and_coordinates.srt`. | MIT |
| `own_image_cues.json` | Written for this repository. Two image cues with PNG images of 2x1 and 1x1 pixels from `PngEncoder`, one forced and one with text. | MIT |
| `own_max_seconds.json` | Written for this repository. The last cue ends at 359999999.999 seconds, the latest time that the parsers accept | MIT |
| `pysubs2_regions_3x.json` | Written for this repository. `JsonFormatter` output with pretty print of release 3.26.10 for `tests/files/ttml/real/pysubs2_regions.ttml`, written in the shape of https://github.com/tkarabela/pysubs2/blob/ae3c520a8aa13130f36d84df02ce15a843bc9bdb/tests/data/ttml_example2.ttml | MIT |
| `w3c_cue_settings_3x.json` | `JsonFormatter` output with pretty print of release 3.26.10 for `tests/files/vtt/real/w3c_cue_settings.vtt`, from https://www.w3.org/TR/webvtt1/, section 1.4, cue settings example | W3C Software and Document License, https://www.w3.org/copyright/software-license-2023/ |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
