# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/encoding/invalid-utf-8, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

All files are written for this repository. Each file is UTF-8 with LF line endings and no BOM. The text of the second cue holds the single byte `E9`, which is `é` in Windows-1252 and not valid UTF-8. The other letters with accents are valid UTF-8, so code page detection keeps each file as UTF-8.

| File | Source | License |
|:--- |:--- |:--- |
| `one-bad-byte.srt` | Written for this repository | MIT |
| `one-bad-byte.vtt` | Written for this repository | MIT |
| `one-bad-byte.sbv` | Written for this repository | MIT |
| `one-bad-byte.ass` | Written for this repository | MIT |
| `one-bad-byte.sub` | Written for this repository. SubViewer 2 | MIT |
| `one-bad-byte.csv` | Written for this repository | MIT |
| `one-bad-byte.smi` | Written for this repository | MIT |
| `one-bad-byte.ttml` | Written for this repository | MIT |
| `one-bad-byte.srv1` | Written for this repository. YouTube transcript XML | MIT |
| `one-bad-byte.json3` | Written for this repository. YouTube json3 | MIT |
