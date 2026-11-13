# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/resegmenting, MIT. In the rows below, "this repository" means subtitle-toolbox-php, except for the last row.

| File | Source | License |
|:--- |:--- |:--- |
| `own_whisper_long_segments.json` | Written for this repository in the shape of openai-whisper `--output_format json --word_timestamps True`, as described in `tests/files/whisper/real/SOURCES.md`. A first segment of 11.8 s with three sentences, a sentence across two segments, and a pause of 2 s. | MIT |
| `own_whisper_long_segments_split.vtt` | Written for this repository. Expected `WebVttFormatter` output for `own_whisper_long_segments.json` after `Resegmenter::apply()` with `ResegmentMode::SplitLong` with the default options. | MIT |
| `own_whisper_long_segments_resegmented.srt` | Written for this repository. Expected `SubRipFormatter` output for `own_whisper_long_segments.json` after `Resegmenter::apply()` with `ResegmentMode::ByWords` with the default options. | MIT |
| `own_whisper_long_segments_resegmented_30x1.srt` | Generated for this repository. Output of `subtitle-toolbox convert own_whisper_long_segments.json --structure-resegment --structure-max-cpl 30 --structure-max-lines 1 --structure-max-word-gap 00:00:00.3 --to srt`, checked by hand | MIT |
| `own_whisper_long_segments_words.vtt` | Generated for this repository. Output of `subtitle-toolbox convert own_whisper_long_segments.json --to vtt --word-timestamps`, checked by hand | MIT |
| `cjk_texts.txt` | Written for this repository (subtitle-toolbox-go). The Japanese texts of the resegment tests, which `gosmopolitan` rejects in Go source: a text of 4 sentences, and a text with the word breaks of the CJK word test. One text per line. | MIT |
| `rtl_split_texts.txt` | Written for this repository (subtitle-toolbox-go). The 3 texts that `testEachSplitPartGetsItsOwnRightToLeftEmbedding` of subtitle-toolbox-php expects, one per line. Cues 2 to 4 of `transforms/own_rtl_long.srt` after `FixRightToLeft` and `SplitLong` with limits 10 x 1 and `MinDuration` 0.5 s | MIT |
