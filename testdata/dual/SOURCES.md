# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/dual, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `station_en.srt` | Written for this repository. English SubRip with UTF-8 BOM, CR LF line endings and an `<i>` tag. | MIT |
| `station_de.srt` | Written for this repository. German translation as SubRip without BOM, with LF line endings. Its cue times differ from the English file by up to 0.4 s, and one English cue has two German cues. | MIT |
| `station_stack.srt`, `station_stack.vtt`, `station_stack.ass` | Written for this repository. Expected `SubRipFormatter`, `WebVttFormatter` and `AssFormatter` output of `DualSubtitle::fromPair()` in mode `stack` with the secondary style `i`. | MIT |
| `station_top_bottom.srt`, `station_top_bottom.vtt`, `station_top_bottom.ass` | Written for this repository. Expected output in mode `topBottom` with the secondary style `font color="#ffff00"`. | MIT |
| `station_top_bottom_aligned.srt` | Generated for this repository. Output of `subtitle-toolbox dual --primary station_en.srt --secondary station_de.srt --mode top-bottom --snap-tolerance 00:00.5 --secondary-alignment 7`, checked by hand | MIT |
