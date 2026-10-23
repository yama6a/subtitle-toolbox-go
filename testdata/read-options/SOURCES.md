# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/read-options, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

The JSON files hold the cues that PHP 1.x read from a real file, as `[start, end, lines]` rows. `TestReadOptionGoldens` checks that Go reads the same cues with the matching `ReadOptions`.

| File | Source | License |
|:--- |:--- |:--- |
| `french_windows_1252.json` | Cues of `../encoding/french-windows-1252.srt`, read with the source encoding `Windows-1252` | MIT |
| `sami_multi_language_frcc.json` | Cues of `../sami/real/multi_language.smi`, read by `new SamiParser("FRCC")` | MIT |
| `microdvd_subsrt_sample.json` | Cues of `../microdvd/real/subsrt_sample.sub`, read by `new MicroDvdParser(23.976)` | MIT |
