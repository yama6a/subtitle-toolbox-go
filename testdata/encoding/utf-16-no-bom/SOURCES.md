# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/encoding/utf-16-no-bom, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

All files are written for this repository. Each format has the same 2 cues. Each file started as UTF-8 with CR LF line endings, except the SBV file, which has LF. `iconv -f UTF-8 -t UTF-16LE` or `-t UTF-16BE` wrote the encoding. No file has a BOM.

| File | Source | License |
|:--- |:--- |:--- |
| `utf-16le-no-bom.srt`, `utf-16be-no-bom.srt` | Written for this repository | MIT |
| `utf-16le-no-bom.vtt`, `utf-16be-no-bom.vtt` | Written for this repository | MIT |
| `utf-16le-no-bom.sbv`, `utf-16be-no-bom.sbv` | Written for this repository in the shape of https://github.com/dagronf/SwiftSubtitles/blob/485c6f1fc71235041198a8bb893fa22f807451a6/Tests/SwiftSubtitlesTests/resources/sbv/captions-LE.sbv: UTF-16 LE without a BOM, LF line endings | MIT |
| `utf-16le-no-bom.ass`, `utf-16be-no-bom.ass` | Written for this repository | MIT |
| `utf-16le-no-bom.smi`, `utf-16be-no-bom.smi` | Written for this repository | MIT |
| `utf-16le-no-bom.ttml`, `utf-16be-no-bom.ttml` | Written for this repository. The XML declaration says `encoding="UTF-16"` | MIT |
| `utf-16le-no-bom.sub`, `utf-16be-no-bom.sub` | Written for this repository. MicroDVD at 10 fps, set by the first line `{1}{1}10` | MIT |
| `utf-16le-no-bom.csv`, `utf-16be-no-bom.csv` | Written for this repository | MIT |
